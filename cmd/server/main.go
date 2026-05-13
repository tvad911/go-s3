package main

import (
	"log/slog"
	"os"

	"gos3/internal/config"
	"gos3/internal/server"
	"gos3/internal/storage/local"
	"gos3/internal/storage/metadata"
)

var version = "dev"

func main() {
	// Initialize default logger temporarily before config is loaded
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, nil)))

	// Load configuration
	cfg, err := config.Load("deploy/config.example.yaml") // Will be overridden by CLI args later
	if err != nil {
		slog.Error("failed to load configuration", "error", err)
		os.Exit(1)
	}

	// Reconfigure logger based on config
	setupLogger(cfg.Log)

	slog.Info("starting gos3 server", "version", version)

	// Initialize metadata store
	metaStore, err := metadata.NewBboltStore(cfg.Storage.DataDir + "/meta.db")
	if err != nil {
		slog.Error("failed to init metadata store", "error", err)
		os.Exit(1)
	}
	defer metaStore.Close()

	// Initialize storage backend
	backend, err := local.NewBackend(cfg.Storage.DataDir, cfg.Storage.DataDir+"/tmp", metaStore)
	if err != nil {
		slog.Error("failed to init storage backend", "error", err)
		os.Exit(1)
	}

	// Initialize and start server
	srv := server.New(cfg, backend)
	if err := srv.Start(); err != nil {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}
}

func setupLogger(cfg config.LogConfig) {
	var level slog.Level
	switch cfg.Level {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	var handler slog.Handler
	opts := &slog.HandlerOptions{Level: level}

	if cfg.Format == "json" {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	slog.SetDefault(slog.New(handler))
}
