<p align="center">
  <img src="preview.jpg" alt="Vocab wallpaper preview" width="600">
</p>

<h1 align="center">Vocab</h1>

<p align="center">
  <em>Learn vocabulary through ambient desktop exposure.</em>
</p>

<p align="center">
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-GPLv3-blue.svg" alt="License"></a>
  <a href="https://go.dev/"><img src="https://img.shields.io/badge/Go-1.25+-00ADD8.svg" alt="Go version"></a>
  <a href="https://github.com/msaeedsaeedi/vocab/releases"><img src="https://img.shields.io/github/v/release/msaeedsaeedi/vocab" alt="Release"></a>
</p>

---

Vocab is a Windows desktop daemon that helps you learn vocabulary through a two-phase learning loop — **ambient wallpaper exposure** followed by **active recall notifications** — all running silently in the background.

## Features

- **Two-phase learning** — Words first appear on your wallpaper for passive absorption, then trigger a notification to test recall.
- **FSRS + BKT scheduler** — Combines Free Spaced Repetition Scheduler with Bayesian Knowledge Tracing for optimal review timing.
- **Adaptive pacing** — Automatically adjusts daily word count and active hours based on your engagement patterns.
- **Curated word list** — Ships a built-in curated seed of common words embedded in the binary; no external data or downloads.
- **Tray controls** — Start a session now, pause/resume learning, create a support report, or quit.
- **Autostart support** — Registers itself in the Windows Registry to launch on login.
- **Dev mode** — 60× faster timeouts for testing (`-dev` flag).

## Installation

### Windows Installer (recommended)

Download the installer from the [Releases page](https://github.com/msaeedsaeedi/vocab/releases):

- `Vocab-*-windows-amd64-setup.exe` is fully self-contained — the curated word list is embedded in the binary, so no download is needed.

The installer:

- Installs to `Program Files\Vocab` (single binary)
- Registers Vocab to start automatically on login
- Adds a Start Menu entry so Vocab can be relaunched after quitting from the tray
- Adds an entry in Add/Remove Programs for clean uninstall
- Preserves learner data by default when uninstalling; it asks before removing `%APPDATA%\vocab`

### Go install

```bash
go install github.com/msaeedsaeedi/vocab/cmd/vocab@latest
```

### Pre-built binaries

Download `vocab_*_windows_amd64.zip` from the [Releases page](https://github.com/msaeedsaeedi/vocab/releases) and extract `vocab.exe`. The word list is embedded in the binary — no additional setup required.

### Build from source

```bash
git clone https://github.com/msaeedsaeedi/vocab.git
cd vocab
make build-windows
```

## Quick start

```powershell
# Start the daemon
vocab.exe -daemon

# Use dev mode for testing (60× faster timeouts)
vocab.exe -daemon -dev
```

Once running, Vocab schedules words from the built-in curated seed and begins learning immediately.

On its first run, Vocab asks for permission to temporarily change your wallpaper and send learning notifications. Your previous wallpaper is restored when you pause or quit Vocab.

## Usage

| Flag | Description |
|------|-------------|
| `-daemon` | Run as a background daemon |
| `-dev` | Dev mode — 60× faster timeouts |
| `-reset-db` | Delete Vocab learner state and start fresh |
| `-review <id>` | Record feedback for word by ID |
| `-knew` | Used with `-review` to mark word as known |
| `-learn-now` | Ask a running daemon to start the next session now |
| `-quit` | Ask a running daemon to shut down cleanly |
| `-preview` | Generate a wallpaper preview image |
| `-register` | Register app for Windows toast notifications |
| `-report` | Create a local diagnostic ZIP for a bug report |
| `-version` | Print version and exit |

### Mark a word as known

```powershell
vocab.exe -review 1 -knew
vocab.exe -review 1            # mark as not known (auto-lapse)
```

### Reset and start fresh

```powershell
vocab.exe -reset-db -daemon
```

### Generate wallpaper preview

```powershell
vocab.exe -preview
```

## How it works

Vocab runs as a persistent daemon on your desktop, cycling through two phases for each word:

1. **Expose phase (30 min)** — The word, definition, and example sentence are rendered as your desktop wallpaper. This passive exposure lets your brain subconsciously absorb the word.

2. **Recall phase (up to 2 hr)** — A desktop notification asks you to recall the word's meaning. You respond via **Knew it**, **Struggled**, or **Forgot** (or with `vocab -review <id> -rating 2`, `1`, or `0`). If no response within the window, the word auto-lapses and is re-scheduled.

A tray icon offers **Learn now** to jump straight into the next session (even outside the active window), **Pause learning** to stop new learning until resumed, **Report a problem...** to create a support bundle, and **Quit** to stop the daemon. `-learn-now` and `-quit` on the command line do the same.

Between sessions, an **adaptive engine** tracks your engagement and adjusts:
- Words per day (fewer if you miss reviews)
- Active hours window (shrinks if engagement drops)
- Inter-word gaps based on remaining daily window

The scheduler combines **FSRS** (spaced repetition with stability/difficulty) and **BKT** (Bayesian skill estimation with slip probability) to pick the next word most likely to benefit from review.

## Configuration

Data is stored at `%APPDATA%/vocab/`:

| Path | Purpose |
|------|---------|
| `vocab.db` | SQLite database with learner items, scheduling, reviews, and engagement |
| `logs/vocab.log` | Daemon log (rotated to `vocab.1.log` when it exceeds 2 MB) |
| `daemon-command` | Local command mailbox for `-learn-now` / `-quit` |
| `wallpaper.jpg` | Current word rendered as wallpaper |
| `reports/` | Local diagnostic ZIPs created for support |

Schema migrations run automatically on startup; a recovery backup (`vocab.db.pre-migration-*.bak`) is kept before a migration and any failed database is preserved, so learner data is never destroyed.

The curated word list lives in `internal/words/seed.jsonl` and is embedded into the binary at build time. Vocab stores only learner state (which words are scheduled, review history, engagement) in `vocab.db` — canonical word content comes from the embedded seed.

## Get help

Use the tray icon’s **Report a problem...** item to create a diagnostic ZIP and open it in Explorer. Attach that ZIP to a [GitHub issue](https://github.com/msaeedsaeedi/vocab/issues). You can also run:

```powershell
vocab.exe -report
```

Reports stay on your computer until you choose to attach them. They include recent Vocab logs plus the app version, Windows/Go runtime details, and a SQLite integrity result. They do not include your learner database or the embedded word list.

## Development

```powershell
make build          # Build for Linux (cross-platform dev)
make build-windows  # Cross-compile for Windows
make test           # Run all tests
make lint           # Run golangci-lint
make coverage       # Test coverage report
make clean          # Remove binaries
```

## Acknowledgments

- [macawls/ogre](https://github.com/macawls/ogre) — HTML/CSS rendering for wallpaper generation
- [modernc.org/sqlite](https://modernc.org/sqlite) — Pure-Go SQLite driver
- Fonts: [Fraunces](https://github.com/undercasetype/Fraunces), [Inter](https://rsms.me/inter/), [JetBrains Mono](https://www.jetbrains.com/lp/mono/)

## License

[GNU General Public License v3.0](LICENSE)
