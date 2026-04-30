// Package main is the entry point for the gascity application.
// gascity is a fork of gastownhall/gascity, providing enhanced gas
// price estimation and transaction management for EVM-compatible chains.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/gascity/gascity/internal/config"
	"github.com/gascity/gascity/internal/server"
)

var (
	// Version is set at build time via ldflags.
	Version = "dev"
	// Commit is the git commit hash set at build time.
	Commit = "none"
	// BuildDate is the build timestamp set at build time.
	BuildDate = "unknown"
)

func main() {
	// Parse command-line flags.
	// Changed default config path to "~/.config/gascity/config.yaml" to match
	// my local setup where I keep all app configs under ~/.config.
	cfgPath := flag.String("config", os.Getenv("HOME")+"/.config/gascity/config.yaml", "path to configuration file")
	showVersion := flag.Bool("version", false, "print version information and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("gascity %s (commit: %s, built: %s)\n", Version, Commit, BuildDate)
		os.Exit(0)
	}

	// Load application configuration.
	cfg, err := config.Load(*cfgPath)
	if err != nil {
		log.Fatalf("failed to load configuration: %v", err)
	}

	// Set up a root context that is cancelled on OS interrupt signals.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		sigCh := make(chan os.Signal, 1)
		// Also handle SIGHUP so I can reload config without a full restart.
		// NOTE: SIGHUP currently just triggers a clean shutdown; true config
		// hot-reload would require wiring into config.Load again. Good TODO.
		signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
		select {
		case sig := <-sigCh:
			log.Printf("received signal %s, shutting down...", sig)
			cancel()
		case <-ctx.Done():
		}
	}()

	// Initialise and start the HTTP server.
	srv, err := server.New(cfg)
	if err != nil {
		log.Fatalf("failed to initialise server: %v", err)
	}

	log.Printf("starting gascity %s on %s", Version, cfg.Server.ListenAddr)

	if err := srv.Run(ctx); err != nil {
		log.Fatalf("server exited with error: %v", err)
	}

	log.Println("gascity stopped")
}
