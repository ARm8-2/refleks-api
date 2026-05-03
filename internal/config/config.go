package config

import (
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultAppName             = "refleks-api"
	defaultEnv                 = "development"
	defaultPort                = 8080
	defaultLogLevel            = "info"
	defaultReadTO              = 15 * time.Second
	defaultWriteTO             = 15 * time.Second
	defaultIdleTO              = 60 * time.Second
	defaultReadHdrTO           = 5 * time.Second
	defaultShutTO              = 10 * time.Second
	defaultRunSyncEnabled      = false
	defaultR2Region            = "auto"
	defaultR2RawPublicBucket   = "refleks-raw-public"
	defaultR2LabPrivateBucket  = "refleks-lab-private"
	defaultR2KeyPrefix         = "runs"
	defaultR2SignedURLTTL      = 15 * time.Minute
	defaultRunSyncMaxFileBytes = 25 << 20
	defaultRunSyncMaxBulkFiles = 100
	defaultRunSyncMaxBulkBytes = 250 << 20
	defaultRunSyncMaxHashes    = 1000
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

	RunSyncEnabled          bool
	DatabaseURL             string
	R2Endpoint              string
	R2Region                string
	R2RawPublicBucket       string
	R2LabPrivateBucket      string
	R2RawPublicBaseURL      string
	R2SignedURLTTL          time.Duration
	R2AccessKeyID           string
	R2SecretAccessKey       string
	R2KeyPrefix             string
	RunSyncMaxFileBytes     int64
	RunSyncMaxBulkFiles     int
	RunSyncMaxBulkBytes     int64
	RunSyncMaxMissingHashes int
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

	runSyncEnabled, err := envBool("RUNSYNC_ENABLED", defaultRunSyncEnabled)
	if err != nil {
		return Config{}, err
	}

	runSyncMaxFileBytes, err := envInt64("RUNSYNC_MAX_FILE_BYTES", defaultRunSyncMaxFileBytes)
	if err != nil {
		return Config{}, err
	}
	if runSyncMaxFileBytes <= 0 {
		return Config{}, fmt.Errorf("RUNSYNC_MAX_FILE_BYTES must be greater than zero")
	}

	runSyncMaxBulkFiles, err := envInt("RUNSYNC_MAX_BULK_FILES", defaultRunSyncMaxBulkFiles)
	if err != nil {
		return Config{}, err
	}
	if runSyncMaxBulkFiles <= 0 {
		return Config{}, fmt.Errorf("RUNSYNC_MAX_BULK_FILES must be greater than zero")
	}

	runSyncMaxBulkBytes, err := envInt64("RUNSYNC_MAX_BULK_BYTES", defaultRunSyncMaxBulkBytes)
	if err != nil {
		return Config{}, err
	}
	if runSyncMaxBulkBytes <= 0 {
		return Config{}, fmt.Errorf("RUNSYNC_MAX_BULK_BYTES must be greater than zero")
	}

	runSyncMaxHashes, err := envInt("RUNSYNC_MAX_MISSING_HASHES", defaultRunSyncMaxHashes)
	if err != nil {
		return Config{}, err
	}
	if runSyncMaxHashes <= 0 {
		return Config{}, fmt.Errorf("RUNSYNC_MAX_MISSING_HASHES must be greater than zero")
	}

	databaseURL, hasDatabaseConfig, err := loadOptionalDatabaseURL()
	if err != nil {
		return Config{}, err
	}
	r2Endpoint := strings.TrimSpace(os.Getenv("R2_ENDPOINT"))
	r2RawBucket := envOrDefault("R2_RAW_PUBLIC_BUCKET", defaultR2RawPublicBucket)
	r2LabPrivateBucket := envOrDefault("R2_LAB_PRIVATE_BUCKET", defaultR2LabPrivateBucket)
	r2RawPublicBaseURL := strings.TrimSpace(os.Getenv("R2_RAW_PUBLIC_BASE_URL"))
	r2SignedURLTTL, err := envDuration("R2_SIGNED_URL_TTL", defaultR2SignedURLTTL)
	if err != nil {
		return Config{}, err
	}
	r2AccessKeyID := strings.TrimSpace(os.Getenv("R2_ACCESS_KEY_ID"))
	r2SecretAccessKey := strings.TrimSpace(os.Getenv("R2_SECRET_ACCESS_KEY"))
	r2Region := envOrDefault("R2_REGION", defaultR2Region)
	r2KeyPrefix := envOrDefault("R2_KEY_PREFIX", defaultR2KeyPrefix)

	if runSyncEnabled {
		if !hasDatabaseConfig {
			return Config{}, fmt.Errorf("DATABASE_URL or POSTGRES_DB/POSTGRES_USER configuration is required when RUNSYNC_ENABLED=true")
		}
		if r2Endpoint == "" {
			return Config{}, fmt.Errorf("R2_ENDPOINT is required when RUNSYNC_ENABLED=true")
		}
		if strings.TrimSpace(r2RawBucket) == "" {
			return Config{}, fmt.Errorf("R2_RAW_PUBLIC_BUCKET is required when RUNSYNC_ENABLED=true")
		}
		if r2AccessKeyID == "" || r2SecretAccessKey == "" {
			return Config{}, fmt.Errorf("R2_ACCESS_KEY_ID and R2_SECRET_ACCESS_KEY are required when RUNSYNC_ENABLED=true")
		}
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

		RunSyncEnabled:          runSyncEnabled,
		DatabaseURL:             databaseURL,
		R2Endpoint:              r2Endpoint,
		R2Region:                r2Region,
		R2RawPublicBucket:       strings.TrimSpace(r2RawBucket),
		R2LabPrivateBucket:      strings.TrimSpace(r2LabPrivateBucket),
		R2RawPublicBaseURL:      r2RawPublicBaseURL,
		R2SignedURLTTL:          r2SignedURLTTL,
		R2AccessKeyID:           r2AccessKeyID,
		R2SecretAccessKey:       r2SecretAccessKey,
		R2KeyPrefix:             r2KeyPrefix,
		RunSyncMaxFileBytes:     runSyncMaxFileBytes,
		RunSyncMaxBulkFiles:     runSyncMaxBulkFiles,
		RunSyncMaxBulkBytes:     runSyncMaxBulkBytes,
		RunSyncMaxMissingHashes: runSyncMaxHashes,
	}, nil
}

func loadOptionalDatabaseURL() (string, bool, error) {
	ensureEnvLoaded()

	if databaseURL := strings.TrimSpace(os.Getenv("DATABASE_URL")); databaseURL != "" {
		return databaseURL, true, nil
	}

	hasAnyPostgresSetting := false
	for _, key := range []string{"POSTGRES_HOST", "POSTGRES_PORT", "POSTGRES_DB", "POSTGRES_USER", "POSTGRES_PASSWORD", "POSTGRES_SSLMODE"} {
		if strings.TrimSpace(os.Getenv(key)) != "" {
			hasAnyPostgresSetting = true
			break
		}
	}
	if !hasAnyPostgresSetting {
		return "", false, nil
	}

	host := strings.TrimSpace(envOrDefault("POSTGRES_HOST", "postgres"))
	port := strings.TrimSpace(envOrDefault("POSTGRES_PORT", "5432"))
	database := strings.TrimSpace(os.Getenv("POSTGRES_DB"))
	user := strings.TrimSpace(os.Getenv("POSTGRES_USER"))
	password := strings.TrimSpace(os.Getenv("POSTGRES_PASSWORD"))
	sslMode := strings.TrimSpace(envOrDefault("POSTGRES_SSLMODE", "disable"))

	if host == "" {
		return "", false, fmt.Errorf("POSTGRES_HOST must not be empty")
	}
	if port == "" {
		return "", false, fmt.Errorf("POSTGRES_PORT must not be empty")
	}
	if _, err := strconv.Atoi(port); err != nil {
		return "", false, fmt.Errorf("POSTGRES_PORT must be a valid integer: %w", err)
	}
	if database == "" {
		return "", false, fmt.Errorf("POSTGRES_DB is required when DATABASE_URL is not set")
	}
	if user == "" {
		return "", false, fmt.Errorf("POSTGRES_USER is required when DATABASE_URL is not set")
	}
	if sslMode == "" {
		return "", false, fmt.Errorf("POSTGRES_SSLMODE must not be empty")
	}

	connectionUser := url.User(user)
	if password != "" {
		connectionUser = url.UserPassword(user, password)
	}

	connectionURL := &url.URL{
		Scheme: "postgresql",
		User:   connectionUser,
		Host:   net.JoinHostPort(host, port),
		Path:   database,
	}

	query := url.Values{}
	query.Set("sslmode", sslMode)
	connectionURL.RawQuery = query.Encode()

	return connectionURL.String(), true, nil
}

func envOrDefault(key, fallback string) string {
	ensureEnvLoaded()
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
	ensureEnvLoaded()
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

func envInt64(key string, fallback int64) (int64, error) {
	ensureEnvLoaded()
	raw, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(raw) == "" {
		return fallback, nil
	}
	v, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s must be a 64-bit integer: %w", key, err)
	}
	return v, nil
}

func envBool(key string, fallback bool) (bool, error) {
	ensureEnvLoaded()
	raw, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(raw) == "" {
		return fallback, nil
	}
	v, err := strconv.ParseBool(strings.TrimSpace(raw))
	if err != nil {
		return false, fmt.Errorf("%s must be true|false: %w", key, err)
	}
	return v, nil
}

func envDuration(key string, fallback time.Duration) (time.Duration, error) {
	ensureEnvLoaded()
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
