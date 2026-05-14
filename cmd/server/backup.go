package main

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gos3/internal/config"
	"gos3/internal/storage/metadata"
)

func runExport(cfg *config.Config, outputFile string) {
	slog.Info("starting data export", "output", outputFile)

	// Ensure data directory exists
	dataDir := cfg.Storage.DataDir
	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		slog.Error("data directory does not exist", "dir", dataDir)
		os.Exit(1)
	}

	out, err := os.Create(outputFile)
	if err != nil {
		slog.Error("failed to create output file", "error", err)
		os.Exit(1)
	}
	defer out.Close()

	gw := gzip.NewWriter(out)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	// 1. Snapshot the database first
	metaStore, err := metadata.NewBboltStore(filepath.Join(dataDir, "meta.db"))
	if err != nil {
		slog.Error("failed to open metadata store for backup", "error", err)
		os.Exit(1)
	}

	// Create a temp file to hold the DB snapshot
	tmpDB, err := os.CreateTemp(cfg.Storage.TempDir, "meta-backup-*.db")
	if err != nil {
		metaStore.Close()
		slog.Error("failed to create temp db file", "error", err)
		os.Exit(1)
	}
	tmpDBPath := tmpDB.Name()
	defer os.Remove(tmpDBPath)

	if err := metaStore.BackupTo(tmpDB); err != nil {
		metaStore.Close()
		tmpDB.Close()
		slog.Error("failed to backup metadata store", "error", err)
		os.Exit(1)
	}
	metaStore.Close()

	// 2. Add the snapshot to tar as meta.db
	fi, err := tmpDB.Stat()
	if err == nil {
		hdr := &tar.Header{
			Name:    "meta.db",
			Mode:    0600,
			Size:    fi.Size(),
			ModTime: time.Now(),
		}
		if err := tw.WriteHeader(hdr); err == nil {
			tmpDB.Seek(0, 0)
			io.Copy(tw, tmpDB)
		}
	}
	tmpDB.Close()

	// 3. Walk through the data directory and add all files except meta.db and tmp folder
	err = filepath.Walk(dataDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(dataDir, path)
		if err != nil {
			return err
		}

		if relPath == "." || relPath == "meta.db" || strings.HasPrefix(relPath, "tmp") || relPath == "tmp" {
			return nil
		}

		hdr, err := tar.FileInfoHeader(info, info.Name())
		if err != nil {
			return err
		}
		hdr.Name = relPath

		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}

		if !info.IsDir() {
			f, err := os.Open(path)
			if err != nil {
				return err
			}
			defer f.Close()
			if _, err := io.Copy(tw, f); err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		slog.Error("failed to export data", "error", err)
		os.Exit(1)
	}

	slog.Info("data export completed successfully")
}

func runImport(cfg *config.Config, inputFile string) {
	slog.Info("starting data import", "input", inputFile)

	dataDir := cfg.Storage.DataDir

	// Open input
	in, err := os.Open(inputFile)
	if err != nil {
		slog.Error("failed to open input file", "error", err)
		os.Exit(1)
	}
	defer in.Close()

	gr, err := gzip.NewReader(in)
	if err != nil {
		slog.Error("failed to read gzip", "error", err)
		os.Exit(1)
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break // End of archive
		}
		if err != nil {
			slog.Error("failed to read tar", "error", err)
			os.Exit(1)
		}

		target := filepath.Join(dataDir, hdr.Name)

		// Protection against path traversal
		if !strings.HasPrefix(target, filepath.Clean(dataDir)+string(os.PathSeparator)) && target != filepath.Clean(dataDir)+string(os.PathSeparator)+"meta.db" {
			continue
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			os.MkdirAll(target, 0755)
		case tar.TypeReg:
			os.MkdirAll(filepath.Dir(target), 0755)
			f, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR|os.O_TRUNC, os.FileMode(hdr.Mode))
			if err != nil {
				slog.Error("failed to create file", "error", err, "path", target)
				os.Exit(1)
			}
			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				slog.Error("failed to write file content", "error", err, "path", target)
				os.Exit(1)
			}
			f.Close()
		}
	}

	slog.Info("data import completed successfully")
}
