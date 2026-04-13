package ratelimit

import "time"

// Bucket is a token-bucket spec for khatru policies (tokens added each Interval, max burst Max).
type Bucket struct {
	Tokens   int
	Interval time.Duration
	Max      int
}

// Enabled reports whether the bucket is configured for use.
func (b Bucket) Enabled() bool {
	return b.Tokens > 0 && b.Interval > 0 && b.Max > 0
}

// Config drives Apply on a khatru relay: connection / event / filter limits, soft rejects, optional close + ban.
type Config struct {
	Connection Bucket
	EventIP    Bucket
	FilterIP   Bucket

	// SoftRejectCount is how many rate-limit rejects per client IP only add a warning suffix before the next
	// reject also closes the WebSocket and may ban. Default 3 → strikes 1–3 soft, 4th closes.
	SoftRejectCount int
	// DisableDisconnect keeps rate-limit rejects soft only; no forced close and no IP ban state.
	// Default false (legacy behavior: close+ban path remains enabled).
	DisableDisconnect bool

	// BaseBanDuration is the first ban length after a forced close (and again after a clean probation). Zero disables all banning.
	BaseBanDuration time.Duration
	// MaxBanDuration caps escalating bans (each probation break scales the previous ban by RepeatOffenderMultiplier until this cap). Normalized default 24h if ≤ 0.
	MaxBanDuration time.Duration

	// ProbationMultiplier scales how long probation lasts after a ban: probation = lastBanDuration × ProbationMultiplier. Zero disables probation (no post-ban window; escalation reset when the ban ends).
	ProbationMultiplier float64
	// RepeatOffenderMultiplier scales the next ban after breaking probation: nextBan = lastBanDuration × RepeatOffenderMultiplier (then capped by MaxBanDuration). Zero disables escalation (next ban is BaseBanDuration).
	RepeatOffenderMultiplier float64

	// CloseReason is the WebSocket close payload (policy violation). Empty defaults to "rate limited".
	CloseReason string

	// LogDebug is optional (e.g. connect to verbose logging).
	LogDebug func(format string, args ...any)
	// OnPanic is called if closing the WebSocket panics; optional.
	OnPanic func(recovered any)
}

func (c *Config) normalize() {
	if c.SoftRejectCount <= 0 {
		c.SoftRejectCount = 3
	}
	if c.DisableDisconnect {
		c.BaseBanDuration = 0
	}
	if c.CloseReason == "" {
		c.CloseReason = "rate limited"
	}
	if c.MaxBanDuration <= 0 {
		c.MaxBanDuration = 24 * time.Hour
	}
}
