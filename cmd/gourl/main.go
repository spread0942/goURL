package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gourl/internal/core"
	"gourl/internal/ui"
)

func main() {
	config, err := os.UserConfigDir()
	if err != nil {
		config = "."
	}
	dir := flag.String("data-dir", filepath.Join(config, "gourl"), "Directory for saved requests and environments")
	timeout := flag.Duration("timeout", 30*time.Second, "HTTP request timeout")
	flag.Parse()
	if *timeout <= 0 {
		fail(fmt.Errorf("timeout must be positive"))
	}
	state, err := core.Load(*dir)
	if err != nil {
		fail(err)
	}
	if err := core.Save(*dir, state); err != nil {
		fail(fmt.Errorf("data directory is not writable: %w", err))
	}
	if err := ui.New(*dir, state, *timeout).Run(); err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "gourl:", err)
	os.Exit(1)
}
