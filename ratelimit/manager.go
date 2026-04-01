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

// ipBanState tracks active ban, post-ban probation, and escalation (last ban length).
type ipBanState struct {
	mu sync.Mutex

	banUntil       time.Time
	probationUntil time.Time
	lastBanDur     time.Duration // length of the last applied ban; scales probation and next-ban duration via config multipliers
}

// Manager holds rate-limit and temporary-ban state for one khatru relay. Create with New, then Apply.
type Manager struct {
	cfg Config

	strikes  sync.Map // strike key (IP or ws:%p) -> *strikeCounter
	banByIP sync.Map // client IP -> *ipBanState
}

// New returns a Manager. cfg is copied and normalized (defaults for SoftRejectCount, CloseReason, MaxBanDuration).
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
	if m.cfg.BaseBanDuration > 0 {
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

func (m *Manager) banStateForIP(ip string) *ipBanState {
	if v, ok := m.banByIP.Load(ip); ok {
		return v.(*ipBanState)
	}
	ns := &ipBanState{}
	actual, _ := m.banByIP.LoadOrStore(ip, ns)
	return actual.(*ipBanState)
}

// durationMul returns d*mul as a duration, clamped to fit time.Duration.
func durationMul(d time.Duration, mul float64) time.Duration {
	if mul <= 0 || d <= 0 {
		return 0
	}
	v := float64(d) * mul
	max := float64((1<<63)-1) / 2 // stay below int64 max for safety
	if v > max {
		return time.Duration(max)
	}
	return time.Duration(v)
}

// refreshStateLocked advances ban → probation and clears expired probation (resets escalation).
func (m *Manager) refreshStateLocked(s *ipBanState, now time.Time) {
	if !s.banUntil.IsZero() && !now.Before(s.banUntil) {
		end := s.banUntil
		dur := s.lastBanDur
		s.banUntil = time.Time{}
		if m.cfg.ProbationMultiplier > 0 && dur > 0 {
			prob := durationMul(dur, m.cfg.ProbationMultiplier)
			s.probationUntil = end.Add(prob)
		} else {
			s.lastBanDur = 0
		}
	}
	if !s.probationUntil.IsZero() && !now.Before(s.probationUntil) {
		s.probationUntil = time.Time{}
		s.lastBanDur = 0
	}
}

func (m *Manager) removeBanStateIfIdle(ip string, s *ipBanState) {
	if s.banUntil.IsZero() && s.probationUntil.IsZero() && s.lastBanDur == 0 {
		m.banByIP.Delete(ip)
	}
}

func (m *Manager) rejectConnectionIfBanned(req *http.Request) bool {
	ip := khatru.GetIPFromRequest(req)
	if ip == "" {
		return false
	}
	now := time.Now()
	s := m.banStateForIP(ip)
	s.mu.Lock()
	defer s.mu.Unlock()
	m.refreshStateLocked(s, now)
	if !s.banUntil.IsZero() && now.Before(s.banUntil) {
		m.logf("rateLimit rejected connection from banned IP %s (until %s)", ip, s.banUntil.UTC().Format(time.RFC3339))
		return true
	}
	m.removeBanStateIfIdle(ip, s)
	return false
}

func (m *Manager) forcedDisconnect(ctx context.Context, strikeKey, ip string) {
	CloseWebSocket(ctx, m.cfg.CloseReason, m.cfg.OnPanic)
	m.strikes.Delete(strikeKey)
	if m.cfg.BaseBanDuration <= 0 || ip == "" {
		return
	}
	now := time.Now()
	s := m.banStateForIP(ip)
	s.mu.Lock()
	defer s.mu.Unlock()
	m.refreshStateLocked(s, now)

	// Already under an active ban (e.g. second open socket from same IP); do not shorten or reset banUntil.
	if !s.banUntil.IsZero() && now.Before(s.banUntil) {
		m.logf("rateLimit forced close for IP %s while ban still active until %s", ip, s.banUntil.UTC().Format(time.RFC3339))
		return
	}

	inProbation := !s.probationUntil.IsZero() && now.Before(s.probationUntil)
	var d time.Duration
	if inProbation {
		if s.lastBanDur <= 0 {
			d = m.cfg.BaseBanDuration
		} else if m.cfg.RepeatOffenderMultiplier <= 0 {
			d = m.cfg.BaseBanDuration
		} else {
			d = durationMul(s.lastBanDur, m.cfg.RepeatOffenderMultiplier)
			if d < s.lastBanDur { // overflow
				d = m.cfg.MaxBanDuration
			}
			if d > m.cfg.MaxBanDuration {
				d = m.cfg.MaxBanDuration
			}
		}
	} else {
		d = m.cfg.BaseBanDuration
	}

	s.banUntil = now.Add(d)
	s.lastBanDur = d
	s.probationUntil = time.Time{}

	if inProbation {
		m.logf("rateLimit banned IP %s until %s (duration %v, probation break, max %v)", ip, s.banUntil.UTC().Format(time.RFC3339), d, m.cfg.MaxBanDuration)
	} else {
		m.logf("rateLimit banned IP %s until %s (duration %v)", ip, s.banUntil.UTC().Format(time.RFC3339), d)
	}
}
