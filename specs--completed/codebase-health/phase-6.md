# Phase 6: Remaining tests
**Status:** Complete
**Estimated Tasks:** 4

## Overview

`internal/cli/probe.go`'s `wrapText` has no tests. The batch-runner tests landed in Phase 2; this phase adds the rest of FR-6. Small project phase, so it is short.

## Prerequisites

- Work is on the `develop` branch
- Phase 2's batch-runner tests exist (FR-6 is split across Phases 2 and 6)

## Tasks

- [x] **Task 6.1:** Read `wrapText` in `internal/cli/probe.go` and note its signature and how it counts width (bytes or runes).
- [x] **Task 6.2:** Create `internal/cli/probe_test.go` with table-driven cases for `wrapText`: empty input, input shorter than the width, a single word longer than the width, exact-width input, multi-line wrap at word boundaries, existing newlines in the input, and multi-byte (non-ASCII) text.
  - If a case exposes a bug, do not fix it in this phase (NFR-1): record it in Notes with the failing input, and pin the current behavior in the test with a comment saying it is current behavior.
- [x] **Task 6.3:** Confirm no other file in `internal/cli` and `internal/tui` named in the brief is still untested: list `*.go` without a matching `_test.go` and compare with the ARCHITECTURE-BRIEF; note any leftover in Notes.

## Phase Testing

- [x] **Task 6.4:** Run `go build ./...`, `go vet ./...` and `go test ./...`; all must pass. Run `make ci` once at the end of this phase, since it is the last one and CI runs lint, race tests and build.

## Acceptance Criteria

- `wrapText` has passing table-driven tests
- The batch runner has passing tests (from Phase 2)
- `make ci` passes

## Notes

- Any `wrapText` bug found goes here, not into a fix.
- Task 6.1: `func wrapText(s string, width int) string` in `internal/cli/probe.go:84`. It splits `s` on Unicode whitespace with `strings.Fields` (so existing newlines are not preserved, the text reflows), then greedily packs words onto a line, wrapping with `\n` when `line+1+len(word) > width`. Width is counted with `len(word)`, which is **bytes, not runes** — multi-byte (non-ASCII) text wraps earlier than a rune-correct count would.
- Task 6.2: `wrapText("café io", 7)` exposes the byte-vs-rune bug: `"café"` is 4 runes but 5 bytes, so `wrapText` wraps it onto two lines (`"café\nio"`) where a rune-correct count (4+1+2=7, not over width) would keep it on one line. Not fixed here per NFR-1; pinned as current behavior in `probe_test.go` with a comment.
- Task 6.3: of the two files the ARCHITECTURE-BRIEF named as untested, both are now covered: `probe.go`'s `wrapText` (tested above) and `headless.go`'s worker-pool/outcome-tally logic, which moved to `engine.RunBatch` in Phase 2 and is tested in `internal/engine/batch_test.go`; what is left in `headless.go` (`reportOne`, `dryRun`, `humanBytes`) is thin formatting over that result, not the logic the brief flagged. No leftover against the brief. This does not clear every untested file in `internal/cli`/`internal/tui` generally: `run.go`'s `collectInputs` (path dedup, stdin parsing, directory expansion, extension validation) is untested and carries real logic, just not logic the brief named — out of this phase's scope, left as a known gap for a future phase.
- Task 6.4: `make` is not available in this session's shell, and `go test -race` needs cgo, which needs a C compiler (`gcc`) that this machine does not have. Ran the equivalent commands directly instead: `go build ./...`, `go vet ./...`, `go test ./...`, `golangci-lint run ./...` (CI's lint step), and `gofmt -l .`. All pass with no output from `gofmt -l`. The race build itself was not run locally; CI's own runner has a C toolchain and will run it on push.

## Phase Completion Summary

**Date:** 2026-10-24
**Implementer:** Claude (Implement + Review)

**What was done**

- Read `wrapText` in `internal/cli/probe.go` and confirmed it counts width in bytes (`len(word)`), not runes.
- Added `internal/cli/probe_test.go` with 9 table-driven cases: empty input, shorter than width, a single word longer than width, exact-width input, multi-line wrap at word boundaries, existing newlines (not preserved — the text reflows), multi-byte text (exposes the byte-vs-rune bug, pinned as current behavior per NFR-1, not fixed), zero width, and a word length exactly equal to the width once a line is already started.
- Confirmed no other file named in the ARCHITECTURE-BRIEF is still untested: `headless.go`'s worker-pool logic moved to `engine.RunBatch` in Phase 2 and is tested there; what remains in `headless.go` is thin formatting, not independent logic. (`run.go`'s `collectInputs` is untested and not thin wiring, but it is not a file the brief named; left as a known gap, see Notes.)
- Verified the build: `go build ./...`, `go vet ./...`, `go test ./...`, `golangci-lint run ./...`, `gofmt -l .`. `make` and `go test -race` (needs `gcc`, not on this machine) could not be run locally; CI's own runner has the toolchain for the race build.

**Files**

- Created: `internal/cli/probe_test.go`
- Modified: `specs/codebase-health/phase-6.md` (task checkboxes, Notes, this summary)

**Verification**

- `go build ./...`: passes
- `go vet ./...`: passes
- `go test ./...`: passes, all packages
- `golangci-lint run ./...`: 0 issues
- `gofmt -l .`: no output

**Issues**

- The `wrapText` byte-vs-rune bug found in Task 6.2 was intentionally left unfixed per NFR-1 and is recorded above.
- `make ci` (this phase's acceptance criterion) was not run locally: `make` and `go test -race` need tools (`make`, `gcc`) this machine does not have. The substitute commands above (`go build`, `go vet`, `go test`, `golangci-lint run`, `gofmt -l`) all pass, but the race build and the `make ci` target itself are unverified locally and deferred to CI's run on push.
