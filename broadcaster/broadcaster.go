package broadcaster

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/girino/broadcast-relay/health"
	"github.com/girino/broadcast-relay/logging"
	"github.com/girino/broadcast-relay/manager"
	"github.com/nbd-wtf/go-nostr"
)

type Broadcaster struct {
	manager         *manager.Manager
	checker         *health.Checker
	mandatoryRelays []string
}

func NewBroadcaster(mgr *manager.Manager, checker *health.Checker, mandatoryRelays []string) *Broadcaster {
	logging.LogV("[BROADCASTER] Initializing broadcaster")
	if len(mandatoryRelays) > 0 {
		log.Printf("[BROADCASTER] Configured with %d mandatory relays", len(mandatoryRelays))
	}
	return &Broadcaster{
		manager:         mgr,
		checker:         checker,
		mandatoryRelays: mandatoryRelays,
	}
}

// Broadcast sends an event to the top N relays concurrently
func (b *Broadcaster) Broadcast(event *nostr.Event) {
	topRelays := b.manager.GetTopRelays()

	// Build complete relay list: mandatory + top N
	relayURLs := make(map[string]bool)

	// Add mandatory relays first
	for _, url := range b.mandatoryRelays {
		relayURLs[url] = true
	}

	// Add top N relays
	for _, relayInfo := range topRelays {
		relayURLs[relayInfo.URL] = true
	}

	// Convert to slice
	broadcastRelays := make([]string, 0, len(relayURLs))
	for url := range relayURLs {
		broadcastRelays = append(broadcastRelays, url)
	}

	if len(broadcastRelays) == 0 {
		log.Printf("[BROADCASTER] WARNING: No relays available for broadcasting event %s (kind %d)", event.ID, event.Kind)
		return
	}

	logging.LogV("[BROADCASTER] Broadcasting event %s (kind %d) to %d relays (%d mandatory + %d top)",
		event.ID, event.Kind, len(broadcastRelays), len(b.mandatoryRelays), len(topRelays))

	var wg sync.WaitGroup
	successCount := 0
	failCount := 0
	var mu sync.Mutex

	for _, url := range broadcastRelays {
		wg.Add(1)
		go func(u string) {
			defer wg.Done()
			success := b.publishToRelay(u, event)
			mu.Lock()
			if success {
				successCount++
			} else {
				failCount++
			}
			mu.Unlock()
		}(url)
	}

	// Track results in background
	go func() {
		wg.Wait()
		logging.LogV("[BROADCASTER] Broadcast complete for event %s | success=%d, failed=%d, total=%d",
			event.ID, successCount, failCount, len(broadcastRelays))
	}()
}

// publishToRelay publishes an event to a single relay and tracks the result
func (b *Broadcaster) publishToRelay(url string, event *nostr.Event) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	start := time.Now()

	relay, err := nostr.RelayConnect(ctx, url)
	if err != nil {
		logging.LogV("[BROADCASTER] FAILED to connect to %s: %v", url, err)
		b.checker.TrackPublishResult(health.PublishResult{
			URL:          url,
			Success:      false,
			ResponseTime: 0,
			Error:        err,
		})
		return false
	}
	defer relay.Close()

	err = relay.Publish(ctx, *event)
	elapsed := time.Since(start)

	success := err == nil

	b.checker.TrackPublishResult(health.PublishResult{
		URL:          url,
		Success:      success,
		ResponseTime: elapsed,
		Error:        err,
	})

	if success {
		logging.LogV("[BROADCASTER] SUCCESS: Published event %s to %s (%.2fms)",
			event.ID, url, elapsed.Seconds()*1000)
	} else {
		logging.LogV("[BROADCASTER] FAILED to publish to %s: %v (%.2fms)",
			url, err, elapsed.Seconds()*1000)
	}

	return success
}

// GetStats returns current broadcast statistics
func (b *Broadcaster) GetStats() map[string]interface{} {
	topRelays := b.manager.GetTopRelays()
	totalRelays := b.manager.GetRelayCount()

	stats := map[string]interface{}{
		"total_relays":  totalRelays,
		"active_relays": len(topRelays),
		"top_relays":    make([]map[string]interface{}, 0, len(topRelays)),
	}

	// Show all active relays
	for _, relay := range topRelays {
		score := b.manager.CalculateScore(relay)
		relayStats := map[string]interface{}{
			"url":             relay.URL,
			"score":           score,
			"success_rate":    relay.SuccessRate,
			"avg_response_ms": relay.AvgResponseTime.Milliseconds(),
			"total_attempts":  relay.TotalAttempts,
		}
		stats["top_relays"] = append(stats["top_relays"].([]map[string]interface{}), relayStats)
	}

	return stats
}
