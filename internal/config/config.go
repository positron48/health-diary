package config

import (
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strings"
)

type Config struct {
	HTTPAddr    string
	DatabaseURL string
	LogLevel    slog.Level
}

func Load() (Config, error) {
	cfg := Config{
		HTTPAddr:    value("HTTP_ADDR", ":8080"),
		DatabaseURL: value("DATABASE_URL", ""),
		LogLevel:    parseLogLevel(value("LOG_LEVEL", "info")),
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
	return cfg, nil
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
