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
	return &Broadcaster{
		manager: mgr,
		checker: checker,
	}
}

// Broadcast sends an event to the top N relays concurrently
func (b *Broadcaster) Broadcast(event *nostr.Event) {
	topRelays := b.manager.GetTopRelays()
	
	if len(topRelays) == 0 {
		log.Println("Warning: No relays available for broadcasting")
		return
	}

	log.Printf("Broadcasting event %s to %d relays", event.ID, len(topRelays))

	var wg sync.WaitGroup
	
	for _, relayInfo := range topRelays {
		wg.Add(1)
		go func(url string) {
			defer wg.Done()
			b.publishToRelay(url, event)
		}(relayInfo.URL)
	}

	// Don't wait for completion - fire and forget
	// But track in background
	go func() {
		wg.Wait()
	}()
}

// publishToRelay publishes an event to a single relay and tracks the result
func (b *Broadcaster) publishToRelay(url string, event *nostr.Event) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	start := time.Now()
	
	relay, err := nostr.RelayConnect(ctx, url)
	if err != nil {
		b.checker.TrackPublishResult(health.PublishResult{
			URL:          url,
			Success:      false,
			ResponseTime: 0,
			Error:        err,
		})
		return
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
		log.Printf("Published event %s to %s (%.2fms)", event.ID, url, elapsed.Seconds()*1000)
	}
}

// GetStats returns current broadcast statistics
func (b *Broadcaster) GetStats() map[string]interface{} {
	topRelays := b.manager.GetTopRelays()
	totalRelays := b.manager.GetRelayCount()

	stats := map[string]interface{}{
		"total_relays":       totalRelays,
		"active_relays":      len(topRelays),
		"top_relays":         make([]map[string]interface{}, 0, len(topRelays)),
	}

	for i, relay := range topRelays {
		if i >= 10 { // Only show top 10 in stats
			break
		}
		relayStats := map[string]interface{}{
			"url":             relay.URL,
			"success_rate":    relay.SuccessRate,
			"avg_response_ms": relay.AvgResponseTime.Milliseconds(),
			"total_attempts":  relay.TotalAttempts,
		}
		stats["top_relays"] = append(stats["top_relays"].([]map[string]interface{}), relayStats)
	}

	return stats
}

