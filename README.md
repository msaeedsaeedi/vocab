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
  <a href="https://github.com/msaeedsaeedi/vocab/actions"><img src="https://img.shields.io/github/actions/workflow/status/msaeedsaeedi/vocab/release.yml?branch=main" alt="CI"></a>
</p>

---

Vocab is a cross-platform desktop daemon that helps you learn vocabulary through a two-phase learning loop — **ambient wallpaper exposure** followed by **active recall notifications** — all running silently in the background.

## Features

- **Two-phase learning** — Words first appear on your wallpaper for passive absorption, then trigger a notification to test recall.
- **FSRS + BKT scheduler** — Combines Free Spaced Repetition Scheduler with Bayesian Knowledge Tracing for optimal review timing.
- **Adaptive pacing** — Automatically adjusts daily word count and active hours based on your engagement patterns.
- **Cross-platform** — Works on Linux, macOS, and Windows.
- **Zero configuration** — Seeds 30 starter words from JSON; just run and learn.
- **Autostart support** — Registers itself to launch on login (Linux `.desktop`, macOS LaunchAgent, Windows Registry).
- **Dev mode** — 60× faster timeouts for testing (`-dev` flag).

## Installation

### Go install

```bash
go install github.com/msaeed/vocab/cmd/vocab@latest
```

### Pre-built binaries

Download the latest release for your platform from the [Releases page](https://github.com/msaeedsaeedi/vocab/releases).

| Platform | Binary |
|----------|--------|
| Linux amd64 | `vocab_linux_amd64.tar.gz` |
| Linux arm64 | `vocab_linux_arm64.tar.gz` |
| macOS amd64 | `vocab_darwin_amd64.tar.gz` |
| macOS arm64 | `vocab_darwin_arm64.tar.gz` |
| Windows amd64 | `vocab_windows_amd64.zip` |

### Build from source

```bash
git clone https://github.com/msaeedsaeedi/vocab.git
cd vocab
make build
```

## Quick start

```bash
# Start the daemon
vocab -daemon

# Use dev mode for testing (60× faster timeouts)
vocab -daemon -dev
```

Once running, Vocab will seed its database, render your first word on the desktop wallpaper, and begin the learning loop.

## Usage

| Flag | Description |
|------|-------------|
| `-daemon` | Run as a background daemon |
| `-dev` | Dev mode — 60× faster timeouts |
| `-reset-db` | Delete database and re-seed |
| `-review <id>` | Record feedback for word by ID |
| `-knew` | Used with `-review` to mark word as known |
| `-preview` | Generate a wallpaper preview image |
| `-register` | Register app for Windows toast notifications |

### Mark a word as known

```bash
vocab -review 1 -knew
vocab -review 1            # mark as not known (auto-lapse)
```

### Reset and start fresh

```bash
vocab -reset-db -daemon
```

### Generate wallpaper preview

```bash
vocab -preview
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

Configuration is stored in the platform's standard data directory:

| Platform | Path |
|----------|------|
| Linux | `~/.local/share/vocab/config.json` |
| macOS | `~/Library/Application Support/vocab/config.json` |
| Windows | `%APPDATA%/vocab/config.json` |

Words are seeded from `data/words.json`. You can add your own words to this file (it's bundled with the binary and also looked up relative to the executable).

## Development

```bash
make build        # Build for Linux
make build-windows  # Cross-compile for Windows
make test         # Run all tests
make lint         # Run golangci-lint
make coverage     # Test coverage report
make clean        # Remove binaries
```

## Acknowledgments

- [fogleman/gg](https://github.com/fogleman/gg) — 2D rendering for wallpaper generation
- [modernc.org/sqlite](https://modernc.org/sqlite) — Pure-Go SQLite driver
- Fonts: [Fraunces](https://github.com/undercasetype/Fraunces), [Inter](https://rsms.me/inter/), [JetBrains Mono](https://www.jetbrains.com/lp/mono/)

## License

[GNU General Public License v3.0](LICENSE)
