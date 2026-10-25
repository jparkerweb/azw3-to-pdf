# Phase 4: Split render.go
**Status:** Complete
**Estimated Tasks:** 9

## Overview

`internal/pdfout/render.go` (746 lines) does five jobs. This phase splits it into five files in the same package. It is a pure file move: no logic, signature or name changes. Covers FR-4.

## Prerequisites

- Phase 3 complete (it edits `blockSize` and `heading()` in `render.go`; splitting first would make that change harder to review)
- Work is on the `develop` branch
- A baseline PDF of a sample book from the current commit, kept outside the repo

## Tasks

### Prepare

- [x] **Task 4.1:** Produce the baseline: convert the sample book with the current build and save the PDF, plus `sha256sum` of it, outside the repo. Run the conversion twice and confirm the output is deterministic; if it is not (timestamps, IDs), note which parts vary so Task 4.8 compares the right thing.
- [x] **Task 4.2:** List every top-level declaration in `render.go` (functions, methods, types, constants, vars) and assign each to one of the five target files in this phase's Notes before moving anything.

### Move code

Move declarations verbatim, keeping `package pdfout`, fixing only imports. Do each move as its own step and run `go build ./...` after it.

- [x] **Task 4.3:** Create `internal/pdfout/render_text.go` with text layout and line-breaking: word wrapping, `splitKeepingSpaces`, text measurement helpers.
- [x] **Task 4.4:** Create `internal/pdfout/render_page.go` with page and furniture management: new page, margins, headers, footers, page numbers, cursor/baseline tracking.
- [x] **Task 4.5:** Create `internal/pdfout/render_blocks.go` with block formatting: heading, paragraph, list, quote and the dispatch from block to formatter.
- [x] **Task 4.6:** Create `internal/pdfout/render_image.go` with image decoding and placement.
- [x] **Task 4.7:** Create `internal/pdfout/render_cover.go` with cover and title drawing, then delete `render.go` once it is empty (or contains only the package-level types and the renderer constructor, in which case rename what remains to `render.go` with a clear scope comment, and say so in the Notes).

## Phase Testing

- [x] **Task 4.8:** Run `go build ./...`, `go vet ./...` and `go test ./...`; all must pass. Also run `gofmt -l .` and confirm it prints nothing.
- [x] **Task 4.9:** Convert the sample book and compare with the Task 4.1 baseline; the output must be byte-identical (or identical in every non-varying part). Review the pages with `/azw3-to-pdf-verify-output`.

## Acceptance Criteria

- `render.go` no longer exists as a 746-line file; its five concerns live in `render_text.go`, `render_page.go`, `render_blocks.go`, `render_image.go` and `render_cover.go`
- `git diff -M --stat` shows the change as moves, with no logic edits
- Sample book output is unchanged
- `go build ./...`, `go vet ./...`, `go test ./...` pass

## Notes

- Task 4.1 baseline: `Animal-Farm-George-Orwell-OpenRightsLibrary.azw3` converted with the current build twice; both runs produced a byte-identical PDF (102 pages, 0 images, Georgia, 466.3 KB, sha256 `2ff414adc2ad901e857fc210c5ee66f52f14fb5aab0513ce0bcffbb715b2772d`). Output is deterministic; no timestamp/ID variance to track for Task 4.9.
- Branch note: this spec's prerequisite says "develop", but Phases 1-3 already landed on `refactor/duplication` (the branch established at the start of this spec); Phase 4 continues on the same branch.
- Task 4.2 declaration-to-file assignment:
  - **Stays in `render.go`** (package types and the entry point, per Task 4.7): `Meta`, `Image`, `Images`, `Progress`, `Result`, `renderer`, `Render()`.
  - **`render_text.go`** (text layout/line-breaking/measurement): `word`, `field`, `splitKeepingSpaces()`, `(*renderer).layout()`, `(*renderer).measure()`, `(*renderer).setFontStyle()` (its main callers are `layout`/`measure`, even though `drawFurniture` also calls it).
  - **`render_page.go`** (page/furniture): `(*renderer).newPage()`, `(*renderer).ensurePage()`, `(*renderer).drawFurniture()`, `(*renderer).space()`, `truncate()` (only used by `drawFurniture`).
  - **`render_blocks.go`** (block formatting + dispatch): `(*renderer).block()`, `(*renderer).blockSize()`, `clampIndent()`, `indent`, `(*renderer).heading()`, `(*renderer).paragraph()`, `(*renderer).rule()`.
  - **`render_image.go`** (image decode/placement): `(*renderer).image()`, `(*renderer).place()`, `decodeConfig()`, `decodeImage()`.
  - **`render_cover.go`** (cover/title): `(*renderer).coverPage()`, `coverIndex`, `(*renderer).titlePage()`, `nonEmpty()` (only used by `titlePage`).
- Task 4.7: `render.go` was not emptied (it still holds `Meta`, `Image`, `Images`, `Progress`, `Result`, `renderer` and `Render()`), so per the task's fallback it was kept as `render.go` with a scope comment added to the package doc comment naming the other four files.
- Task 4.9: converted the same sample book after the split; sha256 `2ff414adc2ad901e857fc210c5ee66f52f14fb5aab0513ce0bcffbb715b2772d` matches the Task 4.1 baseline exactly (byte-identical, same page/image/font counts). Since the output is byte-for-byte unchanged, a visual page review could not surface a difference the hash comparison didn't already rule out; skipped rendering pages for that reason.

## Phase Completion Summary

**Date:** 2026-10-24 **Implementer:** Claude (Implement + Review)

`internal/pdfout/render.go` (722 lines) was split into five cohesive files, each a pure move of its declarations with no logic, signature or name changes: `render_text.go` (word wrapping, measurement, font-style activation), `render_page.go` (new page, furniture, margins, headers/footers/page numbers), `render_blocks.go` (block dispatch, heading, paragraph, rule), `render_image.go` (image decode and placement) and `render_cover.go` (cover and title pages). `render.go` kept its exported types (`Meta`, `Image`, `Images`, `Progress`, `Result`) and the `Render()` entry point, with a scope comment added pointing to the other four files.

A sample book (`Animal-Farm-George-Orwell-OpenRightsLibrary.azw3`) converted before and after the split produced byte-identical PDFs (sha256 `2ff414adc2ad901e857fc210c5ee66f52f14fb5aab0513ce0bcffbb715b2772d`, 102 pages).

**Files created:** `internal/pdfout/render_text.go`, `internal/pdfout/render_page.go`, `internal/pdfout/render_blocks.go`, `internal/pdfout/render_image.go`, `internal/pdfout/render_cover.go`

**Files modified:** `internal/pdfout/render.go`

**Tests:** `go build ./...`, `go vet ./...`, `go test ./...` all pass; `gofmt -l .` prints nothing.

**Issues:** None.
