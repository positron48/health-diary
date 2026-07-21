package config

import (
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	HTTPAddr                 string
	DatabaseURL              string
	LogLevel                 slog.Level
	Telegram                 TelegramConfig
	DataEncryptionKey        string
	DataEncryptionKeyVersion int
	JobMaxAttempts           int
	LLMBaseURL               string
	LLMAPIKey                string
	LLMModel                 string
}

type TelegramConfig struct {
	Token          string
	Mode           string
	AllowedUserIDs map[int64]struct{}
}

func Load() (Config, error) {
	cfg := Config{
		HTTPAddr:                 value("HTTP_ADDR", ":8080"),
		DatabaseURL:              value("DATABASE_URL", ""),
		LogLevel:                 parseLogLevel(value("LOG_LEVEL", "info")),
		Telegram:                 TelegramConfig{Token: value("TELEGRAM_BOT_TOKEN", ""), Mode: value("TELEGRAM_MODE", "long_polling")},
		DataEncryptionKey:        value("DATA_ENCRYPTION_KEY", ""),
		DataEncryptionKeyVersion: intValue("DATA_ENCRYPTION_KEY_VERSION", 1),
		JobMaxAttempts:           intValue("JOB_MAX_ATTEMPTS", 5),
		LLMBaseURL:               value("LLM_BASE_URL", "https://api.polza.ai/api/v1"),
		LLMAPIKey:                value("LLM_API_KEY", ""),
		LLMModel:                 value("LLM_MODEL", "openai/gpt-5-mini"),
	}
	if cfg.HTTPAddr == "" {
		return Config{}, fmt.Errorf("HTTP_ADDR must not be empty")
	}
	if cfg.DatabaseURL != "" {
		u, err := url.Parse(cfg.DatabaseURL)
		if err != nil || u.Scheme == "" || u.Host == "" {
			return Config{}, fmt.Errorf("DATABASE_URL must be a valid URL")
		}
	}
	if cfg.Telegram.Mode != "long_polling" && cfg.Telegram.Mode != "webhook" {
		return Config{}, fmt.Errorf("TELEGRAM_MODE must be long_polling or webhook")
	}
	allowed, err := parseIDs(value("TELEGRAM_ALLOWED_USER_IDS", ""))
	if err != nil {
		return Config{}, err
	}
	cfg.Telegram.AllowedUserIDs = allowed
	return cfg, nil
}

func intValue(key string, fallback int) int {
	raw := value(key, "")
	if raw == "" {
		return fallback
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 {
		return fallback
	}
	return n
}

func parseIDs(raw string) (map[int64]struct{}, error) {
	ids := map[int64]struct{}{}
	if raw == "" {
		return ids, nil
	}
	for _, part := range strings.Split(raw, ",") {
		id, err := strconv.ParseInt(strings.TrimSpace(part), 10, 64)
		if err != nil || id <= 0 {
			return nil, fmt.Errorf("TELEGRAM_ALLOWED_USER_IDS contains invalid ID")
		}
		ids[id] = struct{}{}
	}
	return ids, nil
}

func value(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok {
		return strings.TrimSpace(v)
	}
	return fallback
}

func parseLogLevel(raw string) slog.Level {
	switch strings.ToLower(raw) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
