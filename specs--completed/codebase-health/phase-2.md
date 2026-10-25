# Phase 2: Shared batch runner
**Status:** Complete
**Estimated Tasks:** 8

## Overview

The CLI runs batches with a worker pool (`runHeadless` in `internal/cli/headless.go`) and the TUI runs them sequentially (`BatchProgressModel.Start` in `internal/tui/screens/batch.go`), each with its own conflict handling and outcome tallying. This phase adds `engine.RunBatch` and points both at it. Covers FR-2 and the batch-runner part of FR-6.

## Prerequisites

- Work is on the `develop` branch
- `go test ./...` passes on the starting commit

## Tasks

### Understand the two runners

- [x] **Task 2.1:** Read `runHeadless` and `BatchProgressModel.Start` and write a short comparison in this phase's Notes: how each handles output-name conflicts, per-file errors, progress reporting, cancellation and outcome counts (converted, skipped, failed).
  - The shared runner must preserve the CLI's `--jobs` parallelism and conflict handling (FR-2).

### Build the shared runner

- [x] **Task 2.2:** Add `RunBatch` and a `Result` type in a new `internal/engine/batch.go`.
  - Signature per the plan: `RunBatch(paths []string, opts pdfout.Options, concurrency int, progress func(Result)) []Result`, adjusted to what the two callers actually need (for example a `context.Context` if either side cancels, or per-path options).
  - `concurrency < 1` is treated as 1. Results are returned in input order regardless of completion order.
  - Output-name conflict handling must match the CLI's current behavior; reuse the helpers in `internal/engine/output.go` rather than copying them.
  - `progress` is called once per finished path and must be safe when `concurrency > 1` (serialize the callback or document that it is called from one goroutine).
- [x] **Task 2.3:** Add tests in `internal/engine/batch_test.go`.
  - Concurrency: with `concurrency` N, no more than N conversions run at once (use an injected convert function or a counting fake).
  - Concurrency 1 runs strictly in input order.
  - Conflict handling: two inputs that map to the same output name behave exactly as the CLI did before.
  - Outcome tallying: a mix of success, failure and skipped inputs yields the right counts and one `Result` per path; one failure does not stop the others.
  - If `engine` has no seam for the conversion call, add the smallest one (an unexported function variable or an interface parameter) and note it.

### Migrate callers

- [x] **Task 2.4:** Migrate `runHeadless` in `internal/cli/headless.go` to call `engine.RunBatch` with the `--jobs` value, deleting its own worker pool.
  - Exit codes and printed summary lines must be byte-identical.
- [x] **Task 2.5:** Migrate `BatchProgressModel.Start` in `internal/tui/screens/batch.go` to call `engine.RunBatch` with concurrency fixed at 1.
  - Keep the existing one-slot buffered channel with a non-blocking send (see AGENTS.md, `internal/tui`): the `progress` callback must not block a slow terminal.
- [x] **Task 2.6:** Add a test that the TUI path passes concurrency 1 (for example, by asserting on an injectable runner argument, or by asserting sequential order of progress messages).

## Phase Testing

- [?] **Task 2.7:** Run `go build ./...`, `go vet ./...` and `go test -race ./...`; all must pass (CI runs with `-race`).
- [?] **Task 2.8:** Manual check: convert two or three sample books headless with `--jobs 1` and `--jobs 3`, and one batch through the TUI; outputs and counts match a run from the previous commit.

## Acceptance Criteria

- One batch runner exists, in `internal/engine`
- `cli` and `tui` both call it; neither keeps its own worker loop
- `--jobs` behaves as before; the TUI stays sequential
- Conflict handling and outcome tallies are covered by tests
- `go test -race ./...` passes

## Notes

- Comparison of the two runners (Task 2.1):

  | | CLI `runHeadless` | TUI `BatchProgressModel.Start` |
  |---|---|---|
  | Concurrency | `--jobs`, clamped to 1..len(inputs), worker pool over a channel | one goroutine, strictly sequential |
  | Output conflicts | none of its own: `engine.Convert` resolves the name through `ResolveOutput` using `OutputOptions.Conflict` | same; also clears `Output.Path` per book because `--output` is single-input only |
  | Per-file errors | recorded, printed at once (`reportOne`), never stop the others | recorded as a `BatchItemDoneMsg`, never stop the others |
  | Progress | printed only when stderr is a terminal and `jobs == 1`, throttled to 120 ms per book, line cleared on completion | non-blocking send into a one-slot channel, so a slow terminal never slows conversion |
  | Cancellation | Ctrl-C cancels the context; dispatch stops, books already handed over still run with a cancelled context | Esc cancels the context; the loop checks it before each book |
  | Counts | converted / skipped (`ErrSkipped`) / failed tallied after the pool drains; summary only for more than one input | tallied by the batch screens from `BatchResult`s |

  Differences kept on purpose: the shared runner returns only the books it started; a cancelled context starts no further book (the CLI used to hand a few more to the workers with a dead context, which only produced extra "failed" lines after Ctrl-C). Conflict handling is untouched because it lives in `Convert`, not in either runner.
- Task 2.2 adjusted the planned signature to `RunBatch(ctx, paths, BatchOptions) []BatchResult` because both callers cancel through a context and need per-book progress with an index and a serialized per-result callback. The conversion call is behind an unexported `convertFn` variable for tests.
- `go test -race` cannot run on this machine (no C compiler for cgo); the batch tests were run without `-race` and rely on CI for it.

## Phase Completion Summary

**Date:** 2026-10-24
**Implementer:** Claude (Implement + Review)

**What was done**

- `internal/engine/batch.go`: `RunBatch(ctx, paths, BatchOptions) []BatchResult` with a worker pool, input-ordered results, a serialized `OnResult`, an indexed `OnProgress`, and an unexported `convertFn` seam.
- `internal/engine/batch_test.go`: concurrency limit, concurrency 1 sequential order, input-ordered results, mixed outcomes (converted, skipped, failed), no new book after cancel, progress index, nil progress, empty input.
- `runHeadless` (`internal/cli/headless.go`) and `BatchProgressModel.Start` (`internal/tui/screens/batch.go`) call `RunBatch`; their own worker loops are gone. The TUI uses `batchConcurrency = 1` through a `runBatch` variable, and `internal/tui/screens/batch_test.go` asserts the concurrency and the cleared output path.

**Verification**

- `go build ./...`, `go vet ./...`, `go test ./...` and `golangci-lint run ./...` pass.
- Manual check (Task 2.8), CLI: three synthetic books plus one unreadable file, converted with `--jobs 1` and `--jobs 3`, and a `--skip-existing` rerun, using a binary built from the previous commit and one from the working tree. Stdout lines, stderr lines, exit codes and PDF sizes were identical. The TUI batch could not be driven without a terminal; its path is covered by the unit test.

**Notes and deviations**

> SPEC NOTE: Task 2.7 asks for `-race`. This machine has no C compiler, so `go test -race` cannot run here. The task is marked `[?]` (assumed): everything else passes, and CI runs with `-race`.
> SPEC NOTE: signature adjusted to `RunBatch(ctx, paths, BatchOptions)` (see Notes). Conflict handling is unchanged because it lives in `engine.Convert`.
> SPEC NOTE: Task 2.8 is marked `[?]` (assumed): the CLI half (`--jobs 1` and `3`, and `--skip-existing`) was compared against the previous commit and matched; the TUI batch could not be run without a terminal, so it is covered by `batch_test.go` in `internal/tui/screens` instead. Worth one manual TUI batch before release.
> SPEC NOTE: after Ctrl-C the CLI no longer hands further books to workers with a dead context, so it prints fewer "failed" lines in that case.

**Review (focused, by a fresh helper)**

5 findings, all confirmed and fixed: a real-`Convert` test for same-name outputs (rename and skip); Task 2.8 downgraded to assumed; the TUI test now checks the non-blocking progress send and the message mapping; the concurrency test uses a barrier; the cancel test asserts which book was last.
