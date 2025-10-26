package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/girino/broadcast-relay/config"
	"github.com/girino/broadcast-relay/relay"
	"github.com/girino/nostr-lib/broadcast"
	json "github.com/girino/nostr-lib/json"
	"github.com/girino/nostr-lib/logging"
)

func main() {
	// Parse command-line flags
	var verbose string
	flag.StringVar(&verbose, "verbose", "", "Enable verbose logging. Examples: 'all' or 'true' (everything), 'config,health' (modules), 'broadcaster.addEventToCache' (specific method)")
	flag.StringVar(&verbose, "v", "", "Enable verbose logging (shorthand)")
	flag.Parse()

	// Load configuration
	cfg := config.Load()

	// Use config's verbose setting if command line verbose is empty
	if verbose == "" {
		verbose = cfg.Verbose
	}

	// Set verbose mode in logging package
	logging.SetVerbose(verbose)

	logging.Info("==============================================================")
	logging.Info("=== BROADCAST RELAY STARTING ===")
	logging.Info("==============================================================")
	logging.Info("")
	logging.Info("Configuration loaded:")
	logging.Info("  - Seed relays: %d", len(cfg.SeedRelays))
	for i, seed := range cfg.SeedRelays {
		logging.Debug("    %d. %s", i+1, seed)
	}
	logging.Info("  - Mandatory relays: %d", len(cfg.MandatoryRelays))
	for i, relay := range cfg.MandatoryRelays {
		logging.Debug("    %d. %s", i+1, relay)
	}
	logging.Info("  - Top N relays: %d", cfg.TopNRelays)
	logging.Info("  - Relay port: %s", cfg.RelayPort)
	logging.Info("  - Worker count: %d", cfg.WorkerCount)
	logging.Info("  - Cache TTL: %v", cfg.CacheTTL)
	logging.Debug("  - Refresh interval: %v", cfg.RefreshInterval)
	logging.Debug("  - Health check interval: %v", cfg.HealthCheckInterval)
	logging.Debug("  - Initial timeout: %v", cfg.InitialTimeout)
	logging.Debug("  - Success rate decay: %.2f", cfg.SuccessRateDecay)
	logging.Info("")

	// Initialize components
	logging.Info("Initializing components...")

	// Create broadcast system configuration
	broadcastConfig := &broadcast.Config{
		TopNRelays:       cfg.TopNRelays,
		SuccessRateDecay: cfg.SuccessRateDecay,
		MandatoryRelays:  cfg.MandatoryRelays,
		WorkerCount:      cfg.WorkerCount,
		CacheTTL:         cfg.CacheTTL,
		InitialTimeout:   cfg.InitialTimeout,
	}

	// Create unified broadcast system
	broadcastSystem := broadcast.NewBroadcastSystem(broadcastConfig)

	// Get health checker from broadcast system
	checker := broadcastSystem.GetHealthChecker()
	logging.Info("")

	// Add mandatory relays to the manager for tracking
	if len(cfg.MandatoryRelays) > 0 {
		logging.Info("Adding mandatory relays to manager for tracking...")
		broadcastSystem.AddMandatoryRelays(cfg.MandatoryRelays)
	}

	// Initial relay discovery and testing
	logging.Info("========== PHASE 1: DISCOVERY & TESTING ==========")
	ctx := context.Background()
	broadcastSystem.DiscoverFromSeeds(ctx, cfg.SeedRelays)
	logging.Info("")

	// Mark manager as initialized to switch to exponential decay
	broadcastSystem.MarkInitialized()
	logging.Info("")

	// Log initial top relays
	logging.Info("========== PHASE 2: INITIAL RELAY SELECTION ==========")
	topRelays := broadcastSystem.GetTopRelays()
	logging.Info("Selected top %d relays from %d total relays", len(topRelays), broadcastSystem.GetRelayCount())
	logging.Debug("Top 10 relays:")
	for i, r := range topRelays {
		if i < 10 { // Show top 10
			logging.Debug("  %d. %s", i+1, r.URL)
			logging.Debug("     Success: %.2f%%, Avg time: %.2fms, Attempts: %d",
				r.SuccessRate*100, float64(r.AvgResponseTime.Milliseconds()), r.TotalAttempts)
		}
	}
	logging.Info("")

	// Start periodic refresh
	logging.Info("Starting periodic refresh background task...")
	go startPeriodicRefresh(ctx, cfg, broadcastSystem)

	// Start the broadcast system
	logging.Info("Starting broadcast system...")
	broadcastSystem.Start()

	// Start the relay server
	logging.Info("")
	logging.Info("========== PHASE 3: STARTING RELAY SERVER ==========")
	relayServer := relay.NewRelay(cfg, broadcastSystem, checker)

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Start relay in goroutine
	go func() {
		if err := relayServer.Start(); err != nil {
			logging.Error("Relay server error: %v", err)
			os.Exit(1)
		}
	}()

	logging.Info("")
	logging.Info("==============================================================")
	logging.Info("=== BROADCAST RELAY IS NOW RUNNING ===")
	logging.Info("=== WebSocket: ws://localhost:%s ===", cfg.RelayPort)
	logging.Info("=== Stats: http://localhost:%s/stats ===", cfg.RelayPort)
	logging.Info("=== Press Ctrl+C to stop ===")
	logging.Info("==============================================================")
	logging.Info("")

	// Wait for interrupt signal
	<-sigChan
	logging.Info("")
	logging.Info("==============================================================")
	logging.Info("=== SHUTTING DOWN GRACEFULLY ===")
	logging.Info("==============================================================")

	// Stop the broadcast system
	broadcastSystem.Stop()

	// Print final stats
	finalStats := broadcastSystem.GetStats()
	logging.Info("Final stats:")

	// Extract manager stats
	if statsObj, ok := finalStats.(*json.JsonObject); ok {
		managerObj, hasManager := statsObj.Get("manager")
		if hasManager {
			if managerStats, ok := managerObj.(*json.JsonObject); ok {
				totalRelaysVal, hasTotal := managerStats.Get("total_relays")
				if hasTotal {
					if totalRelaysEntity, ok := totalRelaysVal.(*json.JsonValue); ok {
						if totalRelays, ok := totalRelaysEntity.GetInt(); ok {
							logging.Info("  - Total relays: %d", totalRelays)
						}
					}
				}

				topRelaysVal, hasTop := managerStats.Get("top_relays")
				if hasTop {
					if topRelaysList, ok := topRelaysVal.(*json.JsonList); ok {
						logging.Info("  - Active relays: %d", topRelaysList.Length())
					}
				}
			}
		}
	}
	logging.Info("")
	logging.Info("Goodbye!")
}

func startPeriodicRefresh(ctx context.Context, cfg *config.Config, broadcastSystem *broadcast.BroadcastSystem) {
	ticker := time.NewTicker(cfg.RefreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			logging.Debug("")
			logging.Debug("==============================================================")
			logging.Info("Starting periodic relay refresh...")
			logging.Debug("==============================================================")

			broadcastSystem.DiscoverFromSeeds(ctx, cfg.SeedRelays)

			topRelays := broadcastSystem.GetTopRelays()
			logging.Info("Refresh complete: %d top relays from %d total relays", len(topRelays), broadcastSystem.GetRelayCount())
			logging.Debug("==============================================================")
			logging.Debug("")

		case <-ctx.Done():
			logging.Debug("Periodic refresh stopped")
			return
		}
	}
}
