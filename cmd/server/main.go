package main

import (
	"log/slog"
	"os"

	"gos3/internal/config"
	"gos3/internal/server"
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

	// Initialize and start server
	srv := server.New(cfg)
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
