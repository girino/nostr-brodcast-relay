package manager

import (
	"sort"
	"sync"
	"time"

	"github.com/girino/broadcast-relay/logging"
)

type RelayInfo struct {
	URL                string
	AvgResponseTime    time.Duration
	SuccessRate        float64
	TotalAttempts      int64
	SuccessfulAttempts int64
	LastChecked        time.Time
}

type Manager struct {
	relays      map[string]*RelayInfo
	mu          sync.RWMutex
	decay       float64
	topN        int
	initialized bool
}

func NewManager(topN int, decay float64) *Manager {
	logging.Debug("Manager: Initializing manager: topN=%d, decay=%.2f", topN, decay)
	return &Manager{
		relays:      make(map[string]*RelayInfo),
		decay:       decay,
		topN:        topN,
		initialized: false,
	}
}

// AddRelay adds a new relay to the manager
func (m *Manager) AddRelay(url string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.relays[url]; !exists {
		m.relays[url] = &RelayInfo{
			URL:                url,
			AvgResponseTime:    0,
			SuccessRate:        1.0, // Start optimistic
			TotalAttempts:      0,
			SuccessfulAttempts: 0,
			LastChecked:        time.Now(),
		}
		logging.Debug("Manager: Added new relay: %s (total relays: %d)", url, len(m.relays))
	} else {
		logging.Debug("Manager: Relay already exists: %s", url)
	}
}

// UpdateHealth updates relay health after an initial check
func (m *Manager) UpdateHealth(url string, success bool, responseTime time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	relay, exists := m.relays[url]
	if !exists {
		logging.Warn("Manager: UpdateHealth called for unknown relay: %s", url)
		return
	}

	oldSuccessRate := relay.SuccessRate
	relay.TotalAttempts++

	if success {
		relay.SuccessfulAttempts++

		// Update average response time using exponential moving average
		if relay.AvgResponseTime == 0 {
			relay.AvgResponseTime = responseTime
		} else {
			relay.AvgResponseTime = time.Duration(
				float64(relay.AvgResponseTime)*0.7 + float64(responseTime)*0.3,
			)
		}

		logging.Debug("Manager: Health update SUCCESS: %s | attempts=%d/%d | responseTime=%.2fms",
			url, relay.SuccessfulAttempts, relay.TotalAttempts, responseTime.Seconds()*1000)
	} else {
		logging.Debug("Manager: Health update FAILED: %s | attempts=%d/%d",
			url, relay.SuccessfulAttempts, relay.TotalAttempts)
	}

	relay.LastChecked = time.Now()

	// After initialization, use exponential decay for success rate
	if m.initialized {
		successValue := 0.0
		if success {
			successValue = 1.0
		}
		relay.SuccessRate = relay.SuccessRate*m.decay + successValue*(1-m.decay)
		logging.Debug("Manager: Success rate updated (exponential decay): %s | %.4f -> %.4f",
			url, oldSuccessRate, relay.SuccessRate)
	} else {
		// During initialization, use simple success rate
		if relay.TotalAttempts > 0 {
			relay.SuccessRate = float64(relay.SuccessfulAttempts) / float64(relay.TotalAttempts)
			logging.Debug("Manager: Success rate updated (simple): %s | %.4f -> %.4f",
				url, oldSuccessRate, relay.SuccessRate)
		}
	}
}

// MarkInitialized marks the manager as initialized
func (m *Manager) MarkInitialized() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.initialized = true
	logging.Info("Manager: Initialization complete - switching to exponential decay mode")
	logging.Debug("Manager: Decay factor=%.2f, Total relays=%d", m.decay, len(m.relays))
}

// GetTopRelays returns the top N relays based on composite score
func (m *Manager) GetTopRelays() []*RelayInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	relays := make([]*RelayInfo, 0, len(m.relays))
	untested := 0
	for _, relay := range m.relays {
		// Only include relays that have been tested at least once
		if relay.TotalAttempts > 0 {
			relays = append(relays, relay)
		} else {
			untested++
		}
	}

	logging.Debug("Manager: GetTopRelays - %d tested relays, %d untested", len(relays), untested)

	// Sort by composite score
	sort.Slice(relays, func(i, j int) bool {
		scoreI := m.calculateScore(relays[i])
		scoreJ := m.calculateScore(relays[j])
		return scoreI > scoreJ
	})

	// Log top 5 for visibility (in verbose mode)
	if logging.Verbose {
		logCount := 5
		if len(relays) < logCount {
			logCount = len(relays)
		}
		for i := 0; i < logCount; i++ {
			r := relays[i]
			score := m.calculateScore(r)
			logging.Debug("Manager:   Top #%d: %s | score=%.2f | success=%.2f%% | avg_time=%.2fms | attempts=%d",
				i+1, r.URL, score, r.SuccessRate*100, r.AvgResponseTime.Seconds()*1000, r.TotalAttempts)
		}
	}

	// Return top N
	if len(relays) > m.topN {
		logging.Debug("Manager: Returning top %d out of %d tested relays", m.topN, len(relays))
		return relays[:m.topN]
	}
	logging.Debug("Manager: Returning all %d tested relays (less than topN=%d)", len(relays), m.topN)
	return relays
}

// CalculateScore computes a composite score for ranking
// Higher is better
func (m *Manager) CalculateScore(relay *RelayInfo) float64 {
	// Success rate weight
	successWeight := 100.0

	// Response time penalty (convert to seconds, higher response time = lower score)
	responseTimePenalty := 0.0
	if relay.AvgResponseTime > 0 {
		responseTimePenalty = relay.AvgResponseTime.Seconds() * 10.0
	}

	score := relay.SuccessRate*successWeight - responseTimePenalty

	// Penalize relays with very few attempts during initialization
	if !m.initialized && relay.TotalAttempts < 3 {
		score *= 0.5
	}

	return score
}

// calculateScore is the internal version
func (m *Manager) calculateScore(relay *RelayInfo) float64 {
	return m.CalculateScore(relay)
}

// GetAllRelays returns all relays (for discovery purposes)
func (m *Manager) GetAllRelays() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	urls := make([]string, 0, len(m.relays))
	for url := range m.relays {
		urls = append(urls, url)
	}
	return urls
}

// GetRelayCount returns the number of tracked relays
func (m *Manager) GetRelayCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.relays)
}

// RemoveRelay removes a relay from the manager
func (m *Manager) RemoveRelay(url string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.relays, url)
	logging.Info("Manager: Removed relay: %s", url)
}

// GetRelayInfo returns info about a specific relay
func (m *Manager) GetRelayInfo(url string) *RelayInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if relay, exists := m.relays[url]; exists {
		// Return a copy
		relayCopy := *relay
		return &relayCopy
	}
	return nil
}
