package main

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/romboooo/release-radar/internal/github"
)

func fetchLatestReleaseHandler(w http.ResponseWriter, r *http.Request) {

	owner := r.PathValue("owner")
	repo := r.PathValue("repo")

	release, err := github.FetchLatestRelease(owner, repo)
	if err != nil {
		log.Printf("fetch release: %v", err)
		http.Error(w, "failed to fetch release", http.StatusBadGateway)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(release); err != nil {
		log.Printf("write response: %v", err)
	}

	// fmt.Printf(
	// 	"id=%d tag=%s htmlurl=%s published at=%s \n",
	// 	release.ID,
	// 	release.TagName,
	// 	release.HTMLURL,
	// 	release.PublishedAt,
	// )
}

func main() {

	http.HandleFunc("GET /repos/{owner}/{repo}/latest", fetchLatestReleaseHandler)
	log.Fatal(http.ListenAndServe(":8080", nil))
}
