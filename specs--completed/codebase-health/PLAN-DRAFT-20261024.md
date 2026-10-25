# Plan: codebase-health
**Created:** 2026-10-24
**Status:** Complete
**Confidence:** 92% (Requirements-clarity 23/25, Feasibility-score 24/25, Integration-points 23/25, Risk-assessment 22/25)
**Conversation:** [PLAN-CONVERSATION-20261024.md](./PLAN-CONVERSATION-20261024.md)

## 1. Executive Summary

This plan fixes the 7 structural rough spots an X-ray survey found in the azw3-to-pdf codebase (`specs/codebase-health/ARCHITECTURE-BRIEF-20261024.md`): scattered `Options` field lists, a duplicated batch runner, duplicated font-scale logic, a 746-line god file, three hand-maintained parallel switches in the TUI, and two files with no tests. No behavior or output changes for end users; this is purely internal.

## 2. Requirements

### 2.1 Functional
- [ ] FR-1: Replace the scattered per-layer `Options` field lists (`pdfout`, `presets`, `config`, `cli/flags`, `tui/app`, `tui/screens/layout`, `tui/messages`) with a single composition path (`Options.ApplyOverrides`), so a new layout knob needs one change, not seven.
- [ ] FR-2: Unify the CLI's worker-pool batch runner (`cli/headless.go`) and the TUI's sequential batch runner (`tui/screens/batch.go`) into one shared implementation (`engine.RunBatch`), keeping the CLI's `--jobs` parallelism and conflict handling, and giving the TUI the same underlying runner.
- [ ] FR-3: Give font-size scaling one owned implementation (`htmldoc.EffectiveScale`) shared by `htmldoc/css.go` and `pdfout/render.go`, removing the duplicate re-dampening and the CSS-independent heading table.
- [ ] FR-4: Split `pdfout/render.go` (746 lines, five jobs) into cohesive files: text layout/line-breaking, page/furniture management, block formatting, image decoding, cover/title drawing — same package, no behavior change.
- [ ] FR-5: Replace `tui/app.go`'s three hand-maintained parallel switches (`footerHints`, `screenBindings`, `pushState`) with one `Screen` interface so adding a screen is one registration, not three.
- [ ] FR-6: Add Go unit tests for `cli/probe.go`'s `wrapText` and for the batch-runner logic (currently untested).
- [ ] FR-7: Fold the preset-to-Options mapping duplicated in `presets.Preset.Options()` and `cli/flags.buildPDFOptions()` into the FR-1 composition path.

### 2.2 Non-Functional
- [ ] NFR-1: No visible behavior change — CLI flags, TUI screens, and PDF output stay equivalent for existing inputs.
- [ ] NFR-2: Maintainability is the only goal; no new features or layout options are added as part of this work.
- [ ] NFR-3: Work lands on a `develop` branch, created before implementation starts.

### 2.3 Out of Scope
New layout presets, new TUI screens, new CLI commands, performance work beyond what falls out naturally from the refactor.

### 2.4 Testing Strategy
| Types | Phase testing | Coverage | Approach |
|---|---|---|---|
| Unit (Go `testing`, table-driven) | `go test ./...` at the end of every phase | Critical paths | Code first |

## 3. Tech Stack
| Category | Technology | Version | Justification |
|---|---|---|---|
| Language | Go | existing (`go.mod`) | Refactor, not a rewrite |
| CLI | Cobra | existing | Internals change, not the command tree |
| TUI | Bubble Tea | existing | Internals change, not the framework |
| PDF rendering | gopdf | existing | Internals change, not the library |
| Config | YAML loader | existing | Consumer of the new composition helper |
| Testing | Go `testing`, table-driven | existing | Matches every other package's convention |

## 4. Architecture

### 4.1 Pattern
Explicit override-merge for Options (FR-1/FR-7), a shared batch runner living in `engine` (FR-2), one owned scale-computation function in `htmldoc` (FR-3), file-split-not-sub-package for the renderer (FR-4), and an interface-driven screen registry for the TUI (FR-5). Rejected: textual-only dedup without changing any of the five shapes (still leaves the same number of edit sites); a reflection-based generic struct merger for Options (harder to debug than an explicit method); a sub-package per render concern (premature for tightly-coupled private logic); a lint rule enforcing switch-case parity (catches the gap, doesn't remove it).

### 4.2 System Context
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
No import cycles; no package reaches into another's internals.

### 4.3 Components
| Component | Responsibility | Inputs/Outputs | Dependencies |
|---|---|---|---|
| `pdfout.Options.ApplyOverrides` | Merge a partial `Options` onto a base, explicit-set wins | in: base + partial `Options`; out: merged `Options` | none new |
| `engine.RunBatch` | Run conversion over N paths at a given concurrency, report per-path results | in: paths, `Options`, concurrency, progress callback; out: results | `ebook`, `htmldoc`, `pdfout` (existing) |
| `htmldoc.EffectiveScale` | Compute final point-scale for a block/heading level from a CSS scale | in: CSS scale, block kind, heading level; out: float scale | none new |
| `pdfout/render_*.go` | Same renderer, reorganized by concern | unchanged | unchanged |
| `tui.Screen` interface | Each screen supplies its own footer hint, bindings, and state-push behavior | in: screen model; out: hint/bindings/pushed state | existing screen models implement it |

### 4.4 Data Model
None — no persisted schema changes.

### 4.5 API Design
Internal Go APIs only (no HTTP/CLI surface changes):
- `func (o Options) ApplyOverrides(partial Options) Options`
- `func RunBatch(paths []string, opts pdfout.Options, concurrency int, progress func(Result)) []Result` (package `engine`)
- `func EffectiveScale(cssScale float64, kind BlockKind, level int) float64` (package `htmldoc`)
- `type Screen interface { FooterHint() string; Bindings() []key.Binding; PushState(any) }` (package `tui`)

## 5. Implementation Phases

### Phase 1: Options composition
**Goal:** FR-1, FR-7 · **Dependencies:** none
- [ ] Task 1.1: Add `Options.ApplyOverrides` in `internal/pdfout/options.go`, with unit tests covering each field type (bool, int, string, pointer-vs-zero precedence)
- [ ] Task 1.2: Migrate `internal/presets/presets.go`'s `Preset.Options()` to build via `ApplyOverrides`
- [ ] Task 1.3: Migrate `internal/config/config.go`'s `LayoutConfig` application to `ApplyOverrides`
- [ ] Task 1.4: Migrate `internal/cli/flags.go`'s `buildPDFOptions` to `ApplyOverrides`
- [ ] Task 1.5: Migrate `internal/tui/app.go`'s `applyLayout`, `internal/tui/screens/layout.go`, and `internal/tui/messages/messages.go`'s `LayoutChangedMsg` to the same path
- [ ] Task 1.6: `go test ./...` passes; manual check that an existing preset still produces the same `Options` values

### Phase 2: Shared batch runner
**Goal:** FR-2, part of FR-6 · **Dependencies:** none
- [ ] Task 2.1: Add `engine.RunBatch(paths, opts, concurrency, progress)` with tests for concurrency, conflict handling, and outcome tallying
- [ ] Task 2.2: Migrate `internal/cli/headless.go`'s `runHeadless` to call `engine.RunBatch` with `--jobs` concurrency
- [ ] Task 2.3: Migrate `internal/tui/screens/batch.go`'s `BatchProgressModel.Start` to call `engine.RunBatch` with concurrency 1
- [ ] Task 2.4: `go test ./...` passes; CLI `--jobs` behavior and TUI sequential behavior unchanged

### Phase 3: Font-scale unification
**Goal:** FR-3 · **Dependencies:** none
- [ ] Task 3.1: Add `htmldoc.EffectiveScale(cssScale, kind, level)` with tests reproducing the current clamp bounds and heading table exactly
- [ ] Task 3.2: Migrate `internal/pdfout/render.go`'s `blockSize` and `heading()` to call `EffectiveScale` instead of re-deriving their own clamps
- [ ] Task 3.3: Golden-check a sample book's rendered text sizes are unchanged before/after (via `/azw3-to-pdf-verify-output`)

### Phase 4: Split render.go
**Goal:** FR-4 · **Dependencies:** Phase 3
- [ ] Task 4.1: Split `internal/pdfout/render.go` into `render_text.go` (layout/line-breaking), `render_page.go` (page/furniture), `render_blocks.go` (heading/paragraph/list/quote), `render_image.go` (image decoding), `render_cover.go` (cover/title) — pure file move, no logic changes
- [ ] Task 4.2: `go build ./...`, `go test ./...` pass; sample book's PDF output byte-identical to before the split

### Phase 5: TUI screen registry
**Goal:** FR-5 · **Dependencies:** none
- [ ] Task 5.1: Define the `Screen` interface in `internal/tui`
- [ ] Task 5.2: Implement it on all 11 screen models, moving each screen's footer-hint text, bindings, and state-push logic out of `app.go`'s switches and into the model itself
- [ ] Task 5.3: Replace `footerHints()`, `screenBindings()`, and `pushState()` in `app.go` with lookups against the registered `Screen`
- [ ] Task 5.4: Table-driven test iterating every registered screen, asserting none panics and all return non-empty hints/bindings
- [ ] Task 5.5: Manual smoke-check of the TUI: footer hints and state carry-over still work per screen

### Phase 6: Remaining tests
**Goal:** rest of FR-6 · **Dependencies:** none
- [ ] Task 6.1: Add `cli/probe_test.go` covering `wrapText` (empty input, short line, long word, multi-line wrap)
- [ ] Task 6.2: `go test ./...` passes

## 6. Risks and Mitigations
| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| PDF visual output regresses during the render-split or scale-unification | Medium | High | Golden/byte comparison on a sample book before and after Phases 3-4; manual check via `/azw3-to-pdf-verify-output` |
| TUI batch concurrency changes unintentionally when sharing `engine.RunBatch` | Medium | Medium | TUI calls `RunBatch` with concurrency fixed at 1; a test asserts that |
| `ApplyOverrides` precedence (zero vs explicitly-set) is ambiguous for bool/int fields | Medium | Medium | Use pointer or explicit "set" tracking; per-field-type unit tests |
| Screen registry refactor silently drops one screen's push-state behavior | Low | Medium | Table-driven test iterating every registered screen |
| Scope creep across 7 touched components in one effort | Medium | Medium | No new features (NFR-2); diff reviewed against the brief before each phase closes |
| The `develop` branch step gets skipped before implementation starts | Low | Medium | Git skill creates/switches to `develop` first; Implement checks branch before its first commit |
| Thin git history (9 commits total) means the "why" behind existing magic constants (e.g. render.go's 0.62 dampening) isn't documented, risking an accidental behavior change | Medium | High | Treat Phases 3-4 as reorganization only; golden-test a sample book's rendered text/dimensions before touching logic |

## 7. Success Criteria
- [ ] `go build ./...` and `go vet ./...` pass
- [ ] `go test ./...` passes, including new tests from Phases 1, 2, 5, and 6
- [ ] A hypothetical new layout option only requires touching `ApplyOverrides` call sites (verified by review against Phase 1's diff)
- [ ] CLI and TUI both call `engine.RunBatch`; CLI's `--jobs` concurrency and TUI's sequential default are unchanged
- [ ] A sample book's rendered PDF is unchanged after Phases 3-4 (checked via `/azw3-to-pdf-verify-output`)
- [ ] `render.go` no longer exists as a single file; its five concerns live in separate `render_*.go` files
- [ ] `tui/app.go`'s three switches are replaced by one `Screen`-interface lookup
- [ ] `wrapText` and the batch runner both have passing tests

## 9. Assumptions
- The person's request to create/switch to a `develop` branch before implementation is a Git-skill step, not a Plan or Implement responsibility — flagged here so Implement checks the branch before its first commit.
- `ApplyOverrides`'s "explicitly set" tracking may require some `Options` fields to become pointers, or an accompanying "set" bitmask/struct — the exact mechanism is an implementation-time decision inside Phase 1, not fixed here.
- No real new layout option is added to prove FR-1/FR-7; Success Criterion 3 is verified by code review of the diff, not a live demo of a new knob.

## Planning Metrics
<!-- METRICS_JSON {"confidence": 92, "clarification_rounds": 1, "functional_requirements_count": 7, "non_functional_requirements_count": 3, "risk_count": 7, "phase_count": 6, "verification_gaps_found": 0, "confidence_breakdown": {"requirements": 23, "feasibility": 24, "integration": 23, "risk": 22}} -->
confidence: 92
clarification_rounds: 1
functional_requirements_count: 7
non_functional_requirements_count: 3
risk_count: 7
phase_count: 6
verification_gaps_found: 0
