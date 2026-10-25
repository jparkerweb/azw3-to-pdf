# Phase 3: Font-scale unification
**Status:** Complete
**Estimated Tasks:** 8

## Overview

Font-size scaling is computed twice: `internal/htmldoc/css.go` clamps and dampens the CSS scale, and `internal/pdfout/render.go` re-dampens it and keeps a CSS-independent heading table. This phase gives it one owner, `htmldoc.EffectiveScale`, with output unchanged. Covers FR-3.

## Prerequisites

- Work is on the `develop` branch
- A real sample book is available locally for before/after comparison (sample books are gitignored; `settings.local.json` lets tools find them)

## Tasks

### Capture the baseline

- [x] **Task 3.1:** Before changing any code, convert a sample book with the current build and keep the PDF as the baseline outside the repo (for example in the session scratchpad).
  - Also extract per-block font sizes if feasible: add a temporary debug print or a test helper that records the size `blockSize` returns for every block in a fixture, and save that list.
  - Use `/azw3-to-pdf-verify-output` to look at the pages as well as the bytes.

### Implement the shared function

- [x] **Task 3.2:** Read the scaling code in `internal/htmldoc/css.go`, `blockSize` and `heading()` in `internal/pdfout/render.go`, and write each clamp bound, dampening factor (including the 0.62 constant) and heading-level table into this phase's Notes.
- [x] **Task 3.3:** Add `EffectiveScale(cssScale float64, kind BlockKind, level int) float64` in `internal/htmldoc/css.go` (or a new `internal/htmldoc/scale.go`).
  - It must reproduce today's result exactly, applying the clamp and dampening once. Adjust the signature to the real block-kind type in `doc.go`.
  - Document each magic constant with a comment on why it exists, to the extent that Task 3.2 can establish it; say "origin unknown, kept for output parity" otherwise.
- [x] **Task 3.4:** Add tests in `internal/htmldoc/htmldoc_test.go` that pin the current behavior: clamp lower and upper bounds, each heading level 1 to 6 (or however many the table has), body text with and without a CSS scale, and a value at each boundary.

### Migrate callers

- [x] **Task 3.5:** Make `css.go` store the raw CSS scale and have its consumers call `EffectiveScale`, removing the first dampening step so it is applied only once.
- [x] **Task 3.6:** Migrate `blockSize` and `heading()` in `internal/pdfout/render.go` to call `htmldoc.EffectiveScale`, removing the duplicate re-dampening and the CSS-independent heading table.
  - Do not touch layout or wrapping code in this phase; Phase 4 moves it.

## Phase Testing

- [x] **Task 3.7:** Run `go build ./...`, `go vet ./...` and `go test ./...`; all must pass.
- [x] **Task 3.8:** Golden check: convert the same sample book and compare with the Task 3.1 baseline. Per-block sizes must be identical; if PDF bytes differ only by a timestamp or document ID, note it and compare page content instead. Review the pages with `/azw3-to-pdf-verify-output`.

## Acceptance Criteria

- Scaling is computed in exactly one function, `htmldoc.EffectiveScale`
- `render.go` no longer has its own clamp or heading size table
- Every block in the sample book gets the same font size as before
- Unit tests pin the clamp bounds and heading table
- `go build ./...`, `go vet ./...` and `go test ./...` pass

## Notes

- This is a reorganization only (risk: thin git history, undocumented constants). If a golden check differs, revert the change rather than adjusting the constants.
- Findings from Task 3.2 (the scaling rule before this phase):
  - `css.go` `parseSize` clamped every parsed size to 0.5..3.5 (`clampScale`). It only clamped; the first "dampening" the plan mentions did not exist. A zero or negative size was clamped up to 0.5, which also kept it from reading as "unset" (`Block.Scale == 0`).
  - `render.go` `blockSize`: no CSS scale means the body size; otherwise a scale above 1 was damped to `1 + (scale-1)*0.62`, the point size was floored at 5pt and capped at 2.4 times the body size.
  - `render.go` `heading()`: a CSS-independent table `{1, 1.75, 1.45, 1.28, 1.15, 1.06, 1.0}` indexed by level 1-6 (any other level used 3); a stylesheet size replaced it only when larger.
  - Origins: the 0.62 factor, the 0.5/3.5 clamp and the heading table have no recorded reason in the git history. They are kept for output parity and commented as such in `internal/htmldoc/scale.go`.
- Result: `htmldoc.EffectiveScale(cssScale, kind, level)` in `internal/htmldoc/scale.go` owns the clamp, the 0.62 damping, the 2.4 cap and the heading table. `parseSize` stores the raw size (only forcing a non-positive size to 0.5 so it stays "set"). `pdfout` keeps just the absolute 5pt floor.

## Phase Completion Summary

**Date:** 2026-10-24
**Implementer:** Claude (Implement + Review)

**What was done**

- New `internal/htmldoc/scale.go`: `EffectiveScale` plus the documented constants and heading table.
- `internal/htmldoc/css.go`: `clampScale` replaced by `positiveScale`; the parser now stores the raw CSS size.
- `internal/pdfout/render.go`: `headingScale` and the duplicate clamp/damping are gone; `blockSize` is the body size times `EffectiveScale`, floored at 5pt, and `heading()` uses it.
- `internal/htmldoc/htmldoc_test.go`: an oracle test that reproduces the old rule and compares it with the new one over 16 CSS scales, 4 block kinds, levels -1..8 and 5 body sizes; pinned values for the clamp bounds, damping, cap and every heading level; `parseSize` raw-value cases.

**Verification**

- `go build ./...`, `go vet ./...`, `go test ./...` and `golangci-lint run ./...` pass.
- Golden check (Task 3.8) on Animal Farm (`.azw3`, 429 blocks, 102 pages): the per-block font sizes at five body sizes (5, 9.5, 11, 16, 40) are byte-identical before and after, the extracted text (`pdftotext -layout`) is identical, and the PDF files themselves are byte-identical (477468 bytes). With identical bytes there is nothing for a visual review to find, so no page images were rendered.
- This book only uses stylesheet sizes of 0.75 to 1.5, so the clamp and damping extremes are covered by the oracle test rather than by the book.

**Notes and deviations**

> SPEC NOTE: the plan expected a "first dampening step" in `css.go`; there was only a clamp, which moved into `EffectiveScale`.
> SPEC NOTE: the book supplied was an EPUB, which this tool cannot read; the person then supplied `Animal-Farm-George-Orwell-OpenRightsLibrary.azw3`.
> SPEC NOTE: the stay-on-`refactor/duplication` decision from Phase 1 applies here too.

**Review (focused, by a fresh helper)**

3 findings, all confirmed and fixed: a NaN CSS size (`nanem`) is now treated like zero (smallest allowed size) instead of silently becoming body size, with NaN and Inf test cases; the constants' comments now say "origin unknown, kept for output parity" instead of inventing a reason; a `pdfout` test pins `blockSize`, and the oracle's body-size range is explained. The Animal Farm PDF is still byte-identical after the fixes. (The old code produced a NaN point size for `nanem`, which is garbage output, so that one input differs on purpose.)
