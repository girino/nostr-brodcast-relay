package ratelimit

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
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

	logMu   sync.Mutex
	logFile *os.File
}

// New returns a Manager. cfg is copied and normalized (defaults for SoftRejectCount, CloseReason, MaxBanDuration).
func New(cfg Config) *Manager {
	cfg.normalize()
	m := &Manager{cfg: cfg}
	m.initLogFile()
	return m
}

func (m *Manager) logf(format string, args ...any) {
	if m.cfg.LogDebug != nil {
		m.cfg.LogDebug(format, args...)
	}
}

func (m *Manager) initLogFile() {
	if m.cfg.LogFilePath == "" {
		return
	}
	dir := filepath.Dir(m.cfg.LogFilePath)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			m.logf("rateLimit failed creating log dir %q: %v", dir, err)
			return
		}
	}
	f, err := os.OpenFile(m.cfg.LogFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		m.logf("rateLimit failed opening log file %q: %v", m.cfg.LogFilePath, err)
		return
	}
	m.logFile = f
}

type requestSnapshot struct {
	Method     string              `json:"method"`
	URL        string              `json:"url"`
	Path       string              `json:"path"`
	RawQuery   string              `json:"raw_query"`
	RemoteAddr string              `json:"remote_addr"`
	Host       string              `json:"host"`
	UserAgent  string              `json:"user_agent,omitempty"`
	Referer    string              `json:"referer,omitempty"`
	Origin     string              `json:"origin,omitempty"`
	Headers    map[string][]string `json:"headers,omitempty"`
	Cookies    map[string]string   `json:"cookies,omitempty"`
}

type rateLimitLogEntry struct {
	Timestamp      string           `json:"timestamp"`
	Action         string           `json:"action"`
	Decision       string           `json:"decision"`
	Reason         string           `json:"reason,omitempty"`
	Kind           string           `json:"kind,omitempty"`
	IP             string           `json:"ip,omitempty"`
	StrikeKey      string           `json:"strike_key,omitempty"`
	Strike         int32            `json:"strike,omitempty"`
	SoftRejects    int              `json:"soft_reject_count,omitempty"`
	DisableClose   bool             `json:"disable_disconnect,omitempty"`
	BanUntil       string           `json:"ban_until,omitempty"`
	ProbationUntil string           `json:"probation_until,omitempty"`
	WS             string           `json:"ws,omitempty"`
	Request        *requestSnapshot `json:"request,omitempty"`
	Event          any              `json:"event,omitempty"`
	Filter         any              `json:"filter,omitempty"`
}

func requestFromContext(ctx context.Context) *http.Request {
	ws := khatru.GetConnection(ctx)
	if ws == nil {
		return nil
	}
	return ws.Request
}

func snapshotRequest(req *http.Request) *requestSnapshot {
	if req == nil {
		return nil
	}
	headers := make(map[string][]string, len(req.Header))
	for k, v := range req.Header {
		copied := append([]string(nil), v...)
		headers[k] = copied
	}
	cookies := make(map[string]string)
	for _, c := range req.Cookies() {
		cookies[c.Name] = c.Value
	}
	urlVal := ""
	path := ""
	rawQuery := ""
	if req.URL != nil {
		urlVal = req.URL.String()
		path = req.URL.Path
		rawQuery = req.URL.RawQuery
	}
	return &requestSnapshot{
		Method:     req.Method,
		URL:        urlVal,
		Path:       path,
		RawQuery:   rawQuery,
		RemoteAddr: req.RemoteAddr,
		Host:       req.Host,
		UserAgent:  req.UserAgent(),
		Referer:    req.Referer(),
		Origin:     req.Header.Get("Origin"),
		Headers:    headers,
		Cookies:    cookies,
	}
}

func (m *Manager) writeJSONLog(entry rateLimitLogEntry) {
	if m.logFile == nil {
		return
	}
	entry.Timestamp = time.Now().UTC().Format(time.RFC3339Nano)
	b, err := json.Marshal(entry)
	if err != nil {
		m.logf("rateLimit failed marshaling JSON log: %v", err)
		return
	}
	m.logMu.Lock()
	defer m.logMu.Unlock()
	if _, err := m.logFile.Write(append(b, '\n')); err != nil {
		m.logf("rateLimit failed writing JSON log: %v", err)
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
				ip := khatru.GetIPFromRequest(req)
				m.logf("rateLimit connection rejected from %s", ip)
				m.writeJSONLog(rateLimitLogEntry{
					Action:   "connection",
					Decision: "rejected",
					Reason:   "connection rate limit exceeded",
					IP:       ip,
					Request:  snapshotRequest(req),
				})
			}
			return reject
		})
	}
	if m.cfg.EventIP.Enabled() {
		limiter := policies.EventIPRateLimiter(m.cfg.EventIP.Tokens, m.cfg.EventIP.Interval, m.cfg.EventIP.Max)
		relay.RejectEvent = append(relay.RejectEvent, func(ctx context.Context, event *nostr.Event) (bool, string) {
			reject, msg := limiter(ctx, event)
			if reject {
				return m.handleEventOrFilterReject(ctx, msg, "event", event, nil)
			}
			return reject, msg
		})
	}
	if m.cfg.FilterIP.Enabled() {
		limiter := policies.FilterIPRateLimiter(m.cfg.FilterIP.Tokens, m.cfg.FilterIP.Interval, m.cfg.FilterIP.Max)
		relay.RejectFilter = append(relay.RejectFilter, func(ctx context.Context, filter nostr.Filter) (bool, string) {
			reject, msg := limiter(ctx, filter)
			if reject {
				return m.handleEventOrFilterReject(ctx, msg, "filter", nil, &filter)
			}
			return reject, msg
		})
	}
}

func (m *Manager) handleEventOrFilterReject(ctx context.Context, policyMsg, kind string, event *nostr.Event, filter *nostr.Filter) (bool, string) {
	strike, closeConn, strikeKey, ip, ws := m.strikeAfterReject(ctx)
	wsTag := wsTag(ws)
	soft := m.cfg.SoftRejectCount
	req := requestFromContext(ctx)
	if m.cfg.DisableDisconnect {
		m.logf("rateLimit soft reject %d ws=%s key=%q IP=%s (%s): %s", strike, wsTag, strikeKey, ipLog(ip), kind, policyMsg)
		m.writeJSONLog(rateLimitLogEntry{
			Action:       "event_filter",
			Decision:     "soft_reject",
			Reason:       policyMsg,
			Kind:         kind,
			IP:           ip,
			StrikeKey:    strikeKey,
			Strike:       strike,
			SoftRejects:  soft,
			DisableClose: true,
			WS:           wsTag,
			Request:      snapshotRequest(req),
			Event:        event,
			Filter:       filter,
		})
		return true, policyMsg
	}
	if closeConn {
		m.logf("rateLimit closing ws=%s after %d strikes for key=%q IP=%s (%s): %s", wsTag, strike, strikeKey, ipLog(ip), kind, policyMsg)
		m.writeJSONLog(rateLimitLogEntry{
			Action:      "event_filter",
			Decision:    "forced_disconnect",
			Reason:      policyMsg,
			Kind:        kind,
			IP:          ip,
			StrikeKey:   strikeKey,
			Strike:      strike,
			SoftRejects: soft,
			WS:          wsTag,
			Request:     snapshotRequest(req),
			Event:       event,
			Filter:      filter,
		})
		m.forcedDisconnect(ctx, strikeKey, ip)
		return true, policyMsg
	}
	m.logf("rateLimit warning %d/%d ws=%s key=%q IP=%s (%s): %s", strike, soft, wsTag, strikeKey, ipLog(ip), kind, policyMsg)
	m.writeJSONLog(rateLimitLogEntry{
		Action:      "event_filter",
		Decision:    "warning",
		Reason:      policyMsg,
		Kind:        kind,
		IP:          ip,
		StrikeKey:   strikeKey,
		Strike:      strike,
		SoftRejects: soft,
		WS:          wsTag,
		Request:     snapshotRequest(req),
		Event:       event,
		Filter:      filter,
	})
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
		m.writeJSONLog(rateLimitLogEntry{
			Action:   "connection",
			Decision: "rejected_banned_ip",
			Reason:   "ip currently banned",
			IP:       ip,
			BanUntil: s.banUntil.UTC().Format(time.RFC3339),
			Request:  snapshotRequest(req),
		})
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
	entry := rateLimitLogEntry{
		Action:      "ban",
		Decision:    "banned_ip",
		Reason:      "forced disconnect after rate-limit strikes",
		IP:          ip,
		StrikeKey:   strikeKey,
		BanUntil:    s.banUntil.UTC().Format(time.RFC3339),
		WS:          wsTag(khatru.GetConnection(ctx)),
		Request:     snapshotRequest(requestFromContext(ctx)),
		SoftRejects: m.cfg.SoftRejectCount,
	}
	if !s.probationUntil.IsZero() {
		entry.ProbationUntil = s.probationUntil.UTC().Format(time.RFC3339)
	}
	m.writeJSONLog(entry)
}
