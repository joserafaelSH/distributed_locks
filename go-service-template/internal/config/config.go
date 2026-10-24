package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	ServiceName string
	Environment string

	HTTPPort int
	// pprof is exposed on its own port so it's trivial to firewall off from
	// the internet-facing HTTP port in a real deployment.
	PprofPort int

	OTLPEndpoint string // otel-collector-opentelemetry-collector.observability:4317
	LogLevel     string
	Postgres     PostgresConfig
	Redis        RedisConfig
	Worker       WorkerConfig
}

type PostgresConfig struct {
	Host     string
	Port     int
	Database string
	Username string
	Password string
	// MaxOpenConns/MaxIdleConns bound the connection pool so a burst of
	// concurrent queries (e.g. many jobs finishing at once) queues for a
	// connection instead of the server rejecting it with "too many clients".
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

type RedisConfig struct {
	Addr     string
	Password string
}

type WorkerConfig struct {
	// How often the poller checks for new pending jobs.
	PollInterval time.Duration
	// Max jobs claimed per poll.
	BatchSize int
	// TTL of the distributed poll lock held in Redis — bounds how long a
	// crashed/stalled poller can block other instances from polling.
	LockTTL time.Duration
	// Simulated per-job processing time.
	JobTimeout time.Duration
	// Simulated failure rate for processed jobs, 0-100.
	FailurePercent float64
	// Max jobs processed concurrently across all polls, bounding how many
	// DB connections the worker can hold at once.
	MaxConcurrentJobs int
}

func Load() Config {
	return Config{
		ServiceName:  getEnv("SERVICE_NAME", "service-template"),
		Environment:  getEnv("ENVIRONMENT", "local"),
		HTTPPort:     getEnvInt("HTTP_PORT", 8081),
		PprofPort:    getEnvInt("PPROF_PORT", 6060),
		OTLPEndpoint: getEnv("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4317"),
		LogLevel:     getEnv("LOG_LEVEL", "info"),
		Postgres: PostgresConfig{
			Host:            getEnv("POSTGRES_HOST", "localhost"),
			Port:            getEnvInt("POSTGRES_PORT", 5432),
			Database:        getEnv("POSTGRES_DATABASE", "mydb"),
			Username:        getEnv("POSTGRES_USERNAME", "postgres"),
			Password:        getEnv("POSTGRES_PASSWORD", "postgres"),
			MaxOpenConns:    getEnvInt("POSTGRES_MAX_OPEN_CONNS", 10),
			MaxIdleConns:    getEnvInt("POSTGRES_MAX_IDLE_CONNS", 10),
			ConnMaxLifetime: getEnvDuration("POSTGRES_CONN_MAX_LIFETIME", 5*time.Minute),
		},
		Redis: RedisConfig{
			Addr:     getEnv("REDIS_ADDR", "localhost:6379"),
			Password: getEnv("REDIS_PASSWORD", "redis"),
		},
		Worker: WorkerConfig{
			PollInterval:   getEnvDuration("WORKER_POLL_INTERVAL", 60*time.Second),
			BatchSize:      getEnvInt("WORKER_BATCH_SIZE", 10),
			LockTTL:        getEnvDuration("WORKER_LOCK_TTL", 10*time.Second),
			JobTimeout:        getEnvDuration("WORKER_JOB_TIMEOUT", 2*time.Second),
			FailurePercent:    getEnvFloat("WORKER_FAILURE_PERCENT", 20),
			MaxConcurrentJobs: getEnvInt("WORKER_MAX_CONCURRENT_JOBS", 10),
		},
	}
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return fallback
	}
	return d
}

func getEnvFloat(key string, fallback float64) float64 {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return fallback
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return fallback
	}
	return f
}
