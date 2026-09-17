// Package config 配置加载：环境变量优先，密钥不进 git。
package config

import (
	"os"
	"strconv"
)

type Config struct {
	Port          string
	DBPath        string
	JWTSecret     string
	LogLevel      string
	EncryptionKey string
	// LLM 真实供应商（留空用脚本供应商，本地联调不依赖外网）
	LLMProvider string
	LLMAPIKey   string
	LLMBaseURL  string
	LLMModel    string
}

func Load() Config {
	return Config{
		Port:          getenv("PORT", "8080"),
		DBPath:        getenv("DB_PATH", "./data/homeagent.db"),
		JWTSecret:     getenv("JWT_SECRET", "dev-only-change-me"),
		LogLevel:      getenv("LOG_LEVEL", "info"),
		EncryptionKey: getenv("ENCRYPTION_KEY", "dev-only-32-bytes-key-xxxxx"),
		LLMProvider:   getenv("LLM_PROVIDER", ""),
		LLMAPIKey:     getenv("LLM_API_KEY", ""),
		LLMBaseURL:    getenv("LLM_BASE_URL", ""),
		LLMModel:      getenv("LLM_MODEL", ""),
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getenvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}
