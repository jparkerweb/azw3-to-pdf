# Phase 5: TUI screen registry
**Status:** Complete
**Estimated Tasks:** 10

## Overview

`internal/tui/app.go` keeps three hand-maintained parallel switches (`footerHints`, `screenBindings`, `pushState`) that must all gain a case when a screen is added. This phase defines one `Screen` interface that each of the 11 screen models implements, so adding a screen is one registration. Covers FR-5.

## Prerequisites

- Phases 1 and 2 complete (they edit `app.go`, `screens/layout.go` and `screens/batch.go`, which this phase also edits; running first avoids conflicts. This is stricter than the PLAN-DRAFT's "none".)
- Work is on the `develop` branch
- `go test ./...` passes, including `app_test.go`

## Tasks

### Define the interface

- [x] **Task 5.1:** Read `footerHints`, `screenBindings` and `pushState` in `internal/tui/app.go` and write a table into this phase's Notes: for each of the 11 screens, its footer hint string, its bindings, and what `pushState` does for it (including "nothing").
  - Check how this relates to the existing `style.ScreenModel` interface in `internal/tui/style`; prefer extending it or embedding it over defining a second competing interface.
- [x] **Task 5.2:** Define the interface (name it `Screen`, matching the plan) with `FooterHint() string`, `Bindings() []key.Binding` and `PushState(any)`, adjusting types to what the code really uses (for example a typed shared-state pointer instead of `any`).

### Implement on each screen

- [x] **Task 5.3:** Implement the new methods on the first group of screen models in `internal/tui/screens/`, moving each screen's footer hint, bindings and state-push logic out of `app.go` and into the model: the file/library screens.
- [x] **Task 5.4:** Implement them on the layout and preset screens, including the layout state that Phase 1's `ApplyOverrides` path now handles.
- [x] **Task 5.5:** Implement them on the batch, progress and result screens.
- [x] **Task 5.6:** Implement them on the remaining screens (help, settings, about, or whichever of the 11 are left) until all 11 in the Task 5.1 table are covered.
  - Each move must be a straight transplant of the string or logic, with no wording or key changes.

### Replace the switches

- [x] **Task 5.7:** Replace `footerHints()`, `screenBindings()` and `pushState()` in `app.go` with calls through the registered `Screen`.
  - Registration happens in `NewApp`, per AGENTS.md ("Adding things: a screen"). Keep that the single registration point; update the AGENTS.md "A screen" bullet once the new shape is real (Phase 5 Notes should record this for finalization).
  - Delete the three old switches entirely.

### Tests

- [x] **Task 5.8:** Add a table-driven test in `internal/tui/app_test.go` that iterates every registered screen and asserts: constructing it does not panic, `FooterHint()` is non-empty, `Bindings()` is non-empty, and `PushState` with a representative state does not panic.
  - SPEC NOTE: `Bindings()` is legitimately empty (`nil`) for 4 of the 11 screens (Splash, Converting, BatchProgress, BatchComplete) — this matches the pre-existing `screenBindings()` switch, which had no case for them either (the help overlay shows only the global bindings for those screens). `TestRegisteredScreens` calls `Bindings()` and lets it return nil rather than asserting non-empty, since enforcing non-empty would mean inventing bindings that don't exist in the real interface.
- [x] **Task 5.9:** Add a test that fails when a `messages.Screen` constant has no registered screen, so a forgotten registration is caught.
  - Added a `screenCount` sentinel and exported `messages.Screens()` so the test iterates every defined constant without a hand-maintained list; verified it fails when a registration is removed, then restored it.

## Phase Testing

- [x] **Task 5.10:** Run `go build ./...`, `go vet ./...` and `go test ./...`; then smoke-test the TUI (`make run`): visit every screen and confirm the footer hints match the Task 5.1 table and that state carries over between screens as before.
  - `go build ./...`, `go vet ./...`, `go test ./... -count=1`: all pass.
  - No PTY-driving tool was available in this session to run the interactive `make run` TUI directly. Substituted a headless equivalent: a temporary test (removed after this check, not part of the permanent suite) drove the real `App` through every screen and asserted `footerHints()` and the rendered view match the Task 5.1 table exactly, and that the loaded book, preset and conversion state are visible on Info, Presets and Preview after `PushState`. Both checks passed.

## Acceptance Criteria

- `app.go` has no `footerHints`, `screenBindings` or `pushState` switch
- All 11 screens implement the interface and are registered once
- Footer hints and key bindings are unchanged for every screen
- The iteration test and the missing-registration test pass
- `go build ./...`, `go vet ./...`, `go test ./...` pass

## Notes

### Task 5.1 table

`style.ScreenModel` (`internal/tui/style/screen.go`) already defines `Init() tea.Cmd`, `Update(msg tea.Msg) (ScreenModel, tea.Cmd)` and `View() string`. The new `Screen` interface embeds it and adds the three moved behaviors. `screenBindings()`'s existing shape is `[][2]string` (key, description) for the help overlay, not a `bubbles/key.Binding` matcher list — no screen currently builds `key.Binding` values — so `Bindings()` returns `[][2]string` to match what the code really uses, not `[]key.Binding`. `PushState(any)` becomes `PushState(state)` with a typed struct (book, preset, pdf options, output options, result) built once in `app.go` and passed to every screen's `PushState`, since that is the shared state `pushState()` actually threads through.

| Screen | Footer hint | Bindings (help overlay) | `pushState()` does |
| --- | --- | --- | --- |
| `ScreenSplash` | `any key: continue  \|  ?: help  \|  ctrl+c: quit` | none | nothing |
| `ScreenFilePicker` | `enter: open/select  \|  space: add to batch  \|  tab: type a path  \|  esc: back` | enter, space, b, x, tab, d, h (7) | nothing |
| `ScreenInfo` | `enter: choose layout  \|  c: convert now  \|  esc: back` | enter, c (2) | `SetBook(book)` |
| `ScreenPresets` | `up/down: browse  \|  enter: use preset  \|  esc: back` | up/down, enter, r (3) | `SetBook(book)`, `SetPreset(preset)` |
| `ScreenLayout` | `up/down: setting  \|  left/right: change  \|  enter: apply  \|  esc: cancel` | up/down, left/right, space, enter, r (5) | `SetOptions(pdf)` |
| `ScreenPreview` | `enter: convert  \|  l: layout  \|  p: preset  \|  esc: back` | enter, l, p, o (4) | `SetState(book, preset, pdf, output)` |
| `ScreenConverting` | `esc: cancel  \|  ctrl+c: quit` | none | `SetBook(book)` |
| `ScreenComplete` | `o: open PDF  \|  f: folder  \|  n: another book  \|  q: quit` | o, f, n, q (4) | `SetResult(result)` |
| `ScreenBatchQueue` | `enter: start  \|  d: remove  \|  p: preset  \|  l: layout  \|  esc: back` | up/down, d, enter (3) | `SetPreset(preset)` |
| `ScreenBatchProgress` | `esc: stop after this book  \|  ctrl+c: quit` | none | nothing |
| `ScreenBatchComplete` | `f: open folder  \|  n: new batch  \|  q: quit` | none | nothing |

- AGENTS.md "Adding things > A screen" was updated during this phase's review (Stage 2 finding #1, fixed) to describe the `style.Screen` interface instead of the removed footer/help entries in `app.go`.
- Risk: a screen silently losing its push-state behavior. The Task 5.1 table is the checklist to tick against.
- Incident during Task 5.9: a sanity-check `git checkout -- internal/tui/app.go`, meant only to undo a one-line test edit, reverted the *entire* working-tree file to `HEAD`, discarding both this phase's Task 5.7 edits and the uncommitted Phase 1/2 edits already in that file (nothing in this spec has been committed yet). Recovered by rewriting `app.go` from its full content captured earlier in the session's Task 5.1 read, then reapplying the Task 5.7 edits; `git diff --stat` and the full test suite confirm the recovery matches. No further `git checkout` (or other working-tree-discarding command) will be used on a file with uncommitted changes for the rest of this build.

## Phase Completion Summary

**Date:** 2026-10-24
**Implementer:** Claude (Implement + Review)

Replaced `app.go`'s three hand-maintained parallel switches (`footerHints`, `screenBindings`, `pushState`) with a single `style.Screen` interface (embedding the existing `style.ScreenModel`) that each of the 11 screen models implements: `FooterHint() string`, `Bindings() [][2]string`, `PushState(SharedState) ScreenModel`. `app.go`'s three functions now do one type assertion and delegate; `pushState` loops over the registry instead of hand-listing seven screens. Added `messages.Screens()` (backed by a `screenCount` sentinel) so tests can iterate every defined screen without a hand-maintained list.

**Files modified:**
- `internal/tui/style/screen.go` — `Screen` interface, `SharedState` struct
- `internal/tui/app.go` — `footerHints`, `screenBindings`, `pushState` rewritten to delegate through `Screen`
- `internal/tui/messages/messages.go` — `screenCount` sentinel, `Screens()`
- `internal/tui/screens/{splash,filepicker,info,presets,layout,preview,converting,complete,batch}.go` — `FooterHint`, `Bindings`, `PushState` added to all 11 screen models
- `internal/tui/app_test.go` — `TestRegisteredScreens`, `TestEveryScreenIsRegistered`

**Tests:** `go build ./...`, `go vet ./...`, `go test ./... -count=1` all pass. A headless smoke check (temporary, removed) confirmed footer hints and state carry-over match pre-change behavior exactly.

**Issues:**
- SPEC NOTE (Task 5.8): `Bindings()` legitimately returns `nil` for 4 of the 11 screens, matching pre-existing behavior; the test does not force non-empty.
- Incident during Task 5.9: a sanity-check `git checkout -- internal/tui/app.go` discarded the entire file's uncommitted changes (this phase's Task 5.7 work plus Phases 1-4's uncommitted edits to it), not just the one-line test edit it was meant to undo. Recovered by rewriting the file from content captured earlier in this session, then reapplying Task 5.7; verified with `git diff --stat` and the full test suite. No further working-tree-discarding git command was used.
