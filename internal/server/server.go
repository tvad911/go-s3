package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gos3/internal/config"
)

// Server represents the GoS3 HTTP server.
type Server struct {
	httpServer *http.Server
	config     *config.Config
}

// New creates a new GoS3 Server instance.
func New(cfg *config.Config) *Server {
	router := SetupRouter(cfg)

	srv := &http.Server{
		Addr:              fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler:           router,
		ReadTimeout:       cfg.Server.ReadTimeout,
		WriteTimeout:      cfg.Server.WriteTimeout, // 0 = unlimited for streaming
		IdleTimeout:       cfg.Server.IdleTimeout,
		ReadHeaderTimeout: cfg.Server.ReadHeaderTimeout, // protection against slowloris
		MaxHeaderBytes:    cfg.Server.MaxHeaderBytes,    // 1MB default
	}

	return &Server{
		httpServer: srv,
		config:     cfg,
	}
}

// Start runs the server and blocks until graceful shutdown is complete or an error occurs.
func (s *Server) Start() error {
	serverErrCh := make(chan error, 1)

	go func() {
		if s.config.Server.TLS.Enabled {
			slog.Info("starting HTTPS server", "addr", s.httpServer.Addr)
			if err := s.httpServer.ListenAndServeTLS(s.config.Server.TLS.Cert, s.config.Server.TLS.Key); err != nil && !errors.Is(err, http.ErrServerClosed) {
				serverErrCh <- fmt.Errorf("listen and serve tls error: %w", err)
			}
		} else {
			slog.Info("starting HTTP server", "addr", s.httpServer.Addr)
			if err := s.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				serverErrCh <- fmt.Errorf("listen and serve error: %w", err)
			}
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-serverErrCh:
		return err
	case sig := <-quit:
		slog.Info("shutting down server", "signal", sig)
	}

	// Give active connections 30 seconds to finish
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := s.httpServer.Shutdown(ctx); err != nil {
		return fmt.Errorf("server forced to shutdown: %w", err)
	}

	slog.Info("server exited properly")
	return nil
}
