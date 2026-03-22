package config

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultAppName   = "refleks-api"
	defaultEnv       = "development"
	defaultPort      = 8080
	defaultLogLevel  = "info"
	defaultReadTO    = 15 * time.Second
	defaultWriteTO   = 15 * time.Second
	defaultIdleTO    = 60 * time.Second
	defaultReadHdrTO = 5 * time.Second
	defaultShutTO    = 10 * time.Second
)

// Config contains application runtime settings.
type Config struct {
	AppName           string
	Environment       string
	Version           string
	Port              int
	LogLevel          slog.Level
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	ReadHeaderTimeout time.Duration
	ShutdownTimeout   time.Duration
}

// Load builds Config from environment variables with safe defaults.
func Load(version string) (Config, error) {
	port, err := envInt("APP_PORT", defaultPort)
	if err != nil {
		return Config{}, err
	}
	if port < 1 || port > 65535 {
		return Config{}, fmt.Errorf("APP_PORT must be between 1 and 65535")
	}

	logLevel, err := parseLogLevel(envOrDefault("LOG_LEVEL", defaultLogLevel))
	if err != nil {
		return Config{}, err
	}

	readTimeout, err := envDuration("HTTP_READ_TIMEOUT", defaultReadTO)
	if err != nil {
		return Config{}, err
	}
	writeTimeout, err := envDuration("HTTP_WRITE_TIMEOUT", defaultWriteTO)
	if err != nil {
		return Config{}, err
	}
	idleTimeout, err := envDuration("HTTP_IDLE_TIMEOUT", defaultIdleTO)
	if err != nil {
		return Config{}, err
	}
	readHeaderTimeout, err := envDuration("HTTP_READ_HEADER_TIMEOUT", defaultReadHdrTO)
	if err != nil {
		return Config{}, err
	}
	shutdownTimeout, err := envDuration("HTTP_SHUTDOWN_TIMEOUT", defaultShutTO)
	if err != nil {
		return Config{}, err
	}

	if shutdownTimeout <= 0 {
		return Config{}, fmt.Errorf("HTTP_SHUTDOWN_TIMEOUT must be greater than zero")
	}

	resolvedVersion := envOrDefault("APP_VERSION", version)
	if strings.TrimSpace(resolvedVersion) == "" {
		resolvedVersion = "dev"
	}

	return Config{
		AppName:           envOrDefault("APP_NAME", defaultAppName),
		Environment:       envOrDefault("APP_ENV", defaultEnv),
		Version:           resolvedVersion,
		Port:              port,
		LogLevel:          logLevel,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
		ReadHeaderTimeout: readHeaderTimeout,
		ShutdownTimeout:   shutdownTimeout,
	}, nil
}

func envOrDefault(key, fallback string) string {
	v, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}
	v = strings.TrimSpace(v)
	if v == "" {
		return fallback
	}
	return v
}

func envInt(key string, fallback int) (int, error) {
	raw, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(raw) == "" {
		return fallback, nil
	}
	v, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer: %w", key, err)
	}
	return v, nil
}

func envDuration(key string, fallback time.Duration) (time.Duration, error) {
	raw, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(raw) == "" {
		return fallback, nil
	}
	v, err := time.ParseDuration(strings.TrimSpace(raw))
	if err != nil {
		return 0, fmt.Errorf("%s must be a valid duration (e.g. 5s): %w", key, err)
	}
	if v <= 0 {
		return 0, fmt.Errorf("%s must be greater than zero", key)
	}
	return v, nil
}

func parseLogLevel(raw string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("LOG_LEVEL must be one of debug|info|warn|error")
	}
}
