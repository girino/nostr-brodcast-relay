package broadcaster

import (
	"context"
	"sync"
	"sync/atomic"
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
	eventQueue      chan *nostr.Event
	overflowQueue   []*nostr.Event
	overflowMutex   sync.Mutex
	channelCapacity int
	totalQueued     int64
	peakQueueSize   int64
	saturationCount int64
	lastSaturation  time.Time
	workerCount     int
	ctx             context.Context
	cancel          context.CancelFunc
	wg              sync.WaitGroup
}

func NewBroadcaster(mgr *manager.Manager, checker *health.Checker, mandatoryRelays []string, workerCount int) *Broadcaster {
	logging.Debug("Broadcaster: Initializing broadcaster with %d workers", workerCount)
	if len(mandatoryRelays) > 0 {
		logging.Info("Broadcaster: Configured with %d mandatory relays", len(mandatoryRelays))
	}

	ctx, cancel := context.WithCancel(context.Background())
	channelCapacity := workerCount * 50

	logging.Info("Broadcaster: Channel capacity set to %d (50 * %d workers)", channelCapacity, workerCount)

	return &Broadcaster{
		manager:         mgr,
		checker:         checker,
		mandatoryRelays: mandatoryRelays,
		eventQueue:      make(chan *nostr.Event, channelCapacity),
		overflowQueue:   make([]*nostr.Event, 0),
		channelCapacity: channelCapacity,
		totalQueued:     0,
		peakQueueSize:   0,
		saturationCount: 0,
		workerCount:     workerCount,
		ctx:             ctx,
		cancel:          cancel,
	}
}

// Start initializes and starts the worker pool
func (b *Broadcaster) Start() {
	logging.Info("Broadcaster: Starting %d workers", b.workerCount)
	for i := 0; i < b.workerCount; i++ {
		b.wg.Add(1)
		go b.worker(i)
	}
}

// Stop gracefully shuts down the worker pool
func (b *Broadcaster) Stop() {
	logging.Info("Broadcaster: Stopping worker pool")
	b.cancel()
	close(b.eventQueue)
	b.wg.Wait()
	logging.Info("Broadcaster: All workers stopped")
}

// worker processes events from the queue
func (b *Broadcaster) worker(id int) {
	defer b.wg.Done()
	logging.Debug("Broadcaster: Worker %d started", id)

	for {
		select {
		case <-b.ctx.Done():
			logging.Debug("Broadcaster: Worker %d shutting down (context cancelled)", id)
			return
		case event, ok := <-b.eventQueue:
			if !ok {
				logging.Debug("Broadcaster: Worker %d shutting down (queue closed)", id)
				return
			}
			// Decrement total queued count
			atomic.AddInt64(&b.totalQueued, -1)

			// Try to backfill from overflow
			b.backfillChannel()

			// Broadcast the event
			b.broadcastEvent(event)
		}
	}
}

// backfillChannel attempts to move events from overflow queue to channel
func (b *Broadcaster) backfillChannel() {
	b.overflowMutex.Lock()
	defer b.overflowMutex.Unlock()

	// Move events from overflow to channel while there's space and overflow has events
	for len(b.overflowQueue) > 0 {
		select {
		case b.eventQueue <- b.overflowQueue[0]:
			// Successfully moved to channel, remove from overflow
			b.overflowQueue = b.overflowQueue[1:]
		default:
			// Channel is full, stop trying
			return
		}
	}
}

// Broadcast enqueues an event for broadcasting
func (b *Broadcaster) Broadcast(event *nostr.Event) {
	// Check if shutting down
	select {
	case <-b.ctx.Done():
		logging.Warn("Broadcaster: Cannot queue event %s, broadcaster is shutting down", event.ID)
		return
	default:
	}

	// Try to add to channel first (fast path)
	select {
	case b.eventQueue <- event:
		// Successfully queued to channel
		newTotal := atomic.AddInt64(&b.totalQueued, 1)
		logging.Debug("Broadcaster: Event %s (kind %d) queued to channel (total: %d)",
			event.ID, event.Kind, newTotal)

		// Update peak size
		for {
			peak := atomic.LoadInt64(&b.peakQueueSize)
			if newTotal <= peak || atomic.CompareAndSwapInt64(&b.peakQueueSize, peak, newTotal) {
				break
			}
		}
		return
	default:
		// Channel is full, add to overflow queue (slow path)
		b.overflowMutex.Lock()
		defer b.overflowMutex.Unlock()

		b.overflowQueue = append(b.overflowQueue, event)
		newTotal := atomic.AddInt64(&b.totalQueued, 1)

		// Track saturation
		if len(b.overflowQueue) == 1 {
			// First overflow, log warning
			atomic.AddInt64(&b.saturationCount, 1)
			b.lastSaturation = time.Now()
			logging.Warn("Broadcaster: Channel saturated (%d/%d), using overflow queue",
				len(b.eventQueue), b.channelCapacity)
		}

		logging.Debug("Broadcaster: Event %s (kind %d) queued to overflow (overflow: %d, total: %d)",
			event.ID, event.Kind, len(b.overflowQueue), newTotal)

		// Update peak size
		for {
			peak := atomic.LoadInt64(&b.peakQueueSize)
			if newTotal <= peak || atomic.CompareAndSwapInt64(&b.peakQueueSize, peak, newTotal) {
				break
			}
		}
	}
}

// broadcastEvent sends an event to the top N relays concurrently
func (b *Broadcaster) broadcastEvent(event *nostr.Event) {
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
		logging.Warn("Broadcaster: No relays available for broadcasting event %s (kind %d)", event.ID, event.Kind)
		return
	}

	logging.Debug("Broadcaster: Broadcasting event %s (kind %d) to %d relays (%d mandatory + %d top)",
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
		logging.Debug("Broadcaster: Broadcast complete for event %s | success=%d, failed=%d, total=%d",
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
		logging.Debug("Broadcaster: Failed to connect to %s: %v", url, err)
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
		logging.Debug("Broadcaster: Published event %s to %s (%.2fms)",
			event.ID, url, elapsed.Seconds()*1000)
	} else {
		logging.Debug("Broadcaster: Failed to publish to %s: %v (%.2fms)",
			url, err, elapsed.Seconds()*1000)
	}

	return success
}

// GetStats returns current broadcast statistics
func (b *Broadcaster) GetStats() map[string]interface{} {
	topRelays := b.manager.GetTopRelays()
	mandatoryRelays := b.manager.GetMandatoryRelays()
	totalRelays := b.manager.GetRelayCount()

	// Get queue stats
	b.overflowMutex.Lock()
	overflowSize := len(b.overflowQueue)
	b.overflowMutex.Unlock()

	channelSize := len(b.eventQueue)
	totalQueued := atomic.LoadInt64(&b.totalQueued)
	peakSize := atomic.LoadInt64(&b.peakQueueSize)
	saturationCount := atomic.LoadInt64(&b.saturationCount)
	channelUtilization := float64(channelSize) / float64(b.channelCapacity) * 100.0
	isSaturated := overflowSize > 0

	stats := map[string]interface{}{
		"total_relays":     totalRelays,
		"active_relays":    len(topRelays),
		"mandatory_relays": len(mandatoryRelays),
		"queue": map[string]interface{}{
			"worker_count":        b.workerCount,
			"channel_size":        channelSize,
			"channel_capacity":    b.channelCapacity,
			"channel_utilization": channelUtilization,
			"overflow_size":       overflowSize,
			"total_queued":        totalQueued,
			"peak_size":           peakSize,
			"saturation_count":    saturationCount,
			"is_saturated":        isSaturated,
			"last_saturation":     b.lastSaturation,
		},
		"mandatory_relay_list": make([]map[string]interface{}, 0, len(mandatoryRelays)),
		"top_relays":           make([]map[string]interface{}, 0, len(topRelays)),
	}

	// Show mandatory relays
	for _, relay := range mandatoryRelays {
		score := b.manager.CalculateScore(relay)
		relayStats := map[string]interface{}{
			"url":             relay.URL,
			"score":           score,
			"success_rate":    relay.SuccessRate,
			"avg_response_ms": relay.AvgResponseTime.Milliseconds(),
			"total_attempts":  relay.TotalAttempts,
		}
		stats["mandatory_relay_list"] = append(stats["mandatory_relay_list"].([]map[string]interface{}), relayStats)
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
