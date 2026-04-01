// Package ratelimit provides khatru relay hooks for per-IP token-bucket limits (connection, event, filter),
// progressive warnings, forced WebSocket close, and optional temporary IP ban on new upgrades.
//
// Usage:
//
//	mgr := ratelimit.New(ratelimit.Config{
//	    EventIP:    ratelimit.Bucket{Tokens: 2, Interval: 3 * time.Minute, Max: 10},
//	    FilterIP:   ratelimit.Bucket{Tokens: 20, Interval: time.Minute, Max: 100},
//	    BanDuration: 5 * time.Minute,
//	    LogDebug: log.Printf,
//	})
//	mgr.Apply(relay)
package ratelimit
