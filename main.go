package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

type Release struct {
	ID          int64  `json:"id"`
	TagName     string `json:"tag_name"`
	HTMLURL     string `json:"html_url"`
	PublishedAt string `json:"published_at"`
}

func main() {
	release, err := fetchLatestRelease("bufbuild", "buf")
	if err != nil {
		log.Fatalf("err=%v", err)
	}
	fmt.Printf(
		"id=%d tag=%s htmlurl=%s published at=%s",
		release.ID,
		release.TagName,
		release.HTMLURL,
		release.PublishedAt,
	)
}

func fetchLatestRelease(owner, repo string) (Release, error) {

	if len(owner) <= 0 {
		return Release{}, fmt.Errorf(
			"empty owner")
	}

	if len(repo) <= 0 {
		return Release{}, fmt.Errorf(
			"empty repo")
	}
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest",
		owner, repo,
	)

	client := &http.Client{Timeout: 5 * time.Second}

	resp, err := client.Get(url)
	if err != nil {
		return Release{}, err
	}

	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Release{}, fmt.Errorf(
			"github returned %s", resp.Status)
	}

	var release Release

	err = json.NewDecoder(resp.Body).Decode(&release)

	if err != nil {
		return Release{}, fmt.Errorf("error with request body reading: %v", err)
	}

	return release, nil
}
