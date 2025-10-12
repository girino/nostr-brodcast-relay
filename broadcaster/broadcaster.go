package broadcaster

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/girino/broadcast-relay/health"
	"github.com/girino/broadcast-relay/manager"
	"github.com/nbd-wtf/go-nostr"
)

type Broadcaster struct {
	manager *manager.Manager
	checker *health.Checker
}

func NewBroadcaster(mgr *manager.Manager, checker *health.Checker) *Broadcaster {
	log.Println("[BROADCASTER] Initializing broadcaster")
	return &Broadcaster{
		manager: mgr,
		checker: checker,
	}
}

// Broadcast sends an event to the top N relays concurrently
func (b *Broadcaster) Broadcast(event *nostr.Event) {
	topRelays := b.manager.GetTopRelays()

	if len(topRelays) == 0 {
		log.Printf("[BROADCASTER] WARNING: No relays available for broadcasting event %s (kind %d)", event.ID, event.Kind)
		return
	}

	log.Printf("[BROADCASTER] ======================================")
	log.Printf("[BROADCASTER] Broadcasting event %s (kind %d) to %d relays", event.ID, event.Kind, len(topRelays))
	log.Printf("[BROADCASTER] Event author: %s", event.PubKey[:16]+"...")
	log.Printf("[BROADCASTER] Event content length: %d bytes", len(event.Content))

	var wg sync.WaitGroup
	successCount := 0
	failCount := 0
	var mu sync.Mutex

	for _, relayInfo := range topRelays {
		wg.Add(1)
		go func(url string) {
			defer wg.Done()
			success := b.publishToRelay(url, event)
			mu.Lock()
			if success {
				successCount++
			} else {
				failCount++
			}
			mu.Unlock()
		}(relayInfo.URL)
	}

	// Track results in background
	go func() {
		wg.Wait()
		log.Printf("[BROADCASTER] Broadcast complete for event %s | success=%d, failed=%d, total=%d",
			event.ID, successCount, failCount, len(topRelays))
	}()
}

// publishToRelay publishes an event to a single relay and tracks the result
func (b *Broadcaster) publishToRelay(url string, event *nostr.Event) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	start := time.Now()

	log.Printf("[BROADCASTER] Publishing to %s...", url)

	relay, err := nostr.RelayConnect(ctx, url)
	if err != nil {
		log.Printf("[BROADCASTER] FAILED to connect to %s: %v", url, err)
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
		log.Printf("[BROADCASTER] SUCCESS: Published event %s to %s (%.2fms)",
			event.ID, url, elapsed.Seconds()*1000)
	} else {
		log.Printf("[BROADCASTER] FAILED to publish to %s: %v (%.2fms)",
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
