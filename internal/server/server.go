package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"gos3/internal/auth"
	"gos3/internal/config"
	"gos3/internal/storage"
	"gos3/internal/storage/metadata"
)

// Server represents the GoS3 HTTP server.
type Server struct {
	httpServer  *http.Server
	redirectSrv *http.Server
	config      *config.Config
	backend     storage.Backend
	verifier    *auth.SigV4Verifier
}

// New creates a new GoS3 Server instance.
func New(cfg *config.Config, backend storage.Backend, metaStore metadata.Store, verifier *auth.SigV4Verifier) *Server {
	srv := &http.Server{
		Addr:              fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler:           SetupRouter(cfg, backend, metaStore, verifier),
		ReadTimeout:       cfg.Server.ReadTimeout,
		WriteTimeout:      cfg.Server.WriteTimeout, // 0 = unlimited for streaming
		IdleTimeout:       cfg.Server.IdleTimeout,
		ReadHeaderTimeout: cfg.Server.ReadHeaderTimeout, // protection against slowloris
		MaxHeaderBytes:    cfg.Server.MaxHeaderBytes,    // 1MB default
	}

	return &Server{
		httpServer: srv,
		config:     cfg,
		backend:    backend,
		verifier:   verifier,
	}
}

// Start runs the server and blocks until graceful shutdown is complete or an error occurs.
func (s *Server) Start() error {
	serverErrCh := make(chan error, 1)

	go func() {
		if s.config.Server.TLS.Enabled {
			slog.Info("starting HTTPS server", "addr", s.httpServer.Addr)
			
			if s.config.Server.TLS.AutoRedirect {
				httpAddr := fmt.Sprintf("%s:%d", s.config.Server.Host, s.config.Server.TLS.HTTPPort)
				slog.Info("starting HTTP redirect server", "addr", httpAddr)
				
				redirectMux := http.NewServeMux()
				redirectMux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
					host := r.Host
					if strings.Contains(host, ":") {
						host = strings.Split(host, ":")[0]
					}
					target := fmt.Sprintf("https://%s:%d%s", host, s.config.Server.Port, r.URL.RequestURI())
					http.Redirect(w, r, target, http.StatusMovedPermanently)
				})
				
				s.redirectSrv = &http.Server{Addr: httpAddr, Handler: redirectMux}
				go func() {
					if err := s.redirectSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
						serverErrCh <- fmt.Errorf("redirect server error: %w", err)
					}
				}()
			}

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

	workerCtx, workerCancel := context.WithCancel(context.Background())
	defer workerCancel()
	go s.startLifecycleWorker(workerCtx)

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

	if s.redirectSrv != nil {
		if err := s.redirectSrv.Shutdown(ctx); err != nil {
			slog.Error("redirect server forced to shutdown", "error", err)
		}
	}

	slog.Info("server exited properly")
	return nil
}
