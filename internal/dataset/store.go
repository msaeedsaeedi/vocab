// Package dataset installs and activates verified Lexicon bundles separately
// from Vocab's writable learner database.
package dataset

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/msaeedsaeedi/vocab/internal/database"
	"github.com/msaeedsaeedi/vocab/internal/lexicon"
)

const releaseURL = "https://api.github.com/repos/msaeedsaeedi/lexicon/releases/latest"

type releaseManifest struct {
	Files []struct {
		Filename string `json:"filename"`
	} `json:"files"`
}

// Install verifies sourceDir before and after copying it, then atomically
// switches the active dataset pointer. Existing verified datasets are retained.
func Install(db *database.DB, sourceDir, datasetsDir string) (*lexicon.Dataset, error) {
	sqlitePath, err := lexicon.VerifyBundle(sourceDir)
	if err != nil {
		return nil, fmt.Errorf("verify source bundle: %w", err)
	}
	source, err := lexicon.Open(sqlitePath)
	if err != nil {
		return nil, err
	}
	version := source.DatasetVersion
	source.Close()
	if err := os.MkdirAll(datasetsDir, 0755); err != nil {
		return nil, fmt.Errorf("create datasets directory: %w", err)
	}
	target := filepath.Join(datasetsDir, version)
	if _, err := os.Stat(target); os.IsNotExist(err) {
		stage, err := os.MkdirTemp(datasetsDir, ".install-")
		if err != nil {
			return nil, fmt.Errorf("create install staging directory: %w", err)
		}
		defer os.RemoveAll(stage)
		if err := copyBundle(sourceDir, stage); err != nil {
			return nil, err
		}
		copiedSQLite, err := lexicon.VerifyBundle(stage)
		if err != nil {
			return nil, fmt.Errorf("verify copied bundle: %w", err)
		}
		opened, err := lexicon.Open(copiedSQLite)
		if err != nil {
			return nil, err
		}
		opened.Close()
		if err := os.Rename(stage, target); err != nil {
			return nil, fmt.Errorf("activate dataset files: %w", err)
		}
	}
	sqlitePath, err = lexicon.VerifyBundle(target)
	if err != nil {
		return nil, fmt.Errorf("verify installed bundle: %w", err)
	}
	opened, err := lexicon.Open(sqlitePath)
	if err != nil {
		return nil, err
	}
	tx, err := db.Begin()
	if err != nil {
		opened.Close()
		return nil, err
	}
	_, err = tx.Exec("UPDATE installed_datasets SET active = 0 WHERE active = 1")
	if err == nil {
		_, err = tx.Exec(`INSERT INTO installed_datasets (dataset_version, schema_version, path, active) VALUES (?, ?, ?, 1) ON CONFLICT(dataset_version) DO UPDATE SET schema_version = excluded.schema_version, path = excluded.path, active = 1`, opened.DatasetVersion, opened.SchemaVersion, sqlitePath)
	}
	if err != nil {
		tx.Rollback()
		opened.Close()
		return nil, fmt.Errorf("record active dataset: %w", err)
	}
	if err := tx.Commit(); err != nil {
		opened.Close()
		return nil, fmt.Errorf("commit active dataset: %w", err)
	}
	return opened, nil
}

func Active(db *database.DB) (*lexicon.Dataset, error) {
	var path string
	if err := db.QueryRow("SELECT path FROM installed_datasets WHERE active = 1").Scan(&path); err != nil {
		return nil, fmt.Errorf("no active Lexicon dataset: %w", err)
	}
	return lexicon.Open(path)
}

// InstallBundled discovers verified release directories in root. This keeps
// offline installers independent of the release version they happen to carry.
func InstallBundled(db *database.DB, root, datasetsDir string) (*lexicon.Dataset, error) {
	candidates := []string{root}
	entries, err := os.ReadDir(root)
	if err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				candidates = append(candidates, filepath.Join(root, entry.Name()))
			}
		}
	}
	best := ""
	bestVersion := ""
	for _, candidate := range candidates {
		sqlitePath, err := lexicon.VerifyBundle(candidate)
		if err != nil {
			continue
		}
		ds, err := lexicon.Open(sqlitePath)
		if err != nil {
			continue
		}
		version := ds.DatasetVersion
		ds.Close()
		if best == "" {
			best, bestVersion = candidate, version
			continue
		}
		comparison, err := lexicon.CompareDatasetVersions(version, bestVersion)
		if err == nil && comparison > 0 {
			best, bestVersion = candidate, version
		}
	}
	if best == "" {
		return nil, fmt.Errorf("no verified Lexicon bundle in %s", root)
	}
	return Install(db, best, datasetsDir)
}

// DownloadAndInstall obtains the latest immutable Lexicon release archive,
// verifies its published checksum, then runs the normal bundle checks before
// activation. It never writes into vocab.db except active metadata.
func DownloadAndInstall(ctx context.Context, db *database.DB, datasetsDir string) (*lexicon.Dataset, error) {
	if err := os.MkdirAll(datasetsDir, 0755); err != nil {
		return nil, fmt.Errorf("create datasets directory: %w", err)
	}
	release, err := fetchRelease(ctx)
	if err != nil {
		return nil, err
	}
	return downloadAndInstallRelease(ctx, db, datasetsDir, release)
}

// CheckAndInstallUpdate downloads and activates a release only when it is
// newer than the active dataset. Invalid or schema-incompatible releases leave
// the active bundle unchanged.
func CheckAndInstallUpdate(ctx context.Context, db *database.DB, datasetsDir string) (*lexicon.Dataset, bool, error) {
	active, err := Active(db)
	if err != nil {
		return nil, false, err
	}
	activeVersion := active.DatasetVersion
	active.Close()
	release, err := fetchRelease(ctx)
	if err != nil {
		return nil, false, err
	}
	newer, err := isNewerRelease(release.TagName, activeVersion)
	if err != nil {
		return nil, false, err
	}
	if !newer {
		return nil, false, nil
	}
	installed, err := downloadAndInstallRelease(ctx, db, datasetsDir, release)
	if err != nil {
		return nil, false, err
	}
	return installed, true, nil
}

func isNewerRelease(releaseTag, activeVersion string) (bool, error) {
	if _, err := lexicon.ParseDatasetVersion(releaseTag); err != nil {
		return false, fmt.Errorf("invalid Lexicon release tag %q: %w", releaseTag, err)
	}
	comparison, err := lexicon.CompareDatasetVersions(strings.TrimPrefix(releaseTag, "v"), activeVersion)
	if err != nil {
		return false, err
	}
	return comparison > 0, nil
}

func downloadAndInstallRelease(ctx context.Context, db *database.DB, datasetsDir string, release *githubRelease) (*lexicon.Dataset, error) {
	archiveURL, checksumURL := "", ""
	for _, asset := range release.Assets {
		if strings.HasSuffix(asset.Name, ".tar.gz") {
			archiveURL = asset.URL
		}
		if strings.HasSuffix(asset.Name, ".tar.gz.sha256") {
			checksumURL = asset.URL
		}
	}
	if archiveURL == "" || checksumURL == "" {
		return nil, fmt.Errorf("Lexicon release %q lacks archive or checksum asset", release.TagName)
	}
	archive, err := fetch(ctx, archiveURL)
	if err != nil {
		return nil, err
	}
	checksum, err := fetch(ctx, checksumURL)
	if err != nil {
		return nil, err
	}
	want := strings.Fields(string(checksum))
	if len(want) == 0 {
		return nil, fmt.Errorf("invalid Lexicon archive checksum")
	}
	got := sha256.Sum256(archive)
	if fmt.Sprintf("%x", got) != want[0] {
		return nil, fmt.Errorf("Lexicon archive checksum mismatch")
	}
	stage, err := os.MkdirTemp(datasetsDir, ".download-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(stage)
	if err := extractArchive(archive, stage); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(stage)
	if err != nil {
		return nil, err
	}
	if len(entries) != 1 || !entries[0].IsDir() {
		return nil, fmt.Errorf("unexpected Lexicon archive layout")
	}
	bundleDir := filepath.Join(stage, entries[0].Name())
	previewPath, err := lexicon.VerifyBundle(bundleDir)
	if err != nil {
		return nil, err
	}
	preview, err := lexicon.Open(previewPath)
	if err != nil {
		return nil, err
	}
	version := preview.DatasetVersion
	preview.Close()
	comparison, err := lexicon.CompareDatasetVersions(strings.TrimPrefix(release.TagName, "v"), version)
	if err != nil || comparison != 0 {
		return nil, fmt.Errorf("Lexicon release tag %q does not match bundle dataset version %q", release.TagName, version)
	}
	return Install(db, bundleDir, datasetsDir)
}

type githubRelease struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
	} `json:"assets"`
}

func fetchRelease(ctx context.Context) (*githubRelease, error) {
	data, err := fetch(ctx, releaseURL)
	if err != nil {
		return nil, fmt.Errorf("fetch Lexicon release: %w", err)
	}
	var release githubRelease
	if err := json.Unmarshal(data, &release); err != nil {
		return nil, fmt.Errorf("decode Lexicon release: %w", err)
	}
	return &release, nil
}

func fetch(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/octet-stream")
	req.Header.Set("User-Agent", "vocab/0.3")
	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download %s: %s", url, resp.Status)
	}
	return io.ReadAll(resp.Body)
}

func extractArchive(archive []byte, destination string) error {
	gz, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return fmt.Errorf("open Lexicon archive: %w", err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if header.Typeflag == tar.TypeDir {
			continue
		}
		if header.Typeflag != tar.TypeReg {
			return fmt.Errorf("unsupported archive entry %q", header.Name)
		}
		path := filepath.Join(destination, header.Name)
		if !strings.HasPrefix(filepath.Clean(path), filepath.Clean(destination)+string(os.PathSeparator)) {
			return fmt.Errorf("unsafe archive path %q", header.Name)
		}
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return err
		}
		out, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		if err != nil {
			return err
		}
		if _, err := io.Copy(out, tr); err != nil {
			out.Close()
			return err
		}
		if err := out.Close(); err != nil {
			return err
		}
	}
}

func copyBundle(sourceDir, destination string) error {
	data, err := os.ReadFile(filepath.Join(sourceDir, "release-manifest.json"))
	if err != nil {
		return fmt.Errorf("read release manifest: %w", err)
	}
	var manifest releaseManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return fmt.Errorf("parse release manifest: %w", err)
	}
	names := []string{"release-manifest.json"}
	for _, file := range manifest.Files {
		names = append(names, file.Filename)
	}
	for _, name := range names {
		if filepath.Base(name) != name {
			return fmt.Errorf("unsafe bundle filename %q", name)
		}
		if err := copyFile(filepath.Join(sourceDir, name), filepath.Join(destination, name)); err != nil {
			return err
		}
	}
	return nil
}

func copyFile(source, destination string) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(destination)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return nil
}
