package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	SeedRelays          []string
	TopNRelays          int
	RelayPort           string
	RefreshInterval     time.Duration
	HealthCheckInterval time.Duration
	InitialTimeout      time.Duration
	SuccessRateDecay    float64
}

func Load() *Config {
	return &Config{
		SeedRelays:          parseSeedRelays(getEnv("SEED_RELAYS", "")),
		TopNRelays:          getEnvInt("TOP_N_RELAYS", 50),
		RelayPort:           getEnv("RELAY_PORT", "3334"),
		RefreshInterval:     getEnvDuration("REFRESH_INTERVAL", 24*time.Hour),
		HealthCheckInterval: getEnvDuration("HEALTH_CHECK_INTERVAL", 5*time.Minute),
		InitialTimeout:      getEnvDuration("INITIAL_TIMEOUT", 5*time.Second),
		SuccessRateDecay:    getEnvFloat("SUCCESS_RATE_DECAY", 0.95),
	}
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
