# myTunes

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

A TUI for syncing a remote music library to a local destination such as
a MP3 player or your laptop. Browse the server
library, see what already exists on the destination, and pick — per album — what
to copy or delete in one pass.

Transport is delegated to tools you already trust: myTunes shells out to your
`ssh` and `rsync` binaries and never reimplements them. SSH auth (keys, agent,
`known_hosts`, host-key checking) is whatever your `ssh` is already configured to
do. No secrets are ever stored.

## Features

- **Browse before you sync** — a collapsible Artist → Album tree, with each
  album marked by its state (in sync, server-only, local-only, modified).
- **Per-album copy and delete** — queue exactly what you want in one pass; a
  summary screen shows totals before anything runs.
- **Loadouts** — save a selection as a named preset (e.g. `road-trip`,
  `daily`) and reload it instantly.
- **Live space accounting** — a running size readout and a free-space gate that
  blocks a sync that wouldn't fit.
- **Safe by construction** — deletes are local-only and confined to the
  destination; the server is never deleted from. Deletes run before copies, so
  an aborted run leaves a resumable partial.
- **Single static binary**, no daemon, no database.

## Requirements

- `ssh` and `rsync` available on your `PATH`.
- A remote reachable over SSH whose music library uses a 2-level
  `Artist/Album/Track` layout (as produced by most media-library managers).

## Install

```sh
go install github.com/neeeb1/mytunes@latest
```

Or build from source (Go 1.26+):

```sh
git clone https://github.com/neeeb1/mytunes
cd mytunes
go build -o mytunes .
```

## Quick start

On first launch (or any time with `--setup` flag), myTunes runs a wizard that
asks for:

1. your SSH remote, as `user@host`,
2. the remote music path, and
3. an optional default destination.

These are saved to `~/.config/mytunes/config.toml`. After that, just run:

```sh
mytunes
```

Pick a destination, wait for the scan, check the albums you want, and confirm.
Use `--dry-run` to walk the whole flow without changing any files.

## Usage

```
mytunes [flags]
```

| Flag | Effect |
|---|---|
| `--setup` | Re-run the first-time setup wizard |
| `--dest DIR` | Pre-fill the destination picker |
| `--remote user@host` | Override the configured server |
| `--path PATH` | Override the remote music path |
| `--dry-run` | Plan and report transfer sizes without changing any files |

CLI flags override values from the config file for that run.

## Configuration

Config lives at `~/.config/mytunes/config.toml` (or
`$XDG_CONFIG_HOME/mytunes/config.toml`). The setup wizard writes it for you; you
can also edit it directly:

```toml
remote_user = "you"
remote_host = "192.168.1.10"
remote_path = "/srv/music"
last_dest   = ""
rsync_extra_args = []
```

`rsync_extra_args` is passed through to the copy pass for advanced tuning and is
edited by hand. No passwords or keys are stored — authentication is delegated
entirely to your `ssh`.

## How it works

The TUI moves through a few screens: **setup** (first run only) → **destination**
→ **loading** → **browse** → **confirm** → **progress**.

1. **Destination** — type the target directory (pre-filled from your last run);
   press `Tab` to complete paths. Created if missing; checked for writability.
2. **Loading** — a single SSH `find` scans the server and a local walk scans the
   destination, concurrently.
3. **Browse** — the Artist → Album tree. Each album shows its current state:

   | Icon | State | Meaning |
   |---|---|---|
   | `=` | Synced | on both sides, sizes match |
   | `+` | RemoteOnly | on the server only |
   | `-` | LocalOnly | on the destination only |
   | `~` | Modified | on both sides, a file differs |

   A separate `→COPY` / `→UPDATE` / `→DELETE` tag shows what you've queued this
   session.

   **Keys:** `↑↓`/`jk` move · `←→`/`hl` expand · `space` toggle · `a`/`n` artist
   all/none · `A`/`N` everything all/none · `/` filter · `S` save loadout ·
   `L` loadouts · `s` summary · `r` rescan · `q` quit.
4. **Confirm** — copy / update / delete totals and a free-space gate. Deletes run
   **before** copies, so the gate uses `copy + update − delete`.
5. **Progress** — local deletes first, then an `rsync` copy pass with a live
   progress bar and the file currently transferring. Deletes never touch the
   server.

### Loadouts

A loadout is a named set of albums you want present on the destination. Think a 'daily' loadout for everyday listening, 'roadtrip' for your vacation, or 'date night' for that special someone.

From the browse screen:

- `S` saves the current selection as a new loadout.
- `L` opens the loadout picker, where you can load, rename, duplicate, or delete
  loadouts.

Loading a loadout **replaces** your current selection with its exact set (its
albums checked, everything else unchecked), so it can queue deletes to make the
destination match. You can still tweak the selection afterward; if you do, the
confirm screen offers to update the active loadout before the sync runs.
Loadouts are stored in `~/.config/mytunes/loadouts.toml`.

## Tests

```sh
go test ./...
```

Coverage includes the diff state/action transitions, the rsync arg layout, the
delete-containment guard (never escapes the destination), NUL-delimited scan
parsing of awkward names (`A$AP Rocky`, `Don't Be Dumb`), loadout persistence,
and the TUI screen transitions, setup wizard, and free-space gate.

## License

[MIT](LICENSE)
