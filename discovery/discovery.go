package discovery

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/girino/broadcast-relay/health"
	"github.com/girino/broadcast-relay/logging"
	"github.com/girino/broadcast-relay/manager"
	"github.com/nbd-wtf/go-nostr"
)

type Discovery struct {
	manager *manager.Manager
	checker *health.Checker
}

func NewDiscovery(mgr *manager.Manager, checker *health.Checker) *Discovery {
	logging.Debug("Discovery: Initializing discovery module")
	return &Discovery{
		manager: mgr,
		checker: checker,
	}
}

// DiscoverFromSeeds performs initial relay discovery from seed relays
func (d *Discovery) DiscoverFromSeeds(ctx context.Context, seedRelays []string) {
	logging.Debug("Discovery: Starting seed discovery")
	logging.Info("Discovery: Using %d seed relays", len(seedRelays))

	// First, add seed relays to manager
	for _, seed := range seedRelays {
		logging.Debug("Discovery: Adding seed relay: %s", seed)
		d.manager.AddRelay(seed)
	}

	// Discover more relays from seeds
	relayURLs := make(map[string]bool)

	for i, seedURL := range seedRelays {
		logging.Debug("Discovery: Fetching relay lists from seed %d/%d: %s", i+1, len(seedRelays), seedURL)
		relays := d.fetchRelaysFromRelay(ctx, seedURL)
		for _, relay := range relays {
			relayURLs[relay] = true
		}
	}

	logging.Debug("Discovery: Found %d unique relay URLs from seeds", len(relayURLs))

	// Add discovered relays
	newRelays := []string{}
	for url := range relayURLs {
		if !d.isAlreadyKnown(url) {
			d.manager.AddRelay(url)
			newRelays = append(newRelays, url)
		}
	}

	logging.Info("Discovery: Added %d new relays from discovery", len(newRelays))

	// Test all relays (seeds + discovered)
	allRelays := d.manager.GetAllRelays()
	d.checker.CheckBatch(allRelays)
}

// fetchRelaysFromRelay fetches relay lists from a specific relay
func (d *Discovery) fetchRelaysFromRelay(ctx context.Context, relayURL string) []string {
	relaySet := make(map[string]bool)

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	relay, err := nostr.RelayConnect(ctx, relayURL)
	if err != nil {
		logging.Debug("Discovery: Failed to connect to seed relay %s: %v", relayURL, err)
		return []string{}
	}
	defer relay.Close()

	// Fetch kind 3 (contact lists) and kind 10002 (relay lists)
	filters := []nostr.Filter{
		{
			Kinds: []int{3, 10002},
			Limit: 100,
		},
	}

	sub, err := relay.Subscribe(ctx, filters)
	if err != nil {
		logging.Debug("Discovery: Failed to subscribe to %s: %v", relayURL, err)
		return []string{}
	}

	// Collect events with timeout
	timeout := time.After(10 * time.Second)

	for {
		select {
		case event := <-sub.Events:
			if event == nil {
				continue
			}
			relays := d.extractRelaysFromEvent(event)
			for _, r := range relays {
				relaySet[r] = true
			}
		case <-sub.EndOfStoredEvents:
			// Got all stored events
			goto done
		case <-timeout:
			// Timeout reached
			goto done
		case <-ctx.Done():
			goto done
		}
	}

done:
	sub.Unsub()

	result := make([]string, 0, len(relaySet))
	for relay := range relaySet {
		result = append(result, relay)
	}

	logging.Debug("Discovery: Fetched %d relay URLs from %s", len(result), relayURL)
	return result
}

// ExtractRelaysFromEvent extracts relay URLs from a Nostr event
func (d *Discovery) ExtractRelaysFromEvent(event *nostr.Event) []string {
	return d.extractRelaysFromEvent(event)
}

func (d *Discovery) extractRelaysFromEvent(event *nostr.Event) []string {
	relays := []string{}

	switch event.Kind {
	case 3: // Contact list
		// Relays might be in content (old format) or tags
		if event.Content != "" {
			relays = append(relays, d.parseContactListContent(event.Content)...)
		}

	case 10002: // Relay list metadata (NIP-65)
		// Format: ["r", "<relay-url>", "<read|write>"]
		for _, tag := range event.Tags {
			if len(tag) >= 2 && tag[0] == "r" {
				relay := normalizeRelayURL(tag[1])
				if relay != "" {
					relays = append(relays, relay)
				}
			}
		}
	}

	// Extract relay hints from all events (relay hints in tags)
	for _, tag := range event.Tags {
		// Check for relay hints in 'e' and 'p' tags
		// Format: ["e", "<event-id>", "<relay-url>"]
		// Format: ["p", "<pubkey>", "<relay-url>"]
		if (tag[0] == "e" || tag[0] == "p") && len(tag) >= 3 {
			relay := normalizeRelayURL(tag[2])
			if relay != "" {
				relays = append(relays, relay)
			}
		}
	}

	return relays
}

// parseContactListContent parses relay URLs from kind 3 content
func (d *Discovery) parseContactListContent(content string) []string {
	relays := []string{}

	// Content is typically a JSON object with relay URLs as keys
	var relayMap map[string]interface{}
	if err := json.Unmarshal([]byte(content), &relayMap); err != nil {
		return relays
	}

	for relayURL := range relayMap {
		relay := normalizeRelayURL(relayURL)
		if relay != "" {
			relays = append(relays, relay)
		}
	}

	return relays
}

// normalizeRelayURL normalizes and validates a relay URL
func normalizeRelayURL(url string) string {
	url = strings.TrimSpace(url)

	if url == "" {
		return ""
	}

	// Must start with ws:// or wss://
	if !strings.HasPrefix(url, "ws://") && !strings.HasPrefix(url, "wss://") {
		return ""
	}

	// Remove trailing slash
	url = strings.TrimSuffix(url, "/")

	return url
}

// isAlreadyKnown checks if a relay is already tracked
func (d *Discovery) isAlreadyKnown(url string) bool {
	return d.manager.GetRelayInfo(url) != nil
}

// AddRelayIfNew adds a relay if it's not already known and tests it
func (d *Discovery) AddRelayIfNew(url string) {
	url = normalizeRelayURL(url)
	if url == "" {
		return
	}

	if !d.isAlreadyKnown(url) {
		logging.Debug("Discovery: New relay discovered: %s (testing...)", url)
		d.manager.AddRelay(url)
		// Test the new relay
		go d.checker.CheckInitial(url)
	}
}
