# Codebase Health: Overview
**Created:** 2026-10-24
**Source:** [PLAN-DRAFT-20261024.md](./PLAN-DRAFT-20261024.md)
**Status:** Complete

## Summary

This work fixes the 7 structural rough spots an X-ray survey found in the azw3-to-pdf codebase (`specs/codebase-health/ARCHITECTURE-BRIEF-20261024.md`): scattered `Options` field lists, a duplicated batch runner, duplicated font-scale logic, a 746-line god file, three hand-maintained parallel switches in the TUI, and two files with no tests. No behavior or output changes for end users; this is purely internal.

Work lands on a `develop` branch created before Phase 1 starts (NFR-3). Creating that branch is a Git-skill step, so Implement must check the branch before its first commit.

## Phase Checklist

- [x] [Phase 1: Options composition](./phase-1.md): single `Options.ApplyOverrides` path (FR-1, FR-7)
- [x] [Phase 2: Shared batch runner](./phase-2.md): one `engine.RunBatch` for CLI and TUI (FR-2, part of FR-6)
- [x] [Phase 3: Font-scale unification](./phase-3.md): one `htmldoc.EffectiveScale` (FR-3)
- [x] [Phase 4: Split render.go](./phase-4.md): five cohesive `render_*.go` files (FR-4)
- [x] [Phase 5: TUI screen registry](./phase-5.md): one `Screen` interface replaces three switches (FR-5)
- [x] [Phase 6: Remaining tests](./phase-6.md): `wrapText` tests (rest of FR-6)

## Parallel Execution Groups

| Group | Phases | Reason |
|-------|--------|--------|
| A | 1, 2, 3 | Disjoint files: options/presets/config/flags/layout (1), headless and batch runner (2), `htmldoc/css.go` and `pdfout/render.go` (3) |
| B | 4, 5, 6 | Phase 4 only touches `pdfout` after Phase 3, Phase 5 only touches `tui` after Phases 1 and 2, Phase 6 only adds a new test file in `internal/cli` |

Group B cannot start until Group A is complete: Phase 4 depends on Phase 3, and Phase 5 shares `internal/tui/app.go`, `internal/tui/screens/layout.go` and `internal/tui/screens/batch.go` with Phases 1 and 2. Note this tightens the PLAN-DRAFT, which listed Phase 5 as dependency-free.

## Tech Stack

| Category | Technology | Version | Justification |
|---|---|---|---|
| Language | Go | existing (`go.mod`) | Refactor, not a rewrite |
| CLI | Cobra | existing | Internals change, not the command tree |
| TUI | Bubble Tea | existing | Internals change, not the framework |
| PDF rendering | gopdf | existing | Internals change, not the library |
| Config | YAML loader | existing | Consumer of the new composition helper |
| Testing | Go `testing`, table-driven | existing | Matches every other package's convention |

## Architecture

### Pattern

Explicit override-merge for Options (FR-1/FR-7), a shared batch runner living in `engine` (FR-2), one owned scale-computation function in `htmldoc` (FR-3), file-split-not-sub-package for the renderer (FR-4), and an interface-driven screen registry for the TUI (FR-5).

Rejected: textual-only dedup without changing any of the five shapes (still leaves the same number of edit sites); a reflection-based generic struct merger for Options (harder to debug than an explicit method); a sub-package per render concern (premature for tightly-coupled private logic); a lint rule enforcing switch-case parity (catches the gap, doesn't remove it).

Package layering, which must stay free of import cycles:

```
ebook, htmldoc, config, logging   (leaves, no internal deps)
        |
     pdfout  (imports htmldoc)
        |
    presets  (imports pdfout)
        |
     engine  (imports ebook + htmldoc + pdfout; orchestration hub)
       / \
    cli   tui   (both import nearly everything above; the two entry points)
```

### Component Overview

| Component | Responsibility | Inputs/Outputs | Dependencies |
|---|---|---|---|
| `pdfout.Options.ApplyOverrides` | Merge a partial `Options` onto a base, explicit-set wins | in: base + partial `Options`; out: merged `Options` | none new |
| `engine.RunBatch` | Run conversion over N paths at a given concurrency, report per-path results | in: paths, `Options`, concurrency, progress callback; out: results | `ebook`, `htmldoc`, `pdfout` (existing) |
| `htmldoc.EffectiveScale` | Compute final point-scale for a block/heading level from a CSS scale | in: CSS scale, block kind, heading level; out: float scale | none new |
| `pdfout/render_*.go` | Same renderer, reorganized by concern | unchanged | unchanged |
| `tui.Screen` interface | Each screen supplies its own footer hint, bindings, and state-push behavior | in: screen model; out: hint/bindings/pushed state | existing screen models implement it |

Data model: none, no persisted schema changes.

Internal Go APIs (no HTTP or CLI surface changes). These signatures come from the PLAN-DRAFT and may be adjusted to fit the existing code, as long as each task's behavior holds:

- `func (o Options) ApplyOverrides(partial Options) Options`
- `func RunBatch(paths []string, opts pdfout.Options, concurrency int, progress func(Result)) []Result` (package `engine`)
- `func EffectiveScale(cssScale float64, kind BlockKind, level int) float64` (package `htmldoc`)
- `type Screen interface { FooterHint() string; Bindings() []key.Binding; PushState(any) }` (package `tui`)

## Risks and Mitigations

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| PDF visual output regresses during the render-split or scale-unification | Medium | High | Golden/byte comparison on a sample book before and after Phases 3-4; manual check via `/azw3-to-pdf-verify-output` |
| TUI batch concurrency changes unintentionally when sharing `engine.RunBatch` | Medium | Medium | TUI calls `RunBatch` with concurrency fixed at 1; a test asserts that |
| `ApplyOverrides` precedence (zero vs explicitly-set) is ambiguous for bool/int fields | Medium | Medium | Use pointer or explicit "set" tracking; per-field-type unit tests |
| Screen registry refactor silently drops one screen's push-state behavior | Low | Medium | Table-driven test iterating every registered screen |
| Scope creep across 7 touched components in one effort | Medium | Medium | No new features (NFR-2); diff reviewed against the brief before each phase closes |
| The `develop` branch step gets skipped before implementation starts | Low | Medium | Git skill creates/switches to `develop` first; Implement checks branch before its first commit |
| Thin git history (9 commits total) means the "why" behind existing magic constants (e.g. render.go's 0.62 dampening) isn't documented, risking an accidental behavior change | Medium | High | Treat Phases 3-4 as reorganization only; golden-test a sample book's rendered text/dimensions before touching logic |

## Success Criteria

- `go build ./...` and `go vet ./...` pass
- `go test ./...` passes, including new tests from Phases 1, 2, 5, and 6
- A hypothetical new layout option only requires touching `ApplyOverrides` call sites (verified by review against Phase 1's diff)
- CLI and TUI both call `engine.RunBatch`; CLI's `--jobs` concurrency and TUI's sequential default are unchanged
- A sample book's rendered PDF is unchanged after Phases 3-4 (checked via `/azw3-to-pdf-verify-output`)
- `render.go` no longer exists as a single file; its five concerns live in separate `render_*.go` files
- `tui/app.go`'s three switches are replaced by one `Screen`-interface lookup
- `wrapText` and the batch runner both have passing tests

## Quick Reference

### Key Files

- `internal/pdfout/options.go`: `Options`, `pageSizes`; gains `ApplyOverrides`
- `internal/pdfout/render.go`: split in Phase 4; `blockSize` and `heading()` touched in Phase 3
- `internal/presets/presets.go`: `Preset.Options()`
- `internal/config/config.go`: `LayoutConfig` application
- `internal/cli/flags.go`: `buildPDFOptions`
- `internal/cli/headless.go`: `runHeadless` worker pool
- `internal/cli/probe.go`: `wrapText`
- `internal/engine/convert.go`: home of the new `RunBatch`
- `internal/htmldoc/css.go`: font-size scaling; home of `EffectiveScale`
- `internal/tui/app.go`: `footerHints()`, `screenBindings()`, `pushState()`, `applyLayout`
- `internal/tui/screens/layout.go`, `internal/tui/screens/batch.go`, `internal/tui/messages/messages.go`
- `specs/codebase-health/ARCHITECTURE-BRIEF-20261024.md`: the survey this work answers

### Environment Variables

None.

### External Dependencies

None added. Existing: Cobra, Bubble Tea v2 (`charm.land/*`), `github.com/signintech/gopdf`, `golang.org/x/net/html`, a YAML library. Verification uses a real `.azw3` sample via `/azw3-to-pdf-verify-output`; sample books are gitignored on purpose.

## Completion Summary

**Feature:** Codebase Health | **Completed:** 2026-10-25 | **Completion:** 100% (49/49 tasks, 2 assumed)

### What Was Built

The seven structural rough spots from the X-ray brief are gone. Layout options now merge through one `pdfout.Options.ApplyOverrides` path used by presets, config, CLI flags and the TUI; CLI and TUI share one `engine.RunBatch`; font-size scaling is computed once in `htmldoc.EffectiveScale`; the 746-line `render.go` is split by concern; and the TUI's three parallel switches are replaced by a `style.Screen` interface each screen implements. No end-user behavior or output changes, apart from the Ctrl-C note below.

### Files Created

| File | Purpose |
|------|---------|
| `internal/engine/batch.go` | `RunBatch(ctx, paths, BatchOptions)`: the shared batch runner |
| `internal/engine/batch_test.go` | Concurrency, ordering, cancel and same-name output tests |
| `internal/htmldoc/scale.go` | `EffectiveScale`: the single font-scale computation |
| `internal/pdfout/render_text.go`, `render_page.go`, `render_blocks.go`, `render_image.go`, `render_cover.go` | The renderer, split by concern (pure move) |
| `internal/pdfout/options_test.go` | `ApplyOverrides` per-field-type and coverage tests |
| `internal/cli/flags_test.go` | Flag and config precedence tests |
| `internal/cli/probe_test.go` | `wrapText` tests |
| `internal/tui/screens/batch_test.go` | TUI batch runs at concurrency 1; progress mapping |

### Files Modified

| File | Changes |
|------|---------|
| `internal/pdfout/options.go` | `OptionsOverrides`, `ApplyOverrides`, `OverridesFrom` |
| `internal/pdfout/render.go` | Now only exported types and the `Render` entry point (140 lines) |
| `internal/presets/presets.go`, `internal/cli/flags.go` | Build options through `ApplyOverrides` |
| `internal/cli/headless.go` | Own worker pool removed; calls `engine.RunBatch` |
| `internal/htmldoc/css.go` | Range limits moved into `EffectiveScale` |
| `internal/tui/app.go`, `internal/tui/style/screen.go`, `internal/tui/screens/*.go` | `Screen` interface (`FooterHint`, `Bindings`, `PushState`) replaces three switches |
| `internal/tui/screens/batch.go`, `internal/tui/messages/messages.go` | TUI batch via `engine.RunBatch`; layout message carries overrides |
| `AGENTS.md` | "Adding things" updated for layout settings and screens; finalization added notes on `EffectiveScale`, the `render_*.go` split and `RunBatch` |
| `.claude/skills/azw3-to-pdf-verify-output/SKILL.md` | Reference now points at `render_*.go` and `scale.go` (finalization) |

### Dependencies Added

None.

### Configuration Required

None.

### Known Limitations / Blocked Items

- Task 2.7 assumed: `go test -race` cannot run on the implementing machine (no cgo); `go build`, `go vet` and `go test ./...` pass, and CI runs `-race`.
- Task 2.8 assumed: CLI batches were compared byte-for-byte against the previous commit; a TUI batch was not run by hand. Worth one manual TUI batch before release.
- After Ctrl-C, headless mode no longer hands remaining books to workers, so it prints fewer "failed" lines.
- Work landed on `refactor/duplication` (from `origin/main`) rather than a `develop` branch, by the person's choice.

<!-- METRICS_JSON {"step": "document", "total_tasks": 49, "tasks_per_phase": [10, 8, 8, 9, 10, 4], "phase_count": 6, "parallel_groups_identified": 2, "verification_items_added": 0} -->
