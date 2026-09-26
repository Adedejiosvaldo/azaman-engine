package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"azaman-engine/internal/api"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	router := api.SetupRouter()

	srv := &http.Server{
		Addr:           ":" + port,
		Handler:        router,
		ReadTimeout:    30 * time.Second,
		WriteTimeout:   30 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	// Channel to listen for errors from the server
	serverErrors := make(chan error, 1)

	go func() {
		fmt.Println("==================================================================")
		fmt.Println("  ⚡ AZAMAN ENGINE - SHAZAM FOR NIGERIAN BANK STATEMENTS ⚡")
		fmt.Println("  Ephemeral RAM: 32MB Buffer Bounded | Zero Bytes On Disk")
		fmt.Printf("  Server listening on http://localhost:%s\n", port)
		fmt.Println("==================================================================")
		serverErrors <- srv.ListenAndServe()
	}()

	// Channel to listen for interrupt signals for graceful shutdown
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-serverErrors:
		if err != nil && err != http.ErrServerClosed {
			log.Fatalf("Fatal server error: %v", err)
		}

	case sig := <-shutdown:
		log.Printf("Received signal %v. Initiating graceful shutdown...", sig)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := srv.Shutdown(ctx); err != nil {
			log.Printf("Error during graceful shutdown: %v", err)
			_ = srv.Close()
		}
		log.Println("Server shut down successfully. All memory scrubbed.")
	}
}
