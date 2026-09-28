package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/romboooo/release-radar/internal/cli"
	"github.com/romboooo/release-radar/internal/store"
	"github.com/romboooo/release-radar/internal/tracker"
	"github.com/romboooo/release-radar/internal/tui"
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

	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()
	if len(os.Args) == 1 {
		if err := tui.Run(ctx, service); err != nil {
			log.Fatal(err)
		}
		return
	}
	if err := cli.Run(ctx, os.Args[1:], service, os.Stdout); err != nil {
		log.Fatal(err)
	}
}
