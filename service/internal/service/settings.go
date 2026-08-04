package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

const apiServicePort = 18090

type ProtocolSettings struct {
	OpenSubsonic bool `json:"openSubsonic"`
	Synology     bool `json:"synology"`
	Jellyfin     bool `json:"jellyfin"`
	Ampache      bool `json:"ampache"`
}

type ServiceSettings struct {
	ProtocolPort   int              `json:"protocolPort"`
	AdvertisedHost string           `json:"advertisedHost"`
	FnosGatewayURL string           `json:"fnosGatewayUrl"`
	FnosWebURL     string           `json:"fnosWebUrl"`
	Protocols      ProtocolSettings `json:"protocols"`
}

type settingsStore struct {
	mu    sync.RWMutex
	path  string
	value ServiceSettings
}

type settingsPatch struct {
	ProtocolPort   *int                   `json:"protocolPort"`
	AdvertisedHost *string                `json:"advertisedHost"`
	FnosGatewayURL *string                `json:"fnosGatewayUrl"`
	FnosWebURL     *string                `json:"fnosWebUrl"`
	Protocols      *protocolSettingsPatch `json:"protocols"`
}

type protocolSettingsPatch struct {
	OpenSubsonic *bool `json:"openSubsonic"`
	Synology     *bool `json:"synology"`
	Jellyfin     *bool `json:"jellyfin"`
	Ampache      *bool `json:"ampache"`
}

var hostnamePattern = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9.-]{0,251}[A-Za-z0-9])?$`)

func defaultServiceSettings(config Config) ServiceSettings {
	port, err := strconv.Atoi(listenPort(config.ListenAddr))
	if err != nil {
		port = 14040
	}
	return ServiceSettings{
		ProtocolPort: port,
		Protocols: ProtocolSettings{
			OpenSubsonic: config.EnableSubsonic,
			Synology:     config.EnableSynology,
			Jellyfin:     config.EnableJellyfin,
			Ampache:      config.EnableAmpache,
		},
	}
}

func openSettingsStore(path string, defaults ServiceSettings) (*settingsStore, error) {
	store := &settingsStore{path: path, value: defaults}
	contents, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return store, nil
	}
	if err != nil {
		return nil, err
	}
	var value ServiceSettings
	if err := json.Unmarshal(contents, &value); err != nil {
		return nil, fmt.Errorf("decode service settings: %w", err)
	}
	if err := validateServiceSettings(value); err != nil {
		return nil, fmt.Errorf("validate service settings: %w", err)
	}
	store.value = value
	return store, nil
}

func (s *settingsStore) snapshot() ServiceSettings {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.value
}

func (s *settingsStore) replace(value ServiceSettings) error {
	contents, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	if err := writeAtomic(s.path, append(contents, '\n'), 0o600); err != nil {
		return err
	}
	s.mu.Lock()
	s.value = value
	s.mu.Unlock()
	return nil
}

func (s *Server) ProtocolPort() int {
	return s.settings.snapshot().ProtocolPort
}

func (s *Server) protocolEnabled(name string) bool {
	protocols := s.settings.snapshot().Protocols
	switch name {
	case "openSubsonic":
		return protocols.OpenSubsonic
	case "synology":
		return protocols.Synology
	case "jellyfin":
		return protocols.Jellyfin
	case "ampache":
		return protocols.Ampache
	default:
		return false
	}
}

func (s *Server) handleSettings(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	switch request.Method {
	case http.MethodGet:
		s.writeSettings(writer)
	case http.MethodPut:
		s.updateSettings(writer, request)
	default:
		writer.WriteHeader(http.StatusMethodNotAllowed)
		_ = json.NewEncoder(writer).Encode(map[string]any{"ok": false, "code": "METHOD_NOT_ALLOWED", "error": "method not allowed"})
	}
}

func (s *Server) writeSettings(writer http.ResponseWriter) {
	_ = json.NewEncoder(writer).Encode(map[string]any{
		"ok": true, "settings": s.settings.snapshot(), "managementPort": listenPort(s.config.ListenAddr),
	})
}

func (s *Server) updateSettings(writer http.ResponseWriter, request *http.Request) {
	var patch settingsPatch
	decoder := json.NewDecoder(io.LimitReader(request.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&patch); err != nil {
		writeSettingsError(writer, http.StatusBadRequest, "INVALID_SETTINGS", "设置内容格式不正确")
		return
	}

	s.settingsApplyMu.Lock()
	defer s.settingsApplyMu.Unlock()
	previous := s.settings.snapshot()
	next := mergeSettingsPatch(previous, patch)
	if err := validateServiceSettings(next); err != nil {
		writeSettingsError(writer, http.StatusBadRequest, "INVALID_SETTINGS", err.Error())
		return
	}
	if next.ProtocolPort != previous.ProtocolPort && s.config.ApplyProtocolPort != nil {
		if err := s.config.ApplyProtocolPort(next.ProtocolPort); err != nil {
			writeSettingsError(writer, http.StatusConflict, "PORT_UNAVAILABLE", fmt.Sprintf("端口 %d 无法监听，请确认没有被其他服务占用", next.ProtocolPort))
			return
		}
	}
	if err := s.settings.replace(next); err != nil {
		if next.ProtocolPort != previous.ProtocolPort && s.config.ApplyProtocolPort != nil {
			_ = s.config.ApplyProtocolPort(previous.ProtocolPort)
		}
		s.logger.Printf("save settings error=%v", err)
		writeSettingsError(writer, http.StatusInternalServerError, "SETTINGS_STORE_ERROR", "设置无法保存到本机")
		return
	}
	s.api.setGatewayOrigin(localFnosGatewayOrigin(next.FnosGatewayURL))
	s.writeSettings(writer)
}

func mergeSettingsPatch(value ServiceSettings, patch settingsPatch) ServiceSettings {
	if patch.ProtocolPort != nil {
		value.ProtocolPort = *patch.ProtocolPort
	}
	if patch.AdvertisedHost != nil {
		value.AdvertisedHost = strings.TrimSpace(*patch.AdvertisedHost)
	}
	if patch.FnosGatewayURL != nil {
		value.FnosGatewayURL = strings.TrimSpace(*patch.FnosGatewayURL)
	}
	if patch.FnosWebURL != nil {
		value.FnosWebURL = strings.TrimRight(strings.TrimSpace(*patch.FnosWebURL), "/")
	}
	if patch.Protocols != nil {
		if patch.Protocols.OpenSubsonic != nil {
			value.Protocols.OpenSubsonic = *patch.Protocols.OpenSubsonic
		}
		if patch.Protocols.Synology != nil {
			value.Protocols.Synology = *patch.Protocols.Synology
		}
		if patch.Protocols.Jellyfin != nil {
			value.Protocols.Jellyfin = *patch.Protocols.Jellyfin
		}
		if patch.Protocols.Ampache != nil {
			value.Protocols.Ampache = *patch.Protocols.Ampache
		}
	}
	return value
}

func validateServiceSettings(value ServiceSettings) error {
	if value.ProtocolPort < 1024 || value.ProtocolPort > 65535 {
		return errors.New("客户端服务端口必须在 1024 到 65535 之间")
	}
	if value.ProtocolPort == apiServicePort {
		return errors.New("18090 已由内部 Yinqiao API 使用，请选择其他端口")
	}
	host := strings.TrimSpace(value.AdvertisedHost)
	if host != "" && net.ParseIP(host) == nil {
		if strings.ContainsAny(host, "/:\\ ") || !hostnamePattern.MatchString(host) || strings.Contains(host, "..") {
			return errors.New("局域网主机只能填写 IP 地址或主机名，不要包含协议、端口或路径")
		}
		for _, label := range strings.Split(host, ".") {
			if len(label) == 0 || len(label) > 63 || strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
				return errors.New("局域网主机名格式不正确")
			}
		}
	}
	if err := validateFnosURL(value.FnosGatewayURL, "本机 fnOS 网关"); err != nil {
		return err
	}
	if err := validateFnosURL(value.FnosWebURL, "飞牛访问地址"); err != nil {
		return err
	}
	return nil
}

func validateFnosURL(value, label string) error {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parsed, err := url.Parse(value)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil {
		return fmt.Errorf("%s必须是完整的 HTTP 或 HTTPS 地址", label)
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.Path != "" && parsed.Path != "/") {
		return fmt.Errorf("%s不要包含路径、参数或账号信息", label)
	}
	if port := parsed.Port(); port != "" {
		value, parseErr := strconv.Atoi(port)
		if parseErr != nil || value < 1 || value > 65535 {
			return fmt.Errorf("%s端口无效", label)
		}
	}
	return nil
}

func localFnosGatewayOrigin(value string) string {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return ""
	}
	port := parsed.Port()
	if port == "" {
		if parsed.Scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
	}
	return parsed.Scheme + "://" + net.JoinHostPort("127.0.0.1", port)
}

func writeSettingsError(writer http.ResponseWriter, status int, code, message string) {
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(map[string]any{"ok": false, "code": code, "error": message})
}
