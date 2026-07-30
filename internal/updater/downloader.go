package updater

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

type Downloader struct {
	dir    string
	client *http.Client
}

func NewDownloader(dir string) *Downloader {
	return &Downloader{
		dir:    dir,
		client: &http.Client{Timeout: 10 * time.Minute},
	}
}

func (d *Downloader) Download(ctx context.Context, url, filename string) (string, error) {
	if err := os.MkdirAll(d.dir, 0755); err != nil {
		return "", fmt.Errorf("create updates dir: %w", err)
	}

	destPath := filepath.Join(d.dir, filename)
	tmpPath := destPath + ".tmp"

	if _, err := os.Stat(destPath); err == nil {
		return destPath, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("create download request: %w", err)
	}
	req.Header.Set("Accept", "application/octet-stream")
	req.Header.Set("User-Agent", "vocab-updater/1.0")

	resp, err := d.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download: %s", resp.Status)
	}

	out, err := os.Create(tmpPath)
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}

	written, err := io.Copy(out, resp.Body)
	if err != nil {
		out.Close()
		os.Remove(tmpPath)
		return "", fmt.Errorf("write download: %w", err)
	}
	out.Close()

	if err := os.Rename(tmpPath, destPath); err != nil {
		os.Remove(tmpPath)
		return "", fmt.Errorf("rename download: %w", err)
	}

	_ = written
	return destPath, nil
}

func (d *Downloader) CachedPath(filename string) string {
	return filepath.Join(d.dir, filename)
}

func (d *Downloader) HasCached(filename string) bool {
	_, err := os.Stat(filepath.Join(d.dir, filename))
	return err == nil
}
