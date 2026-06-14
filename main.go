// Command mytunes is a Bubble Tea TUI for syncing a remote music library to a
// local destination (e.g. a USB MP3 player). It browses the server, diffs
// against the destination, and lets the user pick what to copy or delete in a
// single guided pass.
package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/neeeb1/mytunes/internal/config"
	"github.com/neeeb1/mytunes/internal/tui"
)

func main() {
	var (
		remote = flag.String("remote", "", "override server as user@host")
		path   = flag.String("path", "", "override remote music path")
		dest   = flag.String("dest", "", "destination directory (pre-fills picker)")
		dryRun = flag.Bool("dry-run", false, "plan and report without changing any files")
		setup  = flag.Bool("setup", false, "re-run the first-time setup wizard")
	)
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "config:", err)
		os.Exit(1)
	}
	cfg = cfg.Apply(config.Overrides{Remote: *remote, Path: *path, Dest: *dest})

	startSetup := *setup || cfg.NeedsSetup()
	app := tui.New(cfg, *dryRun, startSetup)
	if _, err := tea.NewProgram(app, tea.WithAltScreen()).Run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
