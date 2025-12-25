package main

import (
	"browser-agent/internal/browser"
	"browser-agent/internal/server"
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize browser controller
	browserCtrl, err := browser.NewController(ctx)
	if err != nil {
		log.Fatalf("Failed to initialize browser controller: %v", err)
	}
	defer browserCtrl.Close()

	// Initialize web server
	srv := server.NewServer(browserCtrl, ":8080")

	// Start server in goroutine
	go func() {
		log.Println("Starting server on http://localhost:8080")
		if err := srv.Start(); err != nil {
			log.Printf("Server error: %v", err)
		}
	}()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	fmt.Println("\nShutting down gracefully...")
	
	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}

	log.Println("Server stopped")
}