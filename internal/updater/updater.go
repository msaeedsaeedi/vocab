package updater

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"strings"

	"github.com/msaeedsaeedi/vocab/internal/config"
	"golang.org/x/mod/semver"
)

type Updater struct {
	cfg     *config.Manager
	dataDir string
	version string
	ghOwner string
	ghRepo  string
	checker *VersionChecker
	dl      *Downloader
	inst    *InstallerLauncher
}

func New(cfg *config.Manager, dataDir, currentVersion, ghOwner, ghRepo string) *Updater {
	return &Updater{
		cfg:     cfg,
		dataDir: dataDir,
		version: currentVersion,
		ghOwner: ghOwner,
		ghRepo:  ghRepo,
		checker: NewVersionChecker(ghOwner, ghRepo),
		dl:      NewDownloader(filepath.Join(dataDir, "updates")),
		inst:    NewInstallerLauncher(),
	}
}

type CheckResult struct {
	LatestVersion string
	Release       *Release
	HasUpdate     bool
	InstallerPath string // already downloaded, if available
}

func (u *Updater) Version() string { return u.version }

func (u *Updater) CheckAndDownload(ctx context.Context) (*CheckResult, error) {
	rel, err := u.checker.Latest(ctx)
	if err != nil {
		return nil, err
	}

	if !semver.IsValid(rel.TagName) {
		return nil, fmt.Errorf("invalid semver tag: %s", rel.TagName)
	}

	current := u.version
	if !strings.HasPrefix(current, "v") {
		current = "v" + current
	}

	if !semver.IsValid(current) {
		return &CheckResult{LatestVersion: rel.TagName, Release: rel}, nil
	}

	res := &CheckResult{
		LatestVersion: rel.TagName,
		Release:       rel,
		HasUpdate:     semver.Compare(rel.TagName, current) > 0,
	}

	if !res.HasUpdate {
		return res, nil
	}

	if asset := findInstallerAsset(rel); asset != nil && u.cfg.Get().AutoDownloadUpdates {
		path, err := u.dl.Download(ctx, asset.BrowserDownloadURL, asset.Name)
		if err != nil {
			return res, fmt.Errorf("download update: %w", err)
		}
		res.InstallerPath = path
		log.Printf("update downloaded: %s", path)
	}

	return res, nil
}

func (u *Updater) Install(path string) error {
	if path == "" {
		return fmt.Errorf("no installer path provided")
	}
	log.Printf("launching installer: %s", path)
	return u.inst.Install(path)
}

func (u *Updater) HasCachedInstaller(tag string) (string, bool) {
	installerName := installerFilename(tag)
	if installerName == "" {
		return "", false
	}
	path := u.dl.CachedPath(installerName)
	return path, u.dl.HasCached(installerName)
}
