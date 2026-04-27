package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"clippy-backend/internal/api"
	"clippy-backend/internal/clipboard"
	"clippy-backend/internal/db"
)

func main() {
	port := flag.String("port", "5100", "API server port")
	dataDir := flag.String("data", "./data", "Data directory")
	staticDir := flag.String("static", "./ui-prototype", "Static files directory")
	imagesDir := flag.String("images", "./data/images", "Images directory")
	launchctl := flag.Bool("launchctl", false, "Run in launchctl mode (no stdin)")
	flag.Parse()

	// Suppress unused variable warning
	_ = *launchctl

	_ = os.MkdirAll(*dataDir, 0755)
	_ = os.MkdirAll(*imagesDir, 0755)

	absDataDir, _ := filepath.Abs(*dataDir)
	dbPath := filepath.Join(absDataDir, "clippy.db")

	store, err := db.New(dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}

	// Startup cleanup
	store.CleanupOrphanImages(*imagesDir)
	store.EnforceImageLimit(*imagesDir, 200*1024*1024) // 200MB limit

	monitor := clipboard.New(store, *imagesDir)
	go monitor.Start()

	server := api.New(store, *staticDir, *imagesDir)

	go func() {
		if err := server.ListenAndServe(":" + *port); err != nil {
			log.Printf("Server error: %v", err)
		}
	}()

	log.Printf("✅ Clippy backend started on port %s", *port)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	log.Println("👋 Shutting down...")
	store.Close()
}
