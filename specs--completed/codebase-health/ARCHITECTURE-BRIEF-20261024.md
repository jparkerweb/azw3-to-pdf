# Architecture Brief: whole codebase
**Date:** 2026-10-24 · **Area:** whole codebase · **Sources:** code only (git history too shallow for churn/pairs: 9 commits total, most code landed in 1-2 squashed commits)

## 1. `pdfout.Options` field fan-out across 6 files
**Files:** `internal/pdfout/options.go` (`Options` struct) · `internal/presets/presets.go` (`Preset.Options()`) · `internal/config/config.go` (`LayoutConfig`) · `internal/cli/flags.go` (`buildPDFOptions`) · `internal/tui/app.go` (`applyLayout`) · `internal/tui/screens/layout.go` · `internal/tui/messages/messages.go` (`LayoutChangedMsg`)
**Evidence:** each layer keeps its own independent field list for the same struct, with nothing enforcing they stay in sync.
**Today:** to add one new layout knob you must touch all seven files by hand; a field added to `Options` can silently become unreachable from the CLI or TUI.
**Note:** picked to fix.

## 2. Batch conversion logic duplicated between CLI and TUI
**Files:** `internal/cli/headless.go` (`runHeadless`) · `internal/tui/screens/batch.go` (`BatchProgressModel.Start`)
**Evidence:** both reimplement "loop over paths, call `engine.Convert`, collect results, report" independently; the CLI's is a worker pool with `--jobs`, the TUI's is strictly sequential.
**Today:** fixing a conflict-handling bug, or adding parallelism to the TUI, means reimplementing the same logic a second time, since `engine` has no shared batch runner.
**Note:** picked to fix.

## 3. Font-size scaling split between `htmldoc/css.go` and `pdfout/render.go`
**Files:** `internal/htmldoc/css.go` (`parseSize`, `clampScale`) · `internal/pdfout/render.go` (`blockSize`, `heading()`, `headingScale[7]`)
**Evidence:** CSS parses `font-size` into a scale, then render.go independently re-dampens that scale and also hardcodes a separate heading-level table not derived from CSS at all.
**Today:** changing how a CSS `font-size: 200%` ends up on the page means reasoning about three independent clamps spread across two files.
**Note:** picked to fix.

## 4. `render.go` is a 746-line file doing five unrelated jobs
**Files:** `internal/pdfout/render.go`
**Evidence:** text layout/line-breaking, page/furniture management, heading/paragraph/list/quote formatting, image decoding (jpeg/png/gif/bmp), and cover/title-page drawing all live in one file with no sub-boundaries.
**Today:** adding a new block kind (e.g. a table) means editing the same file as someone fixing GIF decoding or bookmark links, so unrelated changes collide by construction.
**Note:** picked to fix.

## 5. `tui/app.go` keeps three hand-maintained parallel switches over all 11 screens
**Files:** `internal/tui/app.go` (`footerHints()`, `screenBindings()`, `pushState()`)
**Evidence:** each is a separate switch/type-switch enumerating the same screen set; nothing fails loudly if one is missed when a screen is added.
**Today:** adding a new screen means remembering to update all three of these plus registration in `NewApp` and message handling in `Update`, or it silently shows no footer hint or stale state.
**Note:** picked to fix.

## 6. Untested formatting/control-flow logic in `cli/probe.go` and `cli/headless.go`
**Files:** `internal/cli/probe.go` (`wrapText`) · `internal/cli/headless.go` (worker-pool/outcome-tally logic)
**Evidence:** unlike every other package, these have real independent logic with no `_test.go` at all (not just indirect coverage).
**Today:** a regression in word-wrap or in how headless tallies success/failure across workers would ship silently; this is distinct from issue #2's duplication risk.
**Note:** picked to fix.

## 7. `presets.Options()` and `cli/flags.buildPDFOptions()` both encode preset-to-Options mapping, in different directions
**Files:** `internal/presets/presets.go` (`Preset.Options()`) · `internal/cli/flags.go` (`buildPDFOptions`)
**Evidence:** this is the first layer of issue #1's fan-out, and the easiest place to start fixing it, with no shared "apply overrides" helper.
**Today:** adding an override that should apply consistently from preset, config and CLI flags means writing it three times instead of once.
**Note:** picked to fix. The person asked that this whole set of fixes land on a dedicated `develop` branch, created/switched to before work starts (a Git step, outside X-ray's and this brief's scope — hand this to whichever skill starts the actual work).
