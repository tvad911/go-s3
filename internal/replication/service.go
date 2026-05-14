package replication

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"time"

	"gos3/client"
	"gos3/internal/config"
	"gos3/internal/storage"
)

type EventType int

const (
	EventPut EventType = iota
	EventDelete
)

type Event struct {
	Type   EventType
	Bucket string
	Key    string
	Size   int64
	Meta   storage.ObjectMeta
}

type Service struct {
	cfg     *config.Config
	backend storage.Backend
	queue   chan Event
	clients []*client.Client
}

// NewService initializes a new replication service if enabled.
func NewService(cfg *config.Config, backend storage.Backend) (*Service, error) {
	if !cfg.Replication.Enabled || len(cfg.Replication.Targets) == 0 {
		return nil, nil // Disabled
	}

	var clients []*client.Client
	for _, t := range cfg.Replication.Targets {
		c, err := client.New(client.Config{
			Endpoint:        t.Endpoint,
			AccessKeyID:     t.AccessKey,
			SecretAccessKey: t.SecretKey,
			Region:          t.Region,
			PathStyle:       true,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to init replication target %s: %w", t.Endpoint, err)
		}
		clients = append(clients, c)
	}

	return &Service{
		cfg:     cfg,
		backend: backend,
		queue:   make(chan Event, cfg.Replication.MaxQueueSize),
		clients: clients,
	}, nil
}

// Enqueue adds an event to the replication queue asynchronously.
func (s *Service) Enqueue(ev Event) {
	if s == nil {
		return
	}
	select {
	case s.queue <- ev:
	default:
		slog.Warn("replication queue full, dropping event", "bucket", ev.Bucket, "key", ev.Key)
	}
}

// Start launches the background worker goroutines.
func (s *Service) Start(ctx context.Context) {
	if s == nil {
		return
	}

	for i := 0; i < s.cfg.Replication.Workers; i++ {
		go s.worker(ctx, i)
	}
}

func (s *Service) worker(ctx context.Context, id int) {
	slog.Info("starting replication worker", "id", id)
	for {
		select {
		case <-ctx.Done():
			return
		case ev := <-s.queue:
			s.processEvent(ctx, ev)
		}
	}
}

func (s *Service) processEvent(ctx context.Context, ev Event) {
	for _, c := range s.clients {
		err := s.replicateWithRetry(ctx, c, ev)
		if err != nil {
			slog.Error("replication failed after retries", "bucket", ev.Bucket, "key", ev.Key, "error", err)
		}
	}
}

func (s *Service) replicateWithRetry(ctx context.Context, c *client.Client, ev Event) error {
	maxRetries := 3
	backoff := time.Second

	var err error
	for i := 0; i < maxRetries; i++ {
		if err = s.replicateToTarget(ctx, c, ev); err == nil {
			return nil
		}
		slog.Warn("replication attempt failed", "bucket", ev.Bucket, "key", ev.Key, "attempt", i+1, "error", err)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
			backoff *= 2
		}
	}
	return err
}

func (s *Service) replicateToTarget(ctx context.Context, c *client.Client, ev Event) error {
	// First, try to make bucket (it will fail if it exists, but that's fine)
	err := c.MakeBucket(ctx, ev.Bucket)
	if err != nil {
		var clientErr client.ErrorResponse
		if errors.As(err, &clientErr) {
			if clientErr.Code != "BucketAlreadyExists" && clientErr.Code != "BucketAlreadyOwnedByYou" {
				return fmt.Errorf("MakeBucket failed: %w", err)
			}
		}
	}

	switch ev.Type {
	case EventPut:
		pr, pw := io.Pipe()

		// Read from local backend in a goroutine
		go func() {
			obj, err := s.backend.GetObject(ctx, ev.Bucket, ev.Key, storage.GetOptions{})
			if err != nil {
				pw.CloseWithError(err)
				return
			}
			defer obj.Content.Close()
			_, err = io.Copy(pw, obj.Content)
			pw.CloseWithError(err)
		}()

		opts := client.PutObjectOptions{
			ContentType: ev.Meta.ContentType,
			UserMeta:    ev.Meta.UserMeta,
		}

		err = c.PutObject(ctx, ev.Bucket, ev.Key, pr, ev.Size, opts)
		if err != nil {
			return fmt.Errorf("PutObject: %w", err)
		}

	case EventDelete:
		err := c.RemoveObject(ctx, ev.Bucket, ev.Key)
		if err != nil {
			// Ignore NotFound errors during deletion
			var clientErr client.ErrorResponse
			if errors.As(err, &clientErr) && clientErr.Code == "NoSuchKey" {
				return nil
			}
			return fmt.Errorf("RemoveObject: %w", err)
		}
	}
	return nil
}
