package updater

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type Release struct {
	TagName string `json:"tag_name"`
	Name    string `json:"name"`
	Body    string `json:"body"`
	Assets  []Asset `json:"assets"`
	Draft   bool   `json:"draft"`
	Prerelease bool `json:"prerelease"`
}

type Asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	ContentType        string `json:"content_type"`
	Size               int64  `json:"size"`
}

type VersionChecker struct {
	owner  string
	repo   string
	apiURL string
	client *http.Client
}

func NewVersionChecker(owner, repo string) *VersionChecker {
	return &VersionChecker{
		owner:  owner,
		repo:   repo,
		apiURL: fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", owner, repo),
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (v *VersionChecker) Latest(ctx context.Context) (*Release, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "vocab-updater/1.0")

	resp, err := v.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github api: %s", resp.Status)
	}

	var rel Release
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, fmt.Errorf("decode release: %w", err)
	}
	return &rel, nil
}
