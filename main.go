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
	log.Println("==============================================================")
	log.Println("=== BROADCAST RELAY STARTING ===")
	log.Println("==============================================================")

	// Load configuration
	cfg := config.Load()
	log.Println("")
	log.Println("[CONFIG] Configuration loaded:")
	log.Printf("[CONFIG]   - Seed relays: %d", len(cfg.SeedRelays))
	for i, seed := range cfg.SeedRelays {
		log.Printf("[CONFIG]     %d. %s", i+1, seed)
	}
	log.Printf("[CONFIG]   - Top N relays: %d", cfg.TopNRelays)
	log.Printf("[CONFIG]   - Relay port: %s", cfg.RelayPort)
	log.Printf("[CONFIG]   - Refresh interval: %v", cfg.RefreshInterval)
	log.Printf("[CONFIG]   - Health check interval: %v", cfg.HealthCheckInterval)
	log.Printf("[CONFIG]   - Initial timeout: %v", cfg.InitialTimeout)
	log.Printf("[CONFIG]   - Success rate decay: %.2f", cfg.SuccessRateDecay)
	log.Println("")

	if len(cfg.SeedRelays) == 0 {
		log.Fatal("FATAL: No seed relays provided. Set SEED_RELAYS environment variable.")
	}

	// Initialize components
	log.Println("[MAIN] Initializing components...")
	mgr := manager.NewManager(cfg.TopNRelays, cfg.SuccessRateDecay)
	checker := health.NewChecker(mgr, cfg.InitialTimeout)
	disc := discovery.NewDiscovery(mgr, checker)
	bc := broadcaster.NewBroadcaster(mgr, checker)
	log.Println("")

	// Initial relay discovery and testing
	log.Println("[MAIN] ========== PHASE 1: DISCOVERY & TESTING ==========")
	ctx := context.Background()
	disc.DiscoverFromSeeds(ctx, cfg.SeedRelays)
	log.Println("")

	// Mark manager as initialized to switch to exponential decay
	mgr.MarkInitialized()
	log.Println("")

	// Log initial top relays
	log.Println("[MAIN] ========== PHASE 2: INITIAL RELAY SELECTION ==========")
	topRelays := mgr.GetTopRelays()
	log.Printf("[MAIN] Selected top %d relays from %d total relays", len(topRelays), mgr.GetRelayCount())
	log.Println("[MAIN] Top 10 relays:")
	for i, r := range topRelays {
		if i < 10 { // Show top 10
			log.Printf("[MAIN]   %d. %s", i+1, r.URL)
			log.Printf("[MAIN]      Success: %.2f%%, Avg time: %.2fms, Attempts: %d", 
				r.SuccessRate*100, float64(r.AvgResponseTime.Milliseconds()), r.TotalAttempts)
		}
	}
	log.Println("")

	// Start periodic refresh
	log.Println("[MAIN] Starting periodic refresh background task...")
	go startPeriodicRefresh(ctx, cfg, disc, mgr)

	// Start the relay server
	log.Println("")
	log.Println("[MAIN] ========== PHASE 3: STARTING RELAY SERVER ==========")
	relayServer := relay.NewRelay(cfg.RelayPort, bc, disc)

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Start relay in goroutine
	go func() {
		if err := relayServer.Start(); err != nil {
			log.Fatalf("[MAIN] Relay server error: %v", err)
		}
	}()

	log.Println("")
	log.Println("==============================================================")
	log.Println("=== BROADCAST RELAY IS NOW RUNNING ===")
	log.Printf("=== WebSocket: ws://localhost:%s ===", cfg.RelayPort)
	log.Printf("=== Stats: http://localhost:%s/stats ===", cfg.RelayPort)
	log.Println("=== Press Ctrl+C to stop ===")
	log.Println("==============================================================")
	log.Println("")

	// Wait for interrupt signal
	<-sigChan
	log.Println("")
	log.Println("==============================================================")
	log.Println("=== SHUTTING DOWN GRACEFULLY ===")
	log.Println("==============================================================")
	
	// Print final stats
	stats := bc.GetStats()
	log.Printf("[MAIN] Final stats:")
	log.Printf("[MAIN]   - Total relays: %v", stats["total_relays"])
	log.Printf("[MAIN]   - Active relays: %v", stats["active_relays"])
	log.Println("")
	log.Println("[MAIN] Goodbye!")
}

func startPeriodicRefresh(ctx context.Context, cfg *config.Config, disc *discovery.Discovery, mgr *manager.Manager) {
	ticker := time.NewTicker(cfg.RefreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			log.Println("")
			log.Println("==============================================================")
			log.Println("[REFRESH] === STARTING PERIODIC RELAY REFRESH ===")
			log.Println("==============================================================")
			
			disc.DiscoverFromSeeds(ctx, cfg.SeedRelays)
			
			topRelays := mgr.GetTopRelays()
			log.Printf("[REFRESH] Refresh complete: %d top relays from %d total relays", len(topRelays), mgr.GetRelayCount())
			log.Println("==============================================================")
			log.Println("")
			
		case <-ctx.Done():
			log.Println("[REFRESH] Periodic refresh stopped")
			return
		}
	}
}
