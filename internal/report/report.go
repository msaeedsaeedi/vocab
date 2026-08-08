// Package report builds the local diagnostic bundle used to debug issues and
// file bug reports. It never uploads anything and never includes learner content.
package report

import (
	"archive/zip"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/msaeedsaeedi/vocab/internal/apppaths"
	"github.com/msaeedsaeedi/vocab/internal/database"
)

// Info is the runtime/build metadata embedded in the bundle header.
type Info struct {
	Version string
	Commit  string
	Date    string
	Go      string
	OS      string
	Arch    string
}

// Write creates a shareable bundle containing a compact runtime/database health
// snapshot and the recent application logs, and returns its path.
func Write(db *database.DB, logPath string, info Info) (string, error) {
	dir, err := apppaths.ReportDir()
	if err != nil {
		return "", fmt.Errorf("report dir: %w", err)
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("create report directory: %w", err)
	}
	path := filepath.Join(dir, "vocab-report-"+time.Now().UTC().Format("20060102T150405Z")+".zip")
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return "", fmt.Errorf("create bundle: %w", err)
	}
	zw := zip.NewWriter(f)
	closeWithError := func(err error) (string, error) {
		_ = zw.Close()
		_ = f.Close()
		return "", err
	}

	var integrity string
	if err := db.QueryRow("PRAGMA integrity_check").Scan(&integrity); err != nil {
		integrity = "error: " + err.Error()
	}
	diagnostics := fmt.Sprintf("generated_utc=%s\nversion=%s\ncommit=%s\nbuilt=%s\ngo=%s\nos=%s/%s\ndatabase_integrity=%s\n",
		time.Now().UTC().Format(time.RFC3339), info.Version, info.Commit, info.Date, info.Go, info.OS, info.Arch, integrity)
	entry, err := zw.Create("diagnostics.txt")
	if err != nil {
		return closeWithError(fmt.Errorf("add diagnostics: %w", err))
	}
	if _, err := io.WriteString(entry, diagnostics); err != nil {
		return closeWithError(fmt.Errorf("write diagnostics: %w", err))
	}
	for _, source := range []string{logPath, filepath.Join(filepath.Dir(logPath), "vocab.1.log")} {
		in, err := os.Open(source)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return closeWithError(fmt.Errorf("open log: %w", err))
		}
		entry, err := zw.Create(filepath.Join("logs", filepath.Base(source)))
		if err == nil {
			_, err = io.Copy(entry, in)
		}
		closeErr := in.Close()
		if err != nil {
			return closeWithError(fmt.Errorf("add log: %w", err))
		}
		if closeErr != nil {
			return closeWithError(fmt.Errorf("close log: %w", closeErr))
		}
	}
	if err := zw.Close(); err != nil {
		_ = f.Close()
		return "", fmt.Errorf("finish bundle: %w", err)
	}
	if err := f.Close(); err != nil {
		return "", fmt.Errorf("close bundle: %w", err)
	}
	log.Printf("diagnostic report written: %s", path)
	return path, nil
}
