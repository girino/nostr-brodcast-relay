package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/girino/broadcast-relay/broadcaster"
	"github.com/girino/broadcast-relay/config"
	"github.com/girino/broadcast-relay/discovery"
	"github.com/girino/broadcast-relay/health"
	"github.com/girino/broadcast-relay/logging"
	"github.com/girino/broadcast-relay/manager"
	"github.com/girino/broadcast-relay/relay"
)

func main() {
	// Parse command-line flags
	var verbose string
	flag.StringVar(&verbose, "verbose", "", "Enable verbose logging. Examples: 'true' (all), 'config,health' (modules), 'broadcaster.addEventToCache' (specific method)")
	flag.StringVar(&verbose, "v", "", "Enable verbose logging (shorthand)")
	flag.Parse()

	// Set verbose mode in logging package
	logging.SetVerbose(verbose)

	logging.Info("==============================================================")
	logging.Info("=== BROADCAST RELAY STARTING ===")
	logging.Info("==============================================================")

	// Load configuration
	cfg := config.Load()
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
	logging.Debug("  - Refresh interval: %v", cfg.RefreshInterval)
	logging.Debug("  - Health check interval: %v", cfg.HealthCheckInterval)
	logging.Debug("  - Initial timeout: %v", cfg.InitialTimeout)
	logging.Debug("  - Success rate decay: %.2f", cfg.SuccessRateDecay)
	logging.Info("")

	// Initialize components
	logging.Info("Initializing components...")
	mgr := manager.NewManager(cfg.TopNRelays, cfg.SuccessRateDecay)
	checker := health.NewChecker(mgr, cfg.InitialTimeout)
	disc := discovery.NewDiscovery(mgr, checker)
	bc := broadcaster.NewBroadcaster(mgr, checker, cfg.MandatoryRelays, cfg.WorkerCount)
	logging.Info("")

	// Add mandatory relays to the manager for tracking
	if len(cfg.MandatoryRelays) > 0 {
		logging.Info("Adding mandatory relays to manager for tracking...")
		for _, url := range cfg.MandatoryRelays {
			mgr.AddMandatoryRelay(url)
		}
	}

	// Initial relay discovery and testing
	logging.Info("========== PHASE 1: DISCOVERY & TESTING ==========")
	ctx := context.Background()
	disc.DiscoverFromSeeds(ctx, cfg.SeedRelays)
	logging.Info("")

	// Mark manager as initialized to switch to exponential decay
	mgr.MarkInitialized()
	logging.Info("")

	// Log initial top relays
	logging.Info("========== PHASE 2: INITIAL RELAY SELECTION ==========")
	topRelays := mgr.GetTopRelays()
	logging.Info("Selected top %d relays from %d total relays", len(topRelays), mgr.GetRelayCount())
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
	go startPeriodicRefresh(ctx, cfg, disc, mgr)

	// Start the broadcaster worker pool
	logging.Info("Starting broadcaster worker pool...")
	bc.Start()

	// Start the relay server
	logging.Info("")
	logging.Info("========== PHASE 3: STARTING RELAY SERVER ==========")
	relayServer := relay.NewRelay(cfg.RelayPort, bc, disc)

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

	// Stop the broadcaster worker pool
	bc.Stop()

	// Print final stats
	stats := bc.GetStats()
	logging.Info("Final stats:")
	logging.Info("  - Total relays: %v", stats["total_relays"])
	logging.Info("  - Active relays: %v", stats["active_relays"])
	logging.Info("")
	logging.Info("Goodbye!")
}

func startPeriodicRefresh(ctx context.Context, cfg *config.Config, disc *discovery.Discovery, mgr *manager.Manager) {
	ticker := time.NewTicker(cfg.RefreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			logging.Debug("")
			logging.Debug("==============================================================")
			logging.Info("Starting periodic relay refresh...")
			logging.Debug("==============================================================")

			disc.DiscoverFromSeeds(ctx, cfg.SeedRelays)

			topRelays := mgr.GetTopRelays()
			logging.Info("Refresh complete: %d top relays from %d total relays", len(topRelays), mgr.GetRelayCount())
			logging.Debug("==============================================================")
			logging.Debug("")

		case <-ctx.Done():
			logging.Debug("Periodic refresh stopped")
			return
		}
	}
}
