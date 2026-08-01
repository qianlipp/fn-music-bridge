package service

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	ListenAddr        string
	APIBaseURL        string
	CredentialFile    string
	CredentialKey     string
	SettingsFile      string
	RequestTimeout    time.Duration
	EnableSubsonic    bool
	EnableAmpache     bool
	EnableSynology    bool
	EnableJellyfin    bool
	ApplyProtocolPort func(int) error
}

func ConfigFromEnv() Config {
	return Config{
		ListenAddr:     envString("YINQIAO_SERVICE_LISTEN", "0.0.0.0:14040"),
		APIBaseURL:     strings.TrimRight(envString("YINQIAO_API_URL", "http://127.0.0.1:18090/yinqiao/v1"), "/"),
		CredentialFile: envString("YINQIAO_CREDENTIAL_FILE", "/tmp/yinqiao-credentials.enc"),
		CredentialKey:  envString("YINQIAO_CREDENTIAL_KEY_FILE", "/tmp/yinqiao-credentials.key"),
		SettingsFile:   envString("YINQIAO_SETTINGS_FILE", "/tmp/yinqiao-settings.json"),
		RequestTimeout: 60 * time.Second,
		EnableSubsonic: envBool("YINQIAO_ENABLE_SUBSONIC", true),
		EnableAmpache:  envBool("YINQIAO_ENABLE_AMPACHE", true),
		EnableSynology: envBool("YINQIAO_ENABLE_SYNOLOGY", true),
		EnableJellyfin: envBool("YINQIAO_ENABLE_JELLYFIN", true),
	}
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

func envString(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
