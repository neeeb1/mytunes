# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

`mytunes` is a single-binary Bubble Tea TUI that syncs a remote music library
(SSH server, 2-level `<Artist>/<Album>/<File>` layout) to a local destination
such as a USB MP3 player.
Instead of an all-or-nothing mirror, the user browses the server, sees what
already exists on the destination, and queues per-album copies/deletes in one
guided pass. Selections can be saved as named **loadouts** (presets) for quick
re-loading.

## Commands

```
go build -o mytunes .          # build
go test ./...                  # all tests
go test ./internal/diff -run TestName   # single test
go vet ./...

go run . --dest /tmp/mytunes-test --dry-run   # full flow, no file changes
```

`--dry-run` threads through everything: deletes are skipped (`RunDeletes`) and
rsync gets `--dry-run` (`RunCopy`). Use it for any live test against a real
server. CLI flags (`--remote`, `--path`, `--dest`) override the TOML config;
`--setup` (or an empty config) launches the first-run setup wizard.

## Architecture

Transport is **delegated, never reimplemented** — the binary shells out to the
user's `ssh` and `rsync`. There is no native SSH library: this reuses the user's
ssh-agent / known_hosts (more secure, less code) and rsync's resumability.

Data flows one direction through the packages:

- `internal/config` — XDG TOML (`~/.config/mytunes/config.toml`), written empty
  on first run and filled in by the setup wizard. No secrets stored.
- `internal/remote` — one SSH `find ... -printf '%s\t%p\0'` call yields the whole
  tree + per-file sizes. Output is **NUL-delimited and parsed as bytes**, never
  split on whitespace, so names like `A$AP Rocky` / `Don't Be Dumb` round-trip.
- `internal/local` — `WalkDir` on the destination; free-space (`df`) and
  writability checks.
- `internal/diff` — merges remote+local `FileEntry` lists into an Artist→Album
  `Tree`. This is the conceptual core (see below).
- `internal/sync` — turns the tree's queued actions into a `Job`: a delete-first
  local-rm pass, then an rsync `--files-from` copy/update pass.
- `internal/loadout` — TOML store (`~/.config/mytunes/loadouts.toml`) of named
  album-selection presets; the TUI captures/applies them onto the diff tree.
- `internal/tui` — root `App` model in `app.go` switches between screens
  (`setup → dest → loading → browse → confirm → syncing → done`); one file per
  screen. `setup` only runs on first launch or `--setup`.

### The diff model (internal/diff/tree.go)

Two orthogonal concepts — do not conflate them:

- **State** = what is true *now* (`Synced =`, `RemoteOnly +`, `LocalOnly -`,
  `Modified ~`), from comparing the two file→size maps.
- **Action** = what the user queued this session, derived from
  `(State, Checked)` in `Album.Action()`. The UI shows State as the leading icon
  and Action as a separate `→COPY/→UPDATE/→DELETE` tag.

The full `(State, Checked) → Action` table is documented above `Album.Action()`.
If you change checkbox semantics, update that table, `defaultChecked`, and
`Summarize` together — `app_test.go` and `tree_test.go` assert the transitions.

### Sync safety invariants (internal/sync/rsync.go)

- **Deletes run before copies.** This is why the free-space gate is
  `copy + update − delete` (`Summary.NeedBytes`); deleting first reclaims space
  so a near-full drive still fits, and an aborted copy only leaves a resumable
  partial.
- **Deletes are local `os.RemoveAll`, guarded by `withinDest`** — every target
  must `filepath.Clean`-resolve to a strict subdirectory of `Dest` (never the
  root). `CheckDeletes` runs before any removal. The server is never deleted from.
- **No `sh -c`.** All args go through `exec.Command` argv. The one place a remote
  command string is built (the `find` expression) single-quotes the path via
  `shellQuote`.

### TUI concurrency

Scans run concurrently in a `tea.Cmd` goroutine and report back via messages
(`scanDoneMsg`, `sizingDoneMsg`). Live sync progress is pushed onto the buffered
`a.events` channel by a worker goroutine; `waitForEvent` re-arms a command after
each event so Bubble Tea keeps pumping (`delProgressMsg`/`copyProgressMsg`/
`syncDoneMsg`). `scanProgress` splits rsync output on both `\n` and `\r` to catch
carriage-return-refreshed progress lines.

## Layout assumption

A 2-level `<Artist>/<Album>/<File>` layout is assumed and verified on
the server (`find -mindepth 3`). `splitArtistAlbumFile` keeps anything deeper as
part of the file name so multi-disc subdirs don't break parsing.
