package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

type Release struct {
	ID          int64  `json:"id"`
	TagName     string `json:"tag_name"`
	HTMLURL     string `json:"html_url"`
	PublishedAt string `json:"published_at"`
}

func RepositoryExists(ctx context.Context, owner, repo string) (bool, error) {
	if len(owner) <= 0 {
		return false, fmt.Errorf("IsRepositoryExists: empty owner")
	}

	if len(repo) <= 0 {
		return false, fmt.Errorf("IsRepositoryExists: empty repo")
	}
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s",
		owner, repo,
	)

	client := &http.Client{Timeout: 5 * time.Second}

	req, err := http.NewRequestWithContext(
		ctx, http.MethodGet, url, nil,
	)
	if err != nil {
		return false, fmt.Errorf("IsRepositoryExists: create request error %w", err)
	}
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := client.Do(req)
	if err != nil {
		return false, fmt.Errorf("IsRepositoryExists: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusNotFound:
		return false, nil
	case http.StatusOK:
		return true, nil
	default:
		return false, fmt.Errorf("check repository %s/%s: github returned %s", owner, repo, resp.Status)
	}
}

func FetchLatestRelease(ctx context.Context, owner, repo string) (Release, error) {

	if len(owner) <= 0 {
		return Release{}, fmt.Errorf(
			"FetchLatestRelease: empty owner")
	}

	if len(repo) <= 0 {
		return Release{}, fmt.Errorf(
			"FetchLatestRelease: empty repo")
	}
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest",
		owner, repo,
	)

	client := &http.Client{Timeout: 5 * time.Second}

	req, err := http.NewRequestWithContext(
		ctx, http.MethodGet, url, nil,
	)

	if err != nil {
		return Release{}, fmt.Errorf("create request: %w", err)
	}

	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		req.Header.Set("Authorization", "Bearer"+token)
	}
	resp, err := client.Do(req)

	if err != nil {
		return Release{}, fmt.Errorf("fetch release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return Release{}, fmt.Errorf("github returned %s", resp.Status)
	}

	var release Release

	err = json.NewDecoder(resp.Body).Decode(&release)

	if err != nil {
		return Release{}, fmt.Errorf("error with request body reading: %w", err)
	}

	return release, nil
}
