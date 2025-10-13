package relay

import (
	"context"
	"fmt"
	"html/template"
	"math/rand"
	"net/http"

	"github.com/fiatjaf/khatru"
	"github.com/girino/broadcast-relay/broadcaster"
	"github.com/girino/broadcast-relay/config"
	"github.com/girino/broadcast-relay/discovery"
	"github.com/girino/broadcast-relay/logging"
	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip19"
)

type Relay struct {
	khatru      *khatru.Relay
	broadcaster *broadcaster.Broadcaster
	discovery   *discovery.Discovery
	config      *config.Config
	port        string
}

func NewRelay(cfg *config.Config, bc *broadcaster.Broadcaster, disc *discovery.Discovery) *Relay {
	r := &Relay{
		khatru:      khatru.NewRelay(),
		broadcaster: bc,
		discovery:   disc,
		config:      cfg,
		port:        cfg.RelayPort,
	}

	r.setupRelay()
	return r
}

func (r *Relay) setupRelay() {
	relay := r.khatru

	// Generate or derive relay privkey/pubkey
	relayPrivkey := r.config.RelayPrivkey
	relayPubkey := ""

	if relayPrivkey != "" {
		// Decode provided nsec to get private key
		if _, decoded, err := nip19.Decode(relayPrivkey); err == nil {
			if sk, ok := decoded.(string); ok {
				relayPrivkey = sk
				if pk, err := nostr.GetPublicKey(sk); err == nil {
					relayPubkey = pk
					logging.DebugMethod("relay", "setupRelay", "Using provided relay key, pubkey: %s", pk)
				}
			}
		}
	} else {
		// Generate a random key
		relayPrivkey = nostr.GeneratePrivateKey()
		if pk, err := nostr.GetPublicKey(relayPrivkey); err == nil {
			relayPubkey = pk
			logging.Info("Relay: Generated random relay keypair, pubkey: %s", pk)
		}
	}

	// Set default URL if not configured
	relayURL := r.config.RelayURL
	if relayURL == "" {
		relayURL = fmt.Sprintf("ws://localhost:%s", r.config.RelayPort)
		logging.DebugMethod("relay", "setupRelay", "Using default relay URL: %s", relayURL)
	}

	// Set default contact to relay pubkey if not configured
	contactPubkey := r.config.ContactPubkey
	if contactPubkey == "" {
		contactPubkey = relayPubkey
		logging.DebugMethod("relay", "setupRelay", "Using relay pubkey as contact (not configured separately)")
	}

	// Update config with defaults for template rendering
	r.config.RelayURL = relayURL
	r.config.ContactPubkey = contactPubkey

	// Set relay metadata from config
	relay.Info.Name = r.config.RelayName
	relay.Info.Description = r.config.RelayDescription
	relay.Info.PubKey = relayPubkey
	relay.Info.Contact = contactPubkey
	relay.Info.SupportedNIPs = []any{1, 11}
	relay.Info.Software = "https://gitworkshop.dev/girino@girino.org/broadcast-relay"
	relay.Info.Version = "1.0.0"
	relay.Info.Icon = r.config.RelayIcon

	// Note: Banner is shown on main page but not in NIP-11 (not a standard field)

	// Reject cached events (duplicates)
	relay.RejectEvent = append(relay.RejectEvent,
		func(ctx context.Context, event *nostr.Event) (bool, string) {
			// Check if event was already broadcast
			if r.broadcaster.IsEventCached(event.ID) {
				logging.DebugMethod("relay", "RejectEvent", "Rejecting duplicate event %s (kind %d)", event.ID, event.Kind)
				return true, "duplicate: event already broadcast"
			}
			return false, ""
		},
	)

	// Handle incoming events (both regular and ephemeral)
	relay.OnEventSaved = append(relay.OnEventSaved,
		func(ctx context.Context, event *nostr.Event) {
			r.handleEvent(event)
		},
	)

	// Handle ephemeral events (kinds 20000-29999) with the same handler
	relay.OnEphemeralEvent = append(relay.OnEphemeralEvent,
		func(ctx context.Context, event *nostr.Event) {
			r.handleEvent(event)
		},
	)

	// Don't store events - override the store handler
	relay.StoreEvent = append(relay.StoreEvent,
		func(ctx context.Context, event *nostr.Event) error {
			// Don't store, just return success
			return nil
		},
	)

	// Don't query events - we have nothing stored
	relay.QueryEvents = append(relay.QueryEvents,
		func(ctx context.Context, filter nostr.Filter) (chan *nostr.Event, error) {
			// Return empty channel
			ch := make(chan *nostr.Event)
			close(ch)
			return ch, nil
		},
	)

	// Count events - always return 0
	relay.CountEvents = append(relay.CountEvents,
		func(ctx context.Context, filter nostr.Filter) (int64, error) {
			return 0, nil
		},
	)

	// Delete events - we don't store so nothing to delete
	relay.DeleteEvent = append(relay.DeleteEvent,
		func(ctx context.Context, event *nostr.Event) error {
			return nil
		},
	)
}

func (r *Relay) handleEvent(event *nostr.Event) {
	logging.Debug("Relay: Received event id=%s, kind=%d, author=%s", event.ID, event.Kind, event.PubKey[:16]+"...")

	// Extract relay URLs from the event (works for all event kinds)
	relays := r.discovery.ExtractRelaysFromEvent(event)

	if len(relays) > 0 {
		logging.Debug("Relay: Extracted %d relay URLs from event %s (kind %d)", len(relays), event.ID, event.Kind)
		for _, relayURL := range relays {
			r.discovery.AddRelayIfNew(relayURL)
		}
	}

	// Broadcast the event to top N relays
	r.broadcaster.Broadcast(event)
}

// Start starts the relay server
func (r *Relay) Start() error {
	mux := http.NewServeMux()

	// Serve static files (icons, banners)
	fileServer := http.FileServer(http.Dir("."))
	mux.Handle("/static/", fileServer)

	// Main page handler (HTTP) and WebSocket relay (WS)
	mux.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		// Check if this is a WebSocket upgrade request
		if req.Header.Get("Upgrade") == "websocket" {
			// Let khatru handle WebSocket connections
			r.khatru.ServeHTTP(w, req)
			return
		}

		// Serve HTML main page for HTTP requests
		r.serveMainPage(w, req)
	})

	// Add a stats endpoint
	mux.HandleFunc("/stats", func(w http.ResponseWriter, req *http.Request) {
		stats := r.broadcaster.GetStats()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		// Simple JSON formatting
		fmt.Fprintf(w, "{\n")
		fmt.Fprintf(w, "  \"total_relays\": %d,\n", stats["total_relays"])
		fmt.Fprintf(w, "  \"active_relays\": %d,\n", stats["active_relays"])
		fmt.Fprintf(w, "  \"mandatory_relays\": %d,\n", stats["mandatory_relays"])

		// Queue stats
		queue := stats["queue"].(map[string]interface{})
		fmt.Fprintf(w, "  \"queue\": {\n")
		fmt.Fprintf(w, "    \"worker_count\": %d,\n", queue["worker_count"])
		fmt.Fprintf(w, "    \"channel_size\": %d,\n", queue["channel_size"])
		fmt.Fprintf(w, "    \"channel_capacity\": %d,\n", queue["channel_capacity"])
		fmt.Fprintf(w, "    \"channel_utilization\": %.2f,\n", queue["channel_utilization"])
		fmt.Fprintf(w, "    \"overflow_size\": %d,\n", queue["overflow_size"])
		fmt.Fprintf(w, "    \"total_queued\": %d,\n", queue["total_queued"])
		fmt.Fprintf(w, "    \"peak_size\": %d,\n", queue["peak_size"])
		fmt.Fprintf(w, "    \"saturation_count\": %d,\n", queue["saturation_count"])
		fmt.Fprintf(w, "    \"is_saturated\": %v,\n", queue["is_saturated"])
		fmt.Fprintf(w, "    \"last_saturation\": \"%v\"\n", queue["last_saturation"])
		fmt.Fprintf(w, "  },\n")

		// Cache stats
		cache := stats["cache"].(map[string]interface{})
		fmt.Fprintf(w, "  \"cache\": {\n")
		fmt.Fprintf(w, "    \"size\": %d,\n", cache["size"])
		fmt.Fprintf(w, "    \"max_size\": %d,\n", cache["max_size"])
		fmt.Fprintf(w, "    \"utilization_pct\": %.2f,\n", cache["utilization_pct"])
		fmt.Fprintf(w, "    \"hits\": %d,\n", cache["hits"])
		fmt.Fprintf(w, "    \"misses\": %d,\n", cache["misses"])
		fmt.Fprintf(w, "    \"hit_rate_pct\": %.2f\n", cache["hit_rate_pct"])
		fmt.Fprintf(w, "  },\n")

		// Mandatory relays
		mandatoryRelays := stats["mandatory_relay_list"].([]map[string]interface{})
		fmt.Fprintf(w, "  \"mandatory_relay_list\": [\n")
		for i, relay := range mandatoryRelays {
			fmt.Fprintf(w, "    {\n")
			fmt.Fprintf(w, "      \"url\": \"%s\",\n", relay["url"])
			fmt.Fprintf(w, "      \"score\": %.2f,\n", relay["score"])
			fmt.Fprintf(w, "      \"success_rate\": %.4f,\n", relay["success_rate"])
			fmt.Fprintf(w, "      \"avg_response_ms\": %d,\n", relay["avg_response_ms"])
			fmt.Fprintf(w, "      \"total_attempts\": %d\n", relay["total_attempts"])
			if i < len(mandatoryRelays)-1 {
				fmt.Fprintf(w, "    },\n")
			} else {
				fmt.Fprintf(w, "    }\n")
			}
		}
		fmt.Fprintf(w, "  ],\n")

		fmt.Fprintf(w, "  \"top_relays\": [\n")

		topRelays := stats["top_relays"].([]map[string]interface{})
		for i, relay := range topRelays {
			fmt.Fprintf(w, "    {\n")
			fmt.Fprintf(w, "      \"url\": \"%s\",\n", relay["url"])
			fmt.Fprintf(w, "      \"score\": %.2f,\n", relay["score"])
			fmt.Fprintf(w, "      \"success_rate\": %.4f,\n", relay["success_rate"])
			fmt.Fprintf(w, "      \"avg_response_ms\": %d,\n", relay["avg_response_ms"])
			fmt.Fprintf(w, "      \"total_attempts\": %d\n", relay["total_attempts"])
			if i < len(topRelays)-1 {
				fmt.Fprintf(w, "    },\n")
			} else {
				fmt.Fprintf(w, "    }\n")
			}
		}

		fmt.Fprintf(w, "  ]\n")
		fmt.Fprintf(w, "}\n")
	})

	addr := fmt.Sprintf(":%s", r.port)
	logging.Info("Relay: Starting relay server on %s", addr)
	logging.Debug("Relay: WebSocket endpoint ready")
	logging.Debug("Relay: Stats endpoint ready")
	logging.Debug("Relay: Main page endpoint ready")

	return http.ListenAndServe(addr, mux)
}

// serveMainPage serves the HTML main page with relay information
func (r *Relay) serveMainPage(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)

	// Get relay pubkey for display
	relayPubkey := r.khatru.Info.PubKey
	relayNpub := ""
	if relayPubkey != "" {
		if npub, err := nip19.EncodePublicKey(relayPubkey); err == nil {
			relayNpub = npub
		}
	}

	// Get contact npub for display
	contactNpub := ""
	if r.config.ContactPubkey != "" {
		contactNpub = r.config.ContactPubkey
		// If it's a hex pubkey, convert to npub
		if len(r.config.ContactPubkey) == 64 {
			if npub, err := nip19.EncodePublicKey(r.config.ContactPubkey); err == nil {
				contactNpub = npub
			}
		}
	}

	// Select random banner from list
	randomBanner := ""
	if len(r.config.RelayBanners) > 0 {
		randomBanner = r.config.RelayBanners[rand.Intn(len(r.config.RelayBanners))]
	}

	// Prepare template data
	data := map[string]interface{}{
		"Name":        r.config.RelayName,
		"Description": r.config.RelayDescription,
		"URL":         r.config.RelayURL,
		"RelayPubkey": relayPubkey,
		"RelayNpub":   relayNpub,
		"ContactNpub": contactNpub,
		"Icon":        r.config.RelayIcon,
		"Banner":      randomBanner,
		"Version":     r.khatru.Info.Version,
		"Software":    r.khatru.Info.Software,
	}

	logging.DebugMethod("relay", "serveMainPage", "Rendering main page: URL=%s, RelayNpub=%s, ContactNpub=%s, Icon=%s, Banner=%s",
		r.config.RelayURL, relayNpub, contactNpub, r.config.RelayIcon, randomBanner)

	tmpl := template.Must(template.ParseFiles("templates/main.html"))
	tmpl.Execute(w, data)
}
