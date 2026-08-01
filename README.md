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
- **Versioned Lexicon data** — Uses a verified, read-only Lexicon SQLite bundle while keeping learner state local.
- **Autostart support** — Registers itself in the Windows Registry to launch on login.
- **Dev mode** — 60× faster timeouts for testing (`-dev` flag).

## Installation

### Windows Installer (recommended)

Download the installer from the [Releases page](https://github.com/msaeedsaeedi/vocab/releases):

- `Vocab-*-windows-amd64-setup.exe` bundles a verified Lexicon dataset, so no download is needed.

The installer:

- Installs to `Program Files\Vocab` (binary + bundled Lexicon)
- Registers Vocab to start automatically on login
- Adds an entry in Add/Remove Programs for clean uninstall

### Go install

```bash
go install github.com/msaeedsaeedi/vocab/cmd/vocab@latest
```

### Pre-built binaries

Download `vocab_*_windows_amd64.zip` from the [Releases page](https://github.com/msaeedsaeedi/vocab/releases) and extract `vocab.exe`. This archive does not include a Lexicon bundle, so install one with:

```powershell
vocab.exe -install-lexicon <path-to-lexicon-bundle>
```

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

Once running, Vocab activates the Lexicon bundle that ships alongside the binary, then verifies it before learning begins.

## Usage

| Flag | Description |
|------|-------------|
| `-daemon` | Run as a background daemon |
| `-dev` | Dev mode — 60× faster timeouts |
| `-reset-db` | Delete Vocab learner state and start fresh |
| `-install-lexicon <dir>` | Verify, install, and activate a Lexicon release bundle |
| `-review <id>` | Record feedback for word by ID |
| `-knew` | Used with `-review` to mark word as known |
| `-preview` | Generate a wallpaper preview image |
| `-register` | Register app for Windows toast notifications |
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

2. **Recall phase (up to 2 hr)** — A desktop notification asks you to recall the word's meaning. You respond via `vocab -review <id> -knew` or `vocab -review <id>` (forgot). If no response within the window, the word auto-lapses and is re-scheduled.

Between sessions, an **adaptive engine** tracks your engagement and adjusts:
- Words per day (fewer if you miss reviews)
- Active hours window (shrinks if engagement drops)
- Inter-word gaps based on remaining daily window

The scheduler combines **FSRS** (spaced repetition with stability/difficulty) and **BKT** (Bayesian skill estimation with slip probability) to pick the next word most likely to benefit from review.

## Configuration

Data is stored at `%APPDATA%/vocab/`:

| File | Purpose |
|------|---------|
| `vocab.db` | SQLite database with learner items, scheduling, reviews, engagement, and dataset metadata |
| `datasets/` | Verified read-only Lexicon bundles; the active bundle is recorded in `vocab.db` |
| `wallpaper.jpg` | Current word rendered as wallpaper |

Canonical lexical content is never copied into `vocab.db`. Vocab queries `lexemes`, `senses`, `definitions`, and `examples` from the active Lexicon SQLite file in read-only mode.

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
