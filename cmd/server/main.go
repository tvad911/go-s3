package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"time"

	"gos3/internal/auth"
	"gos3/internal/config"
	"gos3/internal/server"
	"gos3/internal/storage/local"
	"gos3/internal/storage/metadata"
)

var version = "dev"

func main() {
	var configFile string
	flag.StringVar(&configFile, "config", "deploy/config.example.yaml", "path to config file")
	flag.Parse()

	// Initialize default logger temporarily before config is loaded
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, nil)))

	// Load configuration
	cfg, err := config.Load(configFile)
	if err != nil {
		slog.Error("failed to load configuration", "error", err)
		os.Exit(1)
	}

	// Reconfigure logger based on config
	setupLogger(cfg.Log)

	slog.Info("starting gos3 server", "version", version)
	slog.Info("replication config loaded", "targets", len(cfg.Replication.Targets))

	// Ensure data directory exists before initializing stores
	if err := os.MkdirAll(cfg.Storage.DataDir, 0755); err != nil {
		slog.Error("failed to create data directory", "error", err)
		os.Exit(1)
	}

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

	// Ensure root user exists
	rootUser := &auth.User{
		Username:    "root",
		AccessKeyID: cfg.Auth.RootAccessKey,
		SecretKey:   cfg.Auth.RootSecretKey,
		IsRoot:      true,
		CreatedAt:   time.Now().UTC(),
	}
	// Try to get root by access key; if it doesn't exist, create it.
	// In a real app we might update the secret key if it changed in config.
	if _, err := metaStore.GetUserByAccessKey(context.Background(), cfg.Auth.RootAccessKey); err != nil {
		_ = metaStore.CreateUser(context.Background(), rootUser)
	}

	sigv4Verifier := auth.NewSigV4Verifier(metaStore, cfg.Auth.Region)

	// Initialize and start server
	srv := server.New(cfg, backend, metaStore, sigv4Verifier)
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
