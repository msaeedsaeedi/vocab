package dataset

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/msaeedsaeedi/vocab/internal/database"
	"github.com/msaeedsaeedi/vocab/internal/lexicon"
)

type releaseManifest struct {
	Files []struct {
		Filename string `json:"filename"`
	} `json:"files"`
}

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