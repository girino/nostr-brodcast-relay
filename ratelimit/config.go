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

// Config drives Apply on a khatru relay: connection / event / filter limits, soft rejects, post-close IP ban.
type Config struct {
	Connection Bucket
	EventIP    Bucket
	FilterIP   Bucket

	// SoftRejectCount is how many rate-limit rejects per client IP only add a warning suffix before the next
	// reject also closes the WebSocket and may ban. Default 3 → strikes 1–3 soft, 4th closes.
	SoftRejectCount int
	// BanDuration blocks new WebSocket upgrades from that client IP after a forced close. Zero disables.
	BanDuration time.Duration

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
	if c.CloseReason == "" {
		c.CloseReason = "rate limited"
	}
}
