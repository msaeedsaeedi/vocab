# Contributing to Vocab

Thanks for helping out. This guide is specific to working on Vocab — it focuses on the toolchain requirements this repo has that you won't find in a generic Go setup.

## Development environment

### Prerequisites

| Tool        | Requirement                        | Why                                                              |
|-------------|------------------------------------|------------------------------------------------------------------|
| Go          | **1.25+ complete install**         | `go.mod` pins `go 1.25.0`; auto-downloaded toolchains break `-cover` (see below) |
| C compiler  | `gcc` (or equivalent, for cgo)     | `go test -race` requires cgo                                      |
| golangci-lint | v2                               | `.golangci.yml` is in v2 format                                   |
| make        | any                                | all dev commands are Make targets                                 |

### Why a complete Go install matters here

`go.mod` requires Go 1.25.0. On a machine with an older `go` and the default `GOTOOLCHAIN=auto`, the `go` command downloads a **toolchain module** (`golang.org/toolchain@...`) instead of using a full install.

Those toolchain modules ship only a subset of precompiled tools, and **`covdata` is missing**. The first time this bites you is `go test -cover` on a package with **no test files** (today that's `internal/wallpaper`):

```console
$ go test ./internal/wallpaper -cover
# github.com/msaeedsaeedi/vocab/internal/wallpaper
go: no such tool "covdata"
```

CI is unaffected because `actions/setup-go` installs a full distribution. Local dev is where you'll hit it. Installing a complete Go distribution at or above the `go` directive version means `GOTOOLCHAIN=auto` never downloads a toolchain module.

### Installing a complete toolchain

**Linux / WSL (Ubuntu/Debian):**

```bash
# C toolchain, so `-race` tests work (the Makefile auto-detects this).
# build-essential pulls in gcc + libc6-dev; without libc6-dev cgo fails to
# compile with "stdlib.h: No such file or directory".
sudo apt-get update && sudo apt-get install -y build-essential

# Full Go distribution — replace 1.25.12 with the latest 1.25.x if newer
cd /tmp
wget https://go.dev/dl/go1.25.12.linux-amd64.tar.gz
sudo rm -rf /usr/local/go
sudo tar -C /usr/local -xzf go1.25.12.linux-amd64.tar.gz

# Make sure /usr/local/go/bin comes before any older Go (e.g. /usr/bin/go)
echo 'export PATH=/usr/local/go/bin:$PATH' >> ~/.bashrc
export PATH=/usr/local/go/bin:$PATH
```

**macOS:**

```bash
brew install go gcc
```

**Windows (native):** install Go from [go.dev/dl](https://go.dev/dl/) and a MinGW-w64 toolchain (e.g. [TDM-GCC](https://jmeubank.github.io/tdm-gcc/)) so cgo/`-race` works; run `make` from Git Bash or MSYS2. Or just use WSL — the repo cross-compiles to Windows with `make build-windows`.

**Verify the install:**

```console
$ go version                # go1.25.12 linux/amd64
$ go env GOROOT             # /usr/local/go  (a real dir, NOT .../golang.org/toolchain@...)
$ go test ./internal/state -cover   # no test files — exercises covdata (fails on partial toolchains)
$ go env CGO_ENABLED        # 1 once gcc is on PATH
```

If `make test` fails with `# runtime/cgo ... _cgo_export.c:3:10: fatal error: stdlib.h: No such file or directory`, the compiler is present but the C library headers aren't — re-run the `apt-get install -y build-essential` step above.

### golangci-lint

The `lint` target uses `$(go env GOPATH)/bin/golangci-lint` if it exists, otherwise it falls back to `go vet`.

```bash
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
```

`.golangci.yml` is in **v2 config format** (`version: "2"`). If you migrate an old v1 config, run `golangci-lint migrate` in the repo root rather than hand-editing it.

### Dependencies

- Keep `go.mod` tidy — run `go mod tidy` whenever imports change, and `go mod download` after touching `go.mod`.
- Bump with `go get -u ./...` (or `go get <module>@latest` for a specific one).
- `github.com/macawls/ogre` is deliberately pinned at `v1.5.1`: upstream's `v2`/`v3` tags lack the `/vN` module path and are unusable as Go modules until [they fix it](https://github.com/Macawls/ogre/issues/15). Don't bump it past `v1.5.1` until that's resolved.

## Day-to-day commands

```bash
make build          # Build for Linux (cross-platform dev)
make build-windows  # Cross-compile for Windows (amd64)
make test           # go test ./... — uses -race when cgo is available
make lint           # golangci-lint run ./... (falls back to go vet)
make coverage       # coverage.out + coverage.html
make clean          # Remove binaries and coverage output
```

`test` and `coverage` auto-detect cgo: if a C toolchain isn't installed they silently drop `-race` instead of failing.

## Verifying a change

1. `gofmt -l cmd internal` must print nothing (CI enforces this on releases).
2. `golangci-lint run ./...` — zero issues.
3. `make test` — all packages green (with `-race` when cgo is present).
4. `make coverage` — HTML report generated.
5. `make build-windows` — confirms the Windows cross-build still works.

## Code conventions

- Match the surrounding style; keep functions small; this repo is lightly commented, so only add comments that carry context.
- **Check errors** — `errcheck` is enabled (`check-type-assertions`, `check-blank`). Wrap errors with `%w`, and log them (`log.Printf`) rather than swallowing.
- Windows-specific code lives in files tagged `//go:build windows` (`internal/notify/`, `internal/wallpaper/set_windows.go`, `internal/tray/`); each has a stub for other platforms so the project builds on Linux for dev.
- Don't add dependencies casually — prefer the standard library or deps already in `go.mod`.
- SQLite schema changes go through the migration machinery in `internal/database` (backups + recovery are automatic; keep it that way).
- `cmd/vocab` is thin wiring (flags, logging, DB, daemon startup). The learning loop and session state machine live in `internal/daemon`; runtime state persistence is in `internal/state`; ephemeral on-disk paths are in `internal/apppaths`; the embedded word catalog is `internal/lexicon`.

## Commit messages

Use [Conventional Commits](https://www.conventionalcommits.org/): `fix:`, `feat:`, `deps:`, `docs:`, `refactor:`, `test:`. Example: `deps: update modernc.org/sqlite to v1.56.0`.

## Pull requests

Fork, branch from `main`, and open a PR. Tagging a release (`v*`) triggers the [Release workflow](.github/workflows/release.yml): gofmt check, `go test ./... -race`, Windows amd64/arm64 cross-builds, then GoReleaser plus the NSIS installer.

## Reporting bugs

From the tray use **Report a problem…**, or run `vocab -report`, and attach the resulting ZIP. It includes logs, version/runtime details, and a SQLite integrity check — the learner database is excluded deliberately.
