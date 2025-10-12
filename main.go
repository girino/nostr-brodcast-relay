package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/girino/broadcast-relay/broadcaster"
	"github.com/girino/broadcast-relay/config"
	"github.com/girino/broadcast-relay/discovery"
	"github.com/girino/broadcast-relay/health"
	"github.com/girino/broadcast-relay/manager"
	"github.com/girino/broadcast-relay/relay"
)

func main() {
	log.Println("=== Broadcast Relay Starting ===")

	// Load configuration
	cfg := config.Load()
	log.Printf("Configuration loaded:")
	log.Printf("  - Seed relays: %d", len(cfg.SeedRelays))
	log.Printf("  - Top N relays: %d", cfg.TopNRelays)
	log.Printf("  - Relay port: %s", cfg.RelayPort)
	log.Printf("  - Refresh interval: %v", cfg.RefreshInterval)
	log.Printf("  - Health check interval: %v", cfg.HealthCheckInterval)
	log.Printf("  - Initial timeout: %v", cfg.InitialTimeout)
	log.Printf("  - Success rate decay: %.2f", cfg.SuccessRateDecay)

	if len(cfg.SeedRelays) == 0 {
		log.Fatal("FATAL: No seed relays provided. Set SEED_RELAYS environment variable.")
	}

	// Initialize components
	mgr := manager.NewManager(cfg.TopNRelays, cfg.SuccessRateDecay)
	checker := health.NewChecker(mgr, cfg.InitialTimeout)
	disc := discovery.NewDiscovery(mgr, checker)
	bc := broadcaster.NewBroadcaster(mgr, checker)

	// Initial relay discovery and testing
	log.Println("Starting initial relay discovery...")
	ctx := context.Background()
	disc.DiscoverFromSeeds(ctx, cfg.SeedRelays)

	// Mark manager as initialized to switch to exponential decay
	mgr.MarkInitialized()

	// Log initial top relays
	topRelays := mgr.GetTopRelays()
	log.Printf("Initial top %d relays selected from %d total relays", len(topRelays), mgr.GetRelayCount())
	for i, r := range topRelays {
		if i < 10 { // Show top 10
			log.Printf("  %d. %s (success: %.2f%%, avg: %.2fms)", 
				i+1, r.URL, r.SuccessRate*100, float64(r.AvgResponseTime.Milliseconds()))
		}
	}

	// Start periodic refresh
	go startPeriodicRefresh(ctx, cfg, disc, mgr)

	// Start the relay server
	relayServer := relay.NewRelay(cfg.RelayPort, bc, disc)

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Start relay in goroutine
	go func() {
		if err := relayServer.Start(); err != nil {
			log.Fatalf("Relay server error: %v", err)
		}
	}()

	// Wait for interrupt signal
	<-sigChan
	log.Println("\n=== Shutting down gracefully ===")
	
	// Print final stats
	stats := bc.GetStats()
	log.Printf("Final stats:")
	log.Printf("  - Total relays: %v", stats["total_relays"])
	log.Printf("  - Active relays: %v", stats["active_relays"])
}

func startPeriodicRefresh(ctx context.Context, cfg *config.Config, disc *discovery.Discovery, mgr *manager.Manager) {
	ticker := time.NewTicker(cfg.RefreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			log.Println("=== Starting periodic relay refresh ===")
			disc.DiscoverFromSeeds(ctx, cfg.SeedRelays)
			
			topRelays := mgr.GetTopRelays()
			log.Printf("Refreshed: %d top relays from %d total relays", len(topRelays), mgr.GetRelayCount())
			
		case <-ctx.Done():
			return
		}
	}
}

