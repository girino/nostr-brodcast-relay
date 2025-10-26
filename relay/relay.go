package relay

import (
	"context"
	"fmt"
	"html/template"
	"math/rand"
	"net/http"
	"time"

	"github.com/fiatjaf/khatru"
	"github.com/girino/broadcast-relay/config"
	"github.com/girino/nostr-lib/broadcast"
	"github.com/girino/nostr-lib/broadcast/health"
	json "github.com/girino/nostr-lib/json"
	"github.com/girino/nostr-lib/logging"
	"github.com/girino/nostr-lib/stats"
	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip19"
)

type Relay struct {
	khatru          *khatru.Relay
	broadcastSystem *broadcast.BroadcastSystem
	healthChecker   *health.Checker
	config          *config.Config
	port            string
}

func NewRelay(cfg *config.Config, broadcastSystem *broadcast.BroadcastSystem, healthChecker *health.Checker) *Relay {
	r := &Relay{
		khatru:          khatru.NewRelay(),
		broadcastSystem: broadcastSystem,
		healthChecker:   healthChecker,
		config:          cfg,
		port:            cfg.RelayPort,
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
			if r.broadcastSystem.IsEventCached(event.ID) {
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
	relays := r.broadcastSystem.ExtractRelaysFromEvent(event)

	if len(relays) > 0 {
		logging.Debug("Relay: Extracted %d relay URLs from event %s (kind %d)", len(relays), event.ID, event.Kind)
		for _, relayURL := range relays {
			r.broadcastSystem.AddRelayIfNew(relayURL)
		}
	}

	// Broadcast the event to top N relays
	r.broadcastSystem.BroadcastEvent(event)
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

		// Check if this is a NIP-11 request (Accept: application/nostr+json)
		accept := req.Header.Get("Accept")
		if accept == "application/nostr+json" {
			// Let khatru handle NIP-11 relay information document
			r.khatru.ServeHTTP(w, req)
			return
		}

		// Serve HTML main page for regular HTTP requests
		r.serveMainPage(w, req)
	})

	// Add a stats endpoint
	mux.HandleFunc("/stats", func(w http.ResponseWriter, req *http.Request) {
		// Use the global stats collector
		allStats := stats.GetCollector().GetAllStats()

		// Add timestamp
		allStats.Set("timestamp", json.NewJsonValue(time.Now().Unix()))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		// Marshal to JSON
		jsonData, err := json.MarshalIndent(allStats, "", "  ")
		if err != nil {
			logging.Error("Failed to marshal stats to JSON: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		w.Write(jsonData)
	})

	// Add a health endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, req *http.Request) {
		// Get basic health information from global stats
		statsObj := stats.GetCollector().GetAllStats()

		// Add current timestamp
		statsObj.Set("timestamp", json.NewJsonValue(time.Now().Unix()))

		// Extract health information from stats
		managerObj, exists := statsObj.Get("manager")
		if !exists {
			logging.Error("Manager stats not found")
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		managerStats, ok := managerObj.(*json.JsonObject)
		if !ok {
			logging.Error("Manager stats is not a JsonObject")
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		// Get total relays
		totalRelaysVal, hasTotal := managerStats.Get("total_relays")
		var totalRelays int
		if hasTotal {
			if totalRelaysEntity, ok := totalRelaysVal.(*json.JsonValue); ok {
				if val, ok := totalRelaysEntity.GetInt(); ok {
					totalRelays = int(val)
				}
			}
		}

		// Get active relays count from top_relays list
		topRelaysVal, hasTop := managerStats.Get("top_relays")
		activeRelays := 0
		if hasTop {
			if topRelaysList, ok := topRelaysVal.(*json.JsonList); ok {
				activeRelays = topRelaysList.Length()
			}
		}

		// Determine health status
		status := "healthy"
		if totalRelays == 0 {
			status = "unhealthy"
		} else if activeRelays == 0 {
			status = "degraded"
		}

		healthResponse := json.NewJsonObject()
		healthResponse.Set("status", json.NewJsonValue(status))
		healthResponse.Set("total_relays", json.NewJsonValue(totalRelays))
		healthResponse.Set("active_relays", json.NewJsonValue(activeRelays))

		// Get timestamp from stats
		if timestampVal, hasTimestamp := statsObj.Get("timestamp"); hasTimestamp {
			if timestampEntity, ok := timestampVal.(*json.JsonValue); ok {
				healthResponse.Set("timestamp", timestampEntity)
			}
		}

		w.Header().Set("Content-Type", "application/json")

		// Set HTTP status code based on health
		switch status {
		case "healthy":
			w.WriteHeader(http.StatusOK)
		case "degraded":
			w.WriteHeader(http.StatusOK) // Still OK but with warning
		case "unhealthy":
			w.WriteHeader(http.StatusServiceUnavailable)
		}

		// Marshal to JSON
		jsonData, err := json.MarshalIndent(healthResponse, "", "  ")
		if err != nil {
			logging.Error("Failed to marshal health to JSON: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		w.Write(jsonData)
	})

	addr := fmt.Sprintf(":%s", r.port)
	logging.Info("Relay: Starting relay server on %s", addr)
	logging.Debug("Relay: WebSocket endpoint ready")
	logging.Debug("Relay: Stats endpoint ready")
	logging.Debug("Relay: Health endpoint ready")
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
