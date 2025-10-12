package health

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/girino/broadcast-relay/manager"
	"github.com/nbd-wtf/go-nostr"
)

type Checker struct {
	manager        *manager.Manager
	initialTimeout time.Duration
}

func NewChecker(mgr *manager.Manager, initialTimeout time.Duration) *Checker {
	return &Checker{
		manager:        mgr,
		initialTimeout: initialTimeout,
	}
}

// CheckInitial performs initial timeout-based health check on a relay
func (c *Checker) CheckInitial(url string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), c.initialTimeout)
	defer cancel()

	start := time.Now()
	relay, err := nostr.RelayConnect(ctx, url)
	if err != nil {
		log.Printf("Failed to connect to relay %s: %v", url, err)
		c.manager.UpdateHealth(url, false, 0)
		return false
	}
	defer relay.Close()

	elapsed := time.Since(start)
	
	// Consider it successful if we connected
	c.manager.UpdateHealth(url, true, elapsed)
	log.Printf("Relay %s initial check: OK (%.2fms)", url, elapsed.Seconds()*1000)
	return true
}

// CheckBatch performs initial checks on multiple relays concurrently
func (c *Checker) CheckBatch(urls []string) {
	sem := make(chan struct{}, 20) // Limit concurrent checks
	var wg sync.WaitGroup

	for _, url := range urls {
		wg.Add(1)
		go func(u string) {
			defer wg.Done()
			sem <- struct{}{}        // Acquire semaphore
			defer func() { <-sem }() // Release semaphore
			
			c.CheckInitial(u)
		}(url)
	}

	wg.Wait()
}

// PublishResult tracks the result of a publish attempt
type PublishResult struct {
	URL          string
	Success      bool
	ResponseTime time.Duration
	Error        error
}

// TrackPublishResult updates relay health based on publish results
func (c *Checker) TrackPublishResult(result PublishResult) {
	c.manager.UpdateHealth(result.URL, result.Success, result.ResponseTime)
	
	if !result.Success && result.Error != nil {
		log.Printf("Publish to %s failed: %v", result.URL, result.Error)
	}
}

