package main

import (
	"context"
	"log"
	"os"
	"path/filepath"

	"github.com/romboooo/release-radar/internal/cli"
	"github.com/romboooo/release-radar/internal/store"
	"github.com/romboooo/release-radar/internal/tracker"
)

func main() {
	configDir, err := os.UserConfigDir()
	if err != nil {
		log.Fatal(err)
	}

	appDir := filepath.Join(configDir, "release-radar")
	if err := os.MkdirAll(appDir, 0700); err != nil {
		log.Fatal(err)
	}

	dbPath := filepath.Join(appDir, "release-radar.db")
	db, err := store.Open(dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	service := tracker.New(db)
	if err := cli.Run(context.Background(), os.Args[1:], service, os.Stdout); err != nil {
		log.Fatal(err)
	}
}
