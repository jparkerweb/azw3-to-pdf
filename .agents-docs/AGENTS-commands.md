# Commands
> Part of [AGENTS.md](../AGENTS.md): project guidance for AI coding agents.

```sh
make build          # build ./ebook-to-pdf with version, commit and date ldflags
make test           # go test ./...
make lint           # golangci-lint run ./...
make ci             # lint, test and build
make run            # build and open the terminal interface
make fmt            # gofmt -w .
make install        # go install with the same ldflags
make tidy           # go mod tidy
make clean          # remove the binary, ebook-to-pdf.exe and dist/
make snapshot       # cross-platform build with goreleaser
make release-check  # goreleaser check
```

`.github/workflows/ci.yml` runs golangci-lint, `go test -race` with a coverage
artifact, and a build that executes `ebook-to-pdf version` to prove the binary
works. All three must be green before a pull request merges, so run `make ci`
first. golangci-lint runs on its defaults, with no configuration file.

`make ci` is close to CI but not identical: CI runs the tests as
`go test -race -coverprofile=coverage.out ./...`, and builds for linux/amd64
with `CGO_ENABLED=0` and no ldflags before running `./ebook-to-pdf version`.
Run the race command yourself when a change touches goroutines (the TUI's
conversion channels, `engine.RunBatch`). Test commands and patterns are in
[Testing](./AGENTS-testing.md).

`.github/workflows/release.yml` fires on a `v*` tag: it runs the tests and then
GoReleaser, which publishes archives for Linux, macOS and Windows on amd64 and
arm64 with the version, commit and date compiled in. Release notes are grouped
from conventional commit prefixes, so `feat:`, `fix:` and `perf:` messages land
in the right section of the changelog; `.goreleaser.yaml` drops `docs:`,
`test:`, `chore(deps):` and merge commits.

Go is pinned to 1.25 in both workflows and in `go.mod`; keep them in step.

Three environment variables exist:

- `EBOOK_TO_PDF_CONFIG` relocates the configuration directory
  (`internal/config/paths.go`). `config_test.go` and `tui/app_test.go` set it
  with `t.Setenv` to keep away from the real settings.
- `EBOOK_TO_PDF_LOG_LEVEL` overrides the log level (`internal/logging/logging.go`),
  and wins even over an explicit `--log-level`. The TUI logs to
  `<UserCacheDir>/ebook-to-pdf/logs/ebook-to-pdf.log`; headless runs log to
  stderr.
- `EBOOK_TO_PDF_NO_TUI`, set to anything, forces headless mode
  (`internal/cli/run.go`).

Font discovery on Windows also reads `WINDIR` and `LOCALAPPDATA`
(`internal/pdfout/fonts.go`).
