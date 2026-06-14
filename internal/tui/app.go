// Package tui implements the Bubble Tea interface: destination picker → loading
// → browse tree → confirm → progress.
package tui

import (
	"context"
	"fmt"
	"os"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/neeeb1/mytunes/internal/config"
	"github.com/neeeb1/mytunes/internal/diff"
	"github.com/neeeb1/mytunes/internal/loadout"
	"github.com/neeeb1/mytunes/internal/local"
	"github.com/neeeb1/mytunes/internal/remote"
	syncpkg "github.com/neeeb1/mytunes/internal/sync"
)

type screen int

const (
	screenSetup screen = iota
	screenDest
	screenLoading
	screenBrowse
	screenConfirm
	screenSyncing
	screenDone
	screenLoadoutPicker
	screenLoadoutName
)

// loadoutAction is the pending name-entry operation on screenLoadoutName.
type loadoutAction int

const (
	loadoutSaveNew loadoutAction = iota
	loadoutRename
	loadoutDuplicate
)

// App is the root Bubble Tea model; all screens share its state.
type App struct {
	cfg    config.Config
	dryRun bool

	screen screen
	width  int
	height int
	err    error

	destInput textinput.Model
	dest      string

	// Tab path completion (shared by the dest picker and setup wizard).
	pathCands  []string // candidate dir names shown under the input
	pathActive int      // highlighted candidate while cycling, or -1
	tabCycle   []string // full candidate values cycled on repeated Tab
	tabIdx     int

	setupInput  textinput.Model
	setupStep   int
	setupRemote string
	setupPath   string
	setupDest   string
	setupErr    string

	spinner spinner.Model
	loadMsg string

	tree   *diff.Tree
	rows   []row
	cursor int
	top    int // first visible row index (scroll window)

	filtering   bool
	filterInput textinput.Model

	summary diff.Summary
	free    int64
	job     syncpkg.Job

	loadouts      loadout.Store
	activeLoadout string             // "" when no loadout is loaded
	loadoutBase   map[[2]string]bool // checked-set snapshot for drift detection
	loadoutMsg    string             // transient note shown on browse (e.g. stale refs)

	loadoutCursor  int             // selection in the picker
	loadoutName    textinput.Model // name entry for save/rename/duplicate
	loadoutAction  loadoutAction
	loadoutSource  string // name being renamed/duplicated
	loadoutNameErr string // inline validation message on the name screen

	prog        progress.Model
	copyPercent float64
	curFile     string // path of the file rsync is currently transferring
	delDone     int
	delTotal    int
	phase       string // "delete" | "copy" | "done"

	events chan tea.Msg // streamed sync progress
}

// New builds the initial model from config and CLI overrides. When startSetup is
// true the app opens on the first-run setup wizard instead of the destination
// picker.
func New(cfg config.Config, dryRun, startSetup bool) *App {
	ti := textinput.New()
	ti.Placeholder = "/run/media/you/USB/Music"
	ti.Prompt = "› "
	ti.SetValue(cfg.LastDest)
	ti.CursorEnd()
	ti.Focus()

	si := textinput.New()
	si.Prompt = "› "

	fi := textinput.New()
	fi.Prompt = "/"
	fi.Placeholder = "filter"

	ni := textinput.New()
	ni.Prompt = "› "
	ni.Placeholder = "loadout name"

	sp := spinner.New()
	sp.Spinner = spinner.Dot

	lo, loErr := loadout.Load()

	app := &App{
		cfg:         cfg,
		dryRun:      dryRun,
		screen:      screenDest,
		destInput:   ti,
		setupInput:  si,
		filterInput: fi,
		loadoutName: ni,
		loadouts:    lo,
		err:         loErr,
		spinner:     sp,
		prog:        progress.New(progress.WithDefaultGradient()),
		events:      make(chan tea.Msg, 16),
	}
	if startSetup {
		app.screen = screenSetup
		app.loadSetupStep()
	}
	return app
}

func (a *App) Init() tea.Cmd { return textinput.Blink }

// ---- messages ----

type validateMsg struct {
	dest string
	err  error
}
type scanDoneMsg struct {
	tree *diff.Tree
	free int64
	err  error
}
type sizingDoneMsg struct {
	summary diff.Summary
	free    int64
	job     syncpkg.Job
	err     error
}
type delProgressMsg struct{ done, total int }
type copyProgressMsg struct{ percent float64 }
type copyFileMsg struct{ name string }
type syncDoneMsg struct{ err error }

// ---- commands ----

func (a *App) validateCmd(raw string) tea.Cmd {
	return func() tea.Msg {
		dest := expandHome(raw)
		if dest == "" {
			return validateMsg{err: fmt.Errorf("no destination given")}
		}
		if _, err := os.Stat(dest); os.IsNotExist(err) {
			if err := os.MkdirAll(dest, 0o755); err != nil {
				return validateMsg{err: fmt.Errorf("create %s: %w", dest, err)}
			}
		}
		if !local.Writable(dest) {
			return validateMsg{err: fmt.Errorf("%s is not a writable directory", dest)}
		}
		return validateMsg{dest: dest}
	}
}

func (a *App) scanCmd() tea.Cmd {
	remoteScanner := remote.Scanner{Remote: a.cfg.Remote(), Path: a.cfg.RemotePath}
	dest := a.dest
	return func() tea.Msg {
		type res struct {
			entries []diff.FileEntry
			err     error
		}
		rc := make(chan res, 1)
		lc := make(chan res, 1)
		go func() {
			e, err := remoteScanner.Scan(context.Background())
			rc <- res{e, err}
		}()
		go func() {
			e, err := local.Scan(dest)
			lc <- res{e, err}
		}()
		r, l := <-rc, <-lc
		if r.err != nil {
			return scanDoneMsg{err: r.err}
		}
		if l.err != nil {
			return scanDoneMsg{err: l.err}
		}
		free, _ := local.FreeBytes(dest) // 0 on error: browse just omits the warning
		return scanDoneMsg{tree: diff.Build(r.entries, l.entries), free: free}
	}
}

func (a *App) sizingCmd() tea.Cmd {
	tree := a.tree
	cfg := a.cfg
	dest := a.dest
	dryRun := a.dryRun
	return func() tea.Msg {
		job := syncpkg.BuildJob(tree, cfg.Remote(), cfg.RemotePath, dest, cfg.RsyncExtraArgs, dryRun)
		if err := job.CheckDeletes(); err != nil {
			return sizingDoneMsg{err: err}
		}
		summary := tree.Summarize()
		// Prefer rsync's precise figure for copy+update; fall back to the
		// in-browse estimate if the dry-run can't run.
		if precise, err := job.DryRunBytes(context.Background()); err == nil {
			summary.CopyBytes = precise
			summary.UpdateBytes = 0
		}
		free, err := local.FreeBytes(dest)
		if err != nil {
			return sizingDoneMsg{err: err}
		}
		return sizingDoneMsg{summary: summary, free: free, job: job}
	}
}

// runSyncCmd kicks off the delete-then-copy worker and returns the first event.
func (a *App) runSyncCmd() tea.Cmd {
	job := a.job
	ch := a.events
	go func() {
		if err := job.RunDeletes(func(done, total int) {
			ch <- delProgressMsg{done, total}
		}); err != nil {
			ch <- syncDoneMsg{err: err}
			return
		}
		if err := job.RunCopy(context.Background(), func(p float64) {
			ch <- copyProgressMsg{p}
		}, func(name string) {
			ch <- copyFileMsg{name}
		}); err != nil {
			ch <- syncDoneMsg{err: err}
			return
		}
		ch <- syncDoneMsg{}
	}()
	return a.waitForEvent()
}

func (a *App) waitForEvent() tea.Cmd {
	return func() tea.Msg { return <-a.events }
}

// ---- update ----

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m := msg.(type) {
	case tea.WindowSizeMsg:
		a.width, a.height = m.Width, m.Height
		a.prog.Width = min(m.Width-4, 60)
		return a, nil

	case tea.KeyMsg:
		return a.handleKey(m)

	case spinner.TickMsg:
		var cmd tea.Cmd
		a.spinner, cmd = a.spinner.Update(msg)
		return a, cmd

	case validateMsg:
		if m.err != nil {
			a.err = m.err
			return a, nil
		}
		a.err = nil
		a.dest = m.dest
		a.screen = screenLoading
		a.loadMsg = "Scanning server and destination…"
		return a, tea.Batch(a.spinner.Tick, a.scanCmd())

	case scanDoneMsg:
		if m.err != nil {
			a.err = m.err
			a.screen = screenDest
			return a, nil
		}
		a.tree = m.tree
		a.free = m.free
		a.expandAll(false)
		a.rebuildRows()
		a.cursor, a.top = 0, 0
		// A re-scan rebuilds the tree, so any active loadout's snapshot no
		// longer points at live albums; re-apply it to restore selection.
		if a.activeLoadout != "" {
			if l, ok := a.loadouts.Get(a.activeLoadout); ok {
				a.applyLoadout(*l)
			} else {
				a.activeLoadout = ""
				a.loadoutBase = nil
			}
		}
		a.screen = screenBrowse
		return a, nil

	case sizingDoneMsg:
		if m.err != nil {
			a.err = m.err
			a.screen = screenBrowse
			return a, nil
		}
		a.summary, a.free, a.job = m.summary, m.free, m.job
		a.screen = screenConfirm
		return a, nil

	case delProgressMsg:
		a.phase = "delete"
		a.delDone, a.delTotal = m.done, m.total
		return a, a.waitForEvent()

	case copyProgressMsg:
		a.phase = "copy"
		a.copyPercent = m.percent
		return a, tea.Batch(a.prog.SetPercent(m.percent/100), a.waitForEvent())

	case copyFileMsg:
		a.phase = "copy"
		a.curFile = m.name
		return a, a.waitForEvent()

	case syncDoneMsg:
		a.phase = "done"
		a.err = m.err
		a.screen = screenDone
		// Persist last-used destination.
		a.cfg.LastDest = a.dest
		_ = a.cfg.Save()
		return a, nil

	case progress.FrameMsg:
		pm, cmd := a.prog.Update(msg)
		a.prog = pm.(progress.Model)
		return a, cmd
	}
	return a, nil
}

func (a *App) View() string {
	switch a.screen {
	case screenSetup:
		return a.viewSetup()
	case screenDest:
		return a.viewDest()
	case screenLoading:
		return a.viewLoading()
	case screenBrowse:
		return a.viewBrowse()
	case screenConfirm:
		return a.viewConfirm()
	case screenSyncing:
		return a.viewProgress()
	case screenDone:
		return a.viewDone()
	case screenLoadoutPicker:
		return a.viewLoadoutPicker()
	case screenLoadoutName:
		return a.viewLoadoutName()
	}
	return ""
}
