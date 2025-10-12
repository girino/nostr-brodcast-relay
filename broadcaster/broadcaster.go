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
	queueMutex      sync.RWMutex
	queueCapacity   int64
	queueSize       int64
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
	initialCapacity := int64(10000)

	return &Broadcaster{
		manager:         mgr,
		checker:         checker,
		mandatoryRelays: mandatoryRelays,
		eventQueue:      make(chan *nostr.Event, initialCapacity),
		queueCapacity:   initialCapacity,
		queueSize:       0,
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
		case event, ok := <-b.getEventQueue():
			if !ok {
				logging.Debug("Broadcaster: Worker %d shutting down (queue closed)", id)
				return
			}
			atomic.AddInt64(&b.queueSize, -1)
			b.broadcastEvent(event)
		}
	}
}

// getEventQueue returns the current event queue with read lock
func (b *Broadcaster) getEventQueue() <-chan *nostr.Event {
	b.queueMutex.RLock()
	defer b.queueMutex.RUnlock()
	return b.eventQueue
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

	// Try to enqueue with read lock first
	b.queueMutex.RLock()
	queue := b.eventQueue
	capacity := atomic.LoadInt64(&b.queueCapacity)
	currentSize := atomic.LoadInt64(&b.queueSize)
	b.queueMutex.RUnlock()

	select {
	case queue <- event:
		// Successfully queued
		newSize := atomic.AddInt64(&b.queueSize, 1)
		logging.Debug("Broadcaster: Event %s (kind %d) queued for broadcast (queue: %d/%d)",
			event.ID, event.Kind, newSize, capacity)

		// Update peak size
		for {
			peak := atomic.LoadInt64(&b.peakQueueSize)
			if newSize <= peak || atomic.CompareAndSwapInt64(&b.peakQueueSize, peak, newSize) {
				break
			}
		}
		return
	default:
		// Queue is full - need to grow it
		atomic.AddInt64(&b.saturationCount, 1)
		b.lastSaturation = time.Now()

		// Log warning (throttled)
		logging.Warn("Broadcaster: Queue saturated at %d/%d events, growing queue...",
			currentSize, capacity)

		// Grow the queue
		b.growQueue()

		// Try again with new queue
		b.queueMutex.RLock()
		queue = b.eventQueue
		b.queueMutex.RUnlock()

		select {
		case queue <- event:
			newSize := atomic.AddInt64(&b.queueSize, 1)
			logging.Debug("Broadcaster: Event %s queued after queue growth (queue: %d)", event.ID, newSize)
		default:
			// Still full? This shouldn't happen but handle it
			logging.Error("Broadcaster: Queue still full after growth, dropping event %s (kind %d)",
				event.ID, event.Kind)
		}
	}
}

// growQueue grows the event queue by 1.25x
func (b *Broadcaster) growQueue() {
	b.queueMutex.Lock()
	defer b.queueMutex.Unlock()

	oldCapacity := atomic.LoadInt64(&b.queueCapacity)
	newCapacity := int64(float64(oldCapacity) * 1.25)
	if newCapacity <= oldCapacity {
		newCapacity = oldCapacity + 1000 // Ensure at least some growth
	}

	logging.Info("Broadcaster: Growing queue from %d to %d", oldCapacity, newCapacity)

	// Create new queue
	newQueue := make(chan *nostr.Event, newCapacity)

	// Drain old queue into new queue
	oldQueue := b.eventQueue
	drained := 0
	drainLoop:
	for {
		select {
		case event, ok := <-oldQueue:
			if !ok {
				break drainLoop
			}
			newQueue <- event
			drained++
		default:
			break drainLoop
		}
	}

	logging.Debug("Broadcaster: Drained %d events from old queue to new queue", drained)

	// Swap the queue
	b.eventQueue = newQueue
	atomic.StoreInt64(&b.queueCapacity, newCapacity)
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
	totalRelays := b.manager.GetRelayCount()

	// Get queue stats
	currentSize := atomic.LoadInt64(&b.queueSize)
	capacity := atomic.LoadInt64(&b.queueCapacity)
	peakSize := atomic.LoadInt64(&b.peakQueueSize)
	saturationCount := atomic.LoadInt64(&b.saturationCount)
	utilizationPercent := float64(currentSize) / float64(capacity) * 100.0

	stats := map[string]interface{}{
		"total_relays":  totalRelays,
		"active_relays": len(topRelays),
		"queue": map[string]interface{}{
			"current_size":       currentSize,
			"capacity":           capacity,
			"peak_size":          peakSize,
			"utilization_pct":    utilizationPercent,
			"saturation_count":   saturationCount,
			"is_saturated":       currentSize >= capacity,
			"last_saturation":    b.lastSaturation,
		},
		"top_relays": make([]map[string]interface{}, 0, len(topRelays)),
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
