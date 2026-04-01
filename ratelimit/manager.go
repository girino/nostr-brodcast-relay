package ratelimit

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fiatjaf/khatru"
	"github.com/fiatjaf/khatru/policies"
	"github.com/nbd-wtf/go-nostr"
)

type strikeCounter struct{ n int32 }

// Manager holds rate-limit and temporary-ban state for one khatru relay. Create with New, then Apply.
type Manager struct {
	cfg Config

	strikes sync.Map // strike key (IP or ws:%p) -> *strikeCounter
	bans    sync.Map // client IP -> ban until time.Time
}

// New returns a Manager. cfg is copied and normalized (defaults for SoftRejectCount and CloseReason).
func New(cfg Config) *Manager {
	cfg.normalize()
	return &Manager{cfg: cfg}
}

func (m *Manager) logf(format string, args ...any) {
	if m.cfg.LogDebug != nil {
		m.cfg.LogDebug(format, args...)
	}
}

// Apply registers RejectConnection / RejectEvent / RejectFilter hooks on relay. Safe to call once per relay.
func (m *Manager) Apply(relay *khatru.Relay) {
	if relay == nil {
		return
	}
	if m.cfg.BanDuration > 0 {
		relay.RejectConnection = append([]func(*http.Request) bool{m.rejectConnectionIfBanned}, relay.RejectConnection...)
	}
	if m.cfg.Connection.Enabled() {
		limiter := policies.ConnectionRateLimiter(m.cfg.Connection.Tokens, m.cfg.Connection.Interval, m.cfg.Connection.Max)
		relay.RejectConnection = append(relay.RejectConnection, func(req *http.Request) bool {
			reject := limiter(req)
			if reject {
				m.logf("rateLimit connection rejected from %s", khatru.GetIPFromRequest(req))
			}
			return reject
		})
	}
	if m.cfg.EventIP.Enabled() {
		limiter := policies.EventIPRateLimiter(m.cfg.EventIP.Tokens, m.cfg.EventIP.Interval, m.cfg.EventIP.Max)
		relay.RejectEvent = append(relay.RejectEvent, func(ctx context.Context, event *nostr.Event) (bool, string) {
			reject, msg := limiter(ctx, event)
			if reject {
				return m.handleEventOrFilterReject(ctx, msg, "event")
			}
			return reject, msg
		})
	}
	if m.cfg.FilterIP.Enabled() {
		limiter := policies.FilterIPRateLimiter(m.cfg.FilterIP.Tokens, m.cfg.FilterIP.Interval, m.cfg.FilterIP.Max)
		relay.RejectFilter = append(relay.RejectFilter, func(ctx context.Context, filter nostr.Filter) (bool, string) {
			reject, msg := limiter(ctx, filter)
			if reject {
				return m.handleEventOrFilterReject(ctx, msg, "filter")
			}
			return reject, msg
		})
	}
}

func (m *Manager) handleEventOrFilterReject(ctx context.Context, policyMsg, kind string) (bool, string) {
	strike, closeConn, strikeKey, ip, ws := m.strikeAfterReject(ctx)
	wsTag := wsTag(ws)
	soft := m.cfg.SoftRejectCount
	if closeConn {
		m.logf("rateLimit closing ws=%s after %d strikes for key=%q IP=%s (%s): %s", wsTag, strike, strikeKey, ipLog(ip), kind, policyMsg)
		m.forcedDisconnect(ctx, strikeKey, ip)
		return true, policyMsg
	}
	m.logf("rateLimit warning %d/%d ws=%s key=%q IP=%s (%s): %s", strike, soft, wsTag, strikeKey, ipLog(ip), kind, policyMsg)
	warn := fmt.Sprintf("%s (warning %d/%d, connection closes on next rate limit)", policyMsg, strike, soft)
	return true, warn
}

func strikeKeyFromContext(ctx context.Context) (key string, ip string, ws *khatru.WebSocket) {
	ws = khatru.GetConnection(ctx)
	ip = khatru.GetIP(ctx)
	if ip != "" {
		return ip, ip, ws
	}
	if ws != nil {
		return fmt.Sprintf("ws:%p", ws), "", ws
	}
	return "", "", nil
}

func wsTag(ws *khatru.WebSocket) string {
	if ws == nil {
		return "-"
	}
	return fmt.Sprintf("%p", ws)
}

func ipLog(ip string) string {
	if ip == "" {
		return "-"
	}
	return ip
}

func (m *Manager) strikeAfterReject(ctx context.Context) (strike int32, shouldClose bool, strikeKey string, ip string, ws *khatru.WebSocket) {
	strikeKey, ip, ws = strikeKeyFromContext(ctx)
	if strikeKey == "" {
		return 0, false, "", ip, ws
	}
	actual, _ := m.strikes.LoadOrStore(strikeKey, new(strikeCounter))
	slot := actual.(*strikeCounter)
	n := atomic.AddInt32(&slot.n, 1)
	return n, n > int32(m.cfg.SoftRejectCount), strikeKey, ip, ws
}

func (m *Manager) rejectConnectionIfBanned(req *http.Request) bool {
	ip := khatru.GetIPFromRequest(req)
	if ip == "" {
		return false
	}
	v, ok := m.bans.Load(ip)
	if !ok {
		return false
	}
	until, ok := v.(time.Time)
	if !ok {
		m.bans.Delete(ip)
		return false
	}
	if time.Now().After(until) {
		m.bans.Delete(ip)
		return false
	}
	m.logf("rateLimit rejected connection from temporarily banned IP %s (until %s)", ip, until.UTC().Format(time.RFC3339))
	return true
}

func (m *Manager) forcedDisconnect(ctx context.Context, strikeKey, ip string) {
	CloseWebSocket(ctx, m.cfg.CloseReason, m.cfg.OnPanic)
	m.strikes.Delete(strikeKey)
	if m.cfg.BanDuration <= 0 || ip == "" {
		return
	}
	until := time.Now().Add(m.cfg.BanDuration)
	m.bans.Store(ip, until)
	m.logf("rateLimit temporarily banned IP %s from new connections until %s (window %v)", ip, until.UTC().Format(time.RFC3339), m.cfg.BanDuration)
}
