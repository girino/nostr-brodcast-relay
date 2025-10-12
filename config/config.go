package config

import (
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/girino/broadcast-relay/logging"
)

type Config struct {
	SeedRelays          []string
	MandatoryRelays     []string
	TopNRelays          int
	RelayPort           string
	RefreshInterval     time.Duration
	HealthCheckInterval time.Duration
	InitialTimeout      time.Duration
	SuccessRateDecay    float64
	WorkerCount         int
}

func Load() *Config {
	workerCount := getEnvInt("WORKER_COUNT", 0)
	if workerCount <= 0 {
		workerCount = runtime.NumCPU() * 2
	}

	cfg := &Config{
		SeedRelays:          parseSeedRelays(getEnv("SEED_RELAYS", "ws://localhost:10547")),
		MandatoryRelays:     parseSeedRelays(getEnv("MANDATORY_RELAYS", "")),
		TopNRelays:          getEnvInt("TOP_N_RELAYS", 50),
		RelayPort:           getEnv("RELAY_PORT", "3334"),
		RefreshInterval:     getEnvDuration("REFRESH_INTERVAL", 24*time.Hour),
		HealthCheckInterval: getEnvDuration("HEALTH_CHECK_INTERVAL", 5*time.Minute),
		InitialTimeout:      getEnvDuration("INITIAL_TIMEOUT", 5*time.Second),
		SuccessRateDecay:    getEnvFloat("SUCCESS_RATE_DECAY", 0.95),
		WorkerCount:         workerCount,
	}

	logging.DebugMethod("config", "Load", "Loaded configuration: SeedRelays=%d, MandatoryRelays=%d, TopN=%d, Port=%s, Workers=%d",
		len(cfg.SeedRelays), len(cfg.MandatoryRelays), cfg.TopNRelays, cfg.RelayPort, cfg.WorkerCount)

	return cfg
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

func getEnvFloat(key string, defaultValue float64) float64 {
	if value := os.Getenv(key); value != "" {
		if floatVal, err := strconv.ParseFloat(value, 64); err == nil {
			return floatVal
		}
	}
	return defaultValue
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}

func parseSeedRelays(seedStr string) []string {
	if seedStr == "" {
		return []string{}
	}

	relays := strings.Split(seedStr, ",")
	result := make([]string, 0, len(relays))

	for _, relay := range relays {
		relay = strings.TrimSpace(relay)
		if relay != "" {
			result = append(result, relay)
		}
	}

	return result
}
