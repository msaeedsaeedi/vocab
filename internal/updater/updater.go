package updater

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"time"

	"github.com/msaeedsaeedi/vocab/internal/config"
	"golang.org/x/mod/semver"
)

const checkInterval = 24 * time.Hour

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

	res := &CheckResult{
		LatestVersion: rel.TagName,
		Release:       rel,
		HasUpdate:     semver.Compare(rel.TagName, current) > 0,
	}

	if !res.HasUpdate {
		return res, nil
	}

	installerName := u.findInstallerAsset(rel)
	if installerName == nil {
		return res, nil
	}

	if u.cfg.Get().AutoDownloadUpdates {
		path, err := u.dl.Download(ctx, installerName.BrowserDownloadURL, installerName.Name)
		if err != nil {
			log.Printf("update download failed: %v", err)
			return res, nil
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
	path := u.dl.CachedPath(installerName)
	return path, u.dl.HasCached(installerName)
}

func (u *Updater) findInstallerAsset(rel *Release) *Asset {
	for _, a := range rel.Assets {
		if strings.HasSuffix(a.Name, ".exe") && strings.Contains(a.ContentType, "octet-stream") {
			return &a
		}
	}
	for _, a := range rel.Assets {
		if strings.HasSuffix(a.Name, ".exe") {
			return &a
		}
	}
	return nil
}

func installerFilename(tag string) string {
	return fmt.Sprintf("vocab_%s_installer.exe", tag)
}
