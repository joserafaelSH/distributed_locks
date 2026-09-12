package config

import (
	"os"
	"strconv"
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
}

func Load() Config {
	return Config{
		ServiceName:  getEnv("SERVICE_NAME", "service-template"),
		Environment:  getEnv("ENVIRONMENT", "local"),
		HTTPPort:     getEnvInt("HTTP_PORT", 8081),
		PprofPort:    getEnvInt("PPROF_PORT", 6060),
		OTLPEndpoint: getEnv("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4317"),
		LogLevel:     getEnv("LOG_LEVEL", "info"),
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
