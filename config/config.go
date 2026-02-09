package config

import (
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/girino/nostr-lib/logging"
)

// RateLimitConfig holds token-bucket parameters: tokens per interval, refill interval, max burst.
// Used for khatru connection, event-by-IP, and filter-by-IP rate limiters.
type RateLimitConfig struct {
	Tokens   int           // tokens added each interval
	Interval time.Duration // refill interval
	Max      int           // max tokens (burst)
}

// Enabled returns true if this rate limit is configured (all values > 0).
func (r *RateLimitConfig) Enabled() bool {
	return r.Tokens > 0 && r.Interval > 0 && r.Max > 0
}

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
	CacheTTL            time.Duration
	Verbose             string
	// Relay metadata
	RelayName        string
	RelayDescription string
	RelayURL         string
	ContactPubkey    string
	RelayPrivkey     string
	RelayIcon        string
	RelayBanners     []string
	// Rate limiting (khatru policies): connection per IP, events per IP, filters (REQ) per IP
	RateLimitConnection RateLimitConfig // e.g. 5 connections per 1m, burst 20
	RateLimitEventIP    RateLimitConfig // e.g. 10 events per 1s per IP, burst 30
	RateLimitFilterIP   RateLimitConfig // e.g. 20 REQ per 1m per IP, burst 100
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
		CacheTTL:            getEnvDuration("CACHE_TTL", 5*time.Minute),
		Verbose:             getEnv("VERBOSE", ""),
		// Relay metadata
		RelayName:        getEnv("RELAY_NAME", "Broadcast Relay"),
		RelayDescription: getEnv("RELAY_DESCRIPTION", "A Nostr relay that broadcasts events to multiple relays"),
		RelayURL:         getEnv("RELAY_URL", ""),
		ContactPubkey:    getEnv("CONTACT_PUBKEY", ""),
		RelayPrivkey:     getEnv("RELAY_PRIVKEY", ""),
		RelayIcon:        getEnv("RELAY_ICON", "/static/icon1.png"),
		RelayBanners:     parseBannerList(getEnv("RELAY_BANNERS", "")),
		// Rate limits: enabled by default, matching khatru policies.ApplySaneDefaults. Format "tokens,interval,max". Use "0,0,0" or "off" to disable.
		RateLimitConnection: parseRateLimitWithDefault(getEnv("RATE_LIMIT_CONNECTION", "1,5m,100"), "1,5m,100"),   // 1 connection per 5m per IP, burst 100
		RateLimitEventIP:    parseRateLimitWithDefault(getEnv("RATE_LIMIT_EVENT_IP", "2,3m,10"), "2,3m,10"),     // 2 events per 3m per IP, burst 10
		RateLimitFilterIP:   parseRateLimitWithDefault(getEnv("RATE_LIMIT_FILTER_IP", "20,1m,100"), "20,1m,100"), // 20 REQ per 1m per IP, burst 100
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

// parseRateLimit parses "tokens,interval,max" e.g. "5,1m,20". Returns zeroed config on parse error.
func parseRateLimit(s string) RateLimitConfig {
	s = strings.TrimSpace(s)
	if s == "" {
		return RateLimitConfig{}
	}
	parts := strings.Split(s, ",")
	if len(parts) != 3 {
		return RateLimitConfig{}
	}
	tokens, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
	interval, err2 := time.ParseDuration(strings.TrimSpace(parts[1]))
	max, err3 := strconv.Atoi(strings.TrimSpace(parts[2]))
	if err1 != nil || err2 != nil || err3 != nil || tokens < 0 || interval < 0 || max < 0 {
		return RateLimitConfig{}
	}
	return RateLimitConfig{Tokens: tokens, Interval: interval, Max: max}
}

// parseRateLimitWithDefault returns parsed config, or defaultConfig if env is invalid. Use "0,0,0" or "off" to disable.
func parseRateLimitWithDefault(envValue, defaultStr string) RateLimitConfig {
	envValue = strings.TrimSpace(strings.ToLower(envValue))
	if envValue == "off" || envValue == "0,0,0" {
		return RateLimitConfig{}
	}
	parsed := parseRateLimit(envValue)
	if parsed.Enabled() {
		return parsed
	}
	return parseRateLimit(defaultStr)
}

func parseBannerList(bannerStr string) []string {
	if bannerStr == "" {
		// Default to local static banners
		return []string{
			"/static/banner1.png",
			"/static/banner2.png",
			"/static/banner3.png",
			"/static/banner4.png",
			"/static/banner5.png",
			"/static/banner6.png",
		}
	}

	// Parse comma-separated list (same as parseSeedRelays)
	banners := strings.Split(bannerStr, ",")
	result := make([]string, 0, len(banners))

	for _, banner := range banners {
		banner = strings.TrimSpace(banner)
		if banner != "" {
			result = append(result, banner)
		}
	}

	return result
}
