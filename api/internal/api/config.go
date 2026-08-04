package api

import (
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultSigningPrefix = "NDzZTVxnRKP8Z0jXg1VAMonaG8akvh"
	defaultClientKey     = "6D5602D4-A342-4799-A0F0-BB795E7167D0"
)

type Config struct {
	ListenAddr         string
	ListenUnixSocket   string
	PublicPrefix       string
	UpstreamOrigin     string
	UpstreamAPIBase    string
	UpstreamUnixSocket string
	UpstreamGateways   []string
	UpstreamRouteMap   string
	SigningPrefix      string
	ClientKey          string
	APIKey             string
	AllowedOrigins     []string
	MaxBodyBytes       int64
	UpstreamTimeout    time.Duration
	ProxyMusicRoot     bool
}

func ConfigFromEnv() Config {
	return Config{
		ListenAddr:         envString("YINQIAO_API_LISTEN", "0.0.0.0:18090"),
		ListenUnixSocket:   strings.TrimSpace(os.Getenv("YINQIAO_API_UNIX_SOCKET")),
		PublicPrefix:       normalizePrefix(envString("YINQIAO_API_PREFIX", "/yinqiao/v1")),
		UpstreamOrigin:     strings.TrimRight(envString("YINQIAO_UPSTREAM_ORIGIN", "http://unix"), "/"),
		UpstreamAPIBase:    normalizePrefix(envString("YINQIAO_UPSTREAM_API_BASE", "/music/api/v1")),
		UpstreamUnixSocket: envString("YINQIAO_UPSTREAM_UNIX_SOCKET", "/var/run/trim_music.socket"),
		UpstreamGateways:   splitCSV(envString("YINQIAO_UPSTREAM_GATEWAYS", "http://127.0.0.1:5666,https://127.0.0.1:5667,http://127.0.0.1:8000,https://127.0.0.1:8001")),
		UpstreamRouteMap:   strings.TrimSpace(os.Getenv("YINQIAO_UPSTREAM_ROUTE_MAP")),
		SigningPrefix:      envString("YINQIAO_UPSTREAM_SIGNING_PREFIX", defaultSigningPrefix),
		ClientKey:          envString("YINQIAO_UPSTREAM_CLIENT_KEY", defaultClientKey),
		APIKey:             strings.TrimSpace(os.Getenv("YINQIAO_API_KEY")),
		AllowedOrigins:     splitCSV(envString("YINQIAO_API_ALLOWED_ORIGINS", "*")),
		MaxBodyBytes:       envInt64("YINQIAO_API_MAX_BODY_BYTES", 32<<20),
		UpstreamTimeout:    time.Duration(envInt64("YINQIAO_UPSTREAM_TIMEOUT_SECONDS", 60)) * time.Second,
		ProxyMusicRoot:     envBool("YINQIAO_API_PROXY_MUSIC_ROOT", false),
	}
}

func normalizePrefix(value string) string {
	value = "/" + strings.Trim(value, "/")
	if value == "/" {
		return ""
	}
	return value
}

func envString(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func envInt64(key string, fallback int64) int64 {
	value, err := strconv.ParseInt(strings.TrimSpace(os.Getenv(key)), 10, 64)
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func envBool(key string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func splitCSV(value string) []string {
	var result []string
	for _, item := range strings.Split(value, ",") {
		if item = strings.TrimSpace(item); item != "" {
			result = append(result, item)
		}
	}
	return result
}
