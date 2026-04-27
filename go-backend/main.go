package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"clippy-backend/internal/api"
	"clippy-backend/internal/clipboard"
	"clippy-backend/internal/db"
)

func main() {
	// Command line flags
	addr := flag.String("addr", ":5100", "HTTP server address")
	dbPath := flag.String("db", "clippy.db", "SQLite database path")
	staticDir := flag.String("static", "./ui-prototype", "Static files directory")
	flag.Parse()

	// Setup logging
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("Starting Clippy v2 Backend...")

	// Initialize database
	store, err := db.NewStore(*dbPath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer store.Close()

	// Initialize API server
	server := api.NewServer(store, *addr, *staticDir)

	// Initialize clipboard monitor
	monitor := clipboard.NewMonitor(func(content string) {
		item, err := store.AddItem(content)
		if err != nil {
			log.Printf("Failed to add clipboard item: %v", err)
			return
		}
		if item != nil {
			log.Printf("New clipboard item #%d: %.50s...", item.ID, item.Content)
		}
	})

	// Setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigCh
		log.Printf("Received signal: %v", sig)
		cancel()
		monitor.Stop()
		server.Shutdown()
	}()

	// Start services
	monitor.Start()

	// Start HTTP server
	go func() {
		if err := server.Start(); err != nil && err.Error() != "http: Server closed" {
			log.Printf("Server error: %v", err)
			cancel()
		}
	}()

	log.Printf("Clippy v2 Backend running on %s", *addr)
	log.Println("Press Ctrl+C to stop")

	// Wait for shutdown
	<-ctx.Done()
	log.Println("Shutting down...")
}
