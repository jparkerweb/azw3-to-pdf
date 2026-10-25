# EPUB, comics, FB2, text and HTML support, and the ebook-to-pdf rename

**Created:** 2026-10-25
**Source:** [PLAN-DRAFT-20261025.md](./PLAN-DRAFT-20261025.md)
**Status:** Complete

## Summary

Turn azw3-to-pdf into a general ebook to PDF converter named **ebook-to-pdf**.
Add EPUB (2 and 3, reflowable and fixed-layout), comic archives (CBZ),
FictionBook (FB2), plain text and HTML, each as an in-process reader that
produces the existing `ebook.Book`, so the markup parser, renderer and engine
stay format-agnostic. Also make the default page size a saved setting that is
US Letter out of the box.

## Phase Checklist

- [x] Phase 1: Rename and default page size ([phase-1.md](./phase-1.md))
- [x] Phase 2: Reader seam and renderer hooks ([phase-2.md](./phase-2.md))
- [x] Phase 3: EPUB, reflowable and fixed-layout ([phase-3.md](./phase-3.md))
- [x] Phase 4: Comics, FictionBook, text, HTML and docs ([phase-4.md](./phase-4.md))

The codebase-health refactor that is uncommitted on `refactor/duplication` is
committed before Phase 1 starts, since the rename touches every file it changed.

## Parallel Execution Groups

| Group | Phases | Reason |
|-------|--------|--------|
| A | 3, 4 | Both build only on Phase 2. Each reader lives in its own files (`epub*.go` vs `cbz.go`, `fb2.go`, `plaintext.go`, `htmlfile.go`) and replaces only its own Phase 2 stub. Exception: Phase 4's docs tasks (4.15 to 4.19) describe every format, so they run after Phase 3 is complete. |

Phases 1 and 2 are sequential: Phase 1 rewrites every import path, and Phase 2
builds on the renamed module.

## Tech Stack

| Category | Technology | Version | Justification |
|----------|-----------|---------|---------------|
| Language | Go | 1.25 | Pinned in go.mod and both workflows |
| PDF writer | github.com/signintech/gopdf | v0.38.0 | Already used; `AddPageWithOption` gives per-page sizes (verified in gopdf.go:806) |
| Markup | golang.org/x/net/html | v0.58.0 | Existing tokenizer; `x/net/html/charset` for HTML charset detection |
| Archives | archive/zip | stdlib | EPUB, CBZ, `.fb2.zip` |
| XML | encoding/xml | stdlib | container.xml, OPF, NCX, encryption.xml, FB2 |
| Encodings | golang.org/x/text (charmap, htmlindex) | v0.41.0 | Windows-1252 for TXT; FB2 Windows-1251, KOI8-R |
| WebP | golang.org/x/image/webp | v0.45.0 | Same module as the BMP decoder; re-embedded as PNG |
| EPUB reading | in-house, `internal/ebook` | n/a | Available libraries are small, lightly maintained, and still leave the HTML rewriting to us |

## Architecture

### Pattern

**Reader per format, one shared `Book`.** `ebook.Open` sniffs the file and calls
a format reader; every reader returns the existing `Book` (markup, CSS flows,
metadata, 1-based numbered images). Rejected: per-format layout blocks
(duplicates the CSS handling in `css.go`/`scale.go`), and converting everything
to EPUB first (extra lossy hop).

```
file bytes
  -> internal/ebook  Open -> Detect -> mobi | epub | cbz | fb2 | plaintext | htmlfile  -> Book
  -> internal/htmldoc  markup + CSS (+ 3 hooks) -> blocks
  -> internal/pdfout   blocks -> pages (per-page sizes, full-page images)
  -> internal/engine   orchestration, paged-book rules, folder scans
```

### Component Overview

| Component | Where | Responsibility |
|-----------|-------|----------------|
| Dispatcher | `ebook/open.go` | `Open`, `Decode`, `Detect`, `DetectFile`: Palm `BOOKMOBI`/`TEXtREAd`; zip with `mimetype` (EPUB), with a `.fb2` (FB2), mostly images (CBZ); `<FictionBook` XML; `.txt`/`.html` by extension |
| MOBI reader | `ebook/mobi.go` | Today's `Decode`, moved unchanged |
| EPUB reader | `ebook/epub*.go` | container.xml, OPF, spine, `dc:*` metadata, cover (EPUB 3 `cover-image`, EPUB 2 `meta name=cover`), spine XHTML concatenated, image paths rewritten to `recindex`, `<svg><image>` unwrapped, linked and inline CSS gathered into `Flows`, nav/NCX targets marked as headings, duplicate cover page skipped |
| DRM check | `ebook/epub_drm.go` | Refuse `rights.xml`, `sinf.xml`, `license.lcpl`, or any `encryption.xml` entry that is not IDPF/Adobe font obfuscation |
| Comic reader | `ebook/cbz.go` | Natural sort, skip dotfiles and `__MACOSX`, one full-page `<img>` per image, `ComicInfo.xml` title |
| FB2 reader | `ebook/fb2.go` | Streamed XML to HTML (section depth gives heading level), base64 binaries to images, `title-info` metadata, `.fb2.zip` unwrap |
| Text reader | `ebook/plaintext.go` | BOM-aware UTF-8 else Windows-1252; paragraph split; conservative heading patterns; Gutenberg header/licence trimmed |
| HTML reader | `ebook/htmlfile.go` | Charset detection; on-disk images within the file's folder tree; `<title>`/`<meta>` metadata |
| Zip guard | `ebook/zipsafe.go` | Size and ratio caps, path cleaning |
| Markup hooks | `htmldoc/parse.go` | `data-ebook-heading="N"`, `data-ebook-fullpage` on `<img>`, `<ebook:page width height>` |
| Renderer | `pdfout/render_page.go`, `render_image.go` | Per-page sizes, full-page images without margins or furniture, margins recomputed per page, WebP |
| Engine | `engine/convert.go`, `output.go` | Paged books skip cover and title pages; `DiscoverBooks(dir, recursive, includeText)` |
| Default page size | `config`, `presets`, `cli/flags.go`, `tui/screens/layout.go` | Letter when unset, `SizePreset` flag, Save as default |
| Rename | repo-wide | ebook-to-pdf everywhere |

### Data Model

- `Book.Paged bool` (pre-paginated: comics, fixed-layout EPUB)
- `Book.Format` new names: "EPUB 3", "EPUB 2 (fixed layout)", "Comic archive (CBZ)", "FictionBook 2", "Plain text", "HTML"
- `Resource.Kind` adds `"webp"`
- `htmldoc.Block` adds `FullPage bool`, `PageWidth`, `PageHeight` (points, on page breaks)
- `presets.Preset` adds `SizePreset bool`

### API

- `ebook.Open(path string) (*Book, error)`: signature unchanged; its callers untouched
- `ebook.Decode(data []byte) (*Book, error)`: every format except HTML
- `engine.DiscoverBooks(dir string, recursive, includeText bool) []string`
- CLI: `--include-text`

## Risks and Mitigations

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| EPUB CSS uses selectors `css.go` ignores, so some headings look like body text | High | Medium | TOC targets marked as headings independently of CSS; unknown selectors skipped; check real books with the verify-output skill |
| TOC targets that are not block elements are missed | Medium | Low | Fall back to the first block in that file; existing heading detection still runs |
| Plain-text heading guesses misfire | Medium | Medium | Conservative patterns, tests for negative cases |
| Fixed-layout text pages differ from the original | High | Low | Accepted in requirements; noted in README |
| Page size changes break page numbers, headers or bookmarks | Medium | Medium | Margins recomputed per page; full-page images skip furniture; renderer tests for mixed sizes |
| Large comics exhaust memory | Low | Medium | 1 GiB total cap with a clear error |
| Wrong text encoding in FB2 or TXT | Low | Medium | Honour declaration; else UTF-8 if valid, else Windows-1252 |
| `go install` and releases break between code rename and repo rename | Medium | High | Rename the repo before the next tag; GitHub redirects old URLs |
| E-reader users notice A5 became Letter | High | Low | Changelog entry with how to save A5 as default |

## Success Criteria

- A reflowable EPUB 2 and EPUB 3 each convert with spine order, metadata, cover, images and TOC headings as bookmarks
- A fixed-layout EPUB and a CBZ produce one PDF page per book page, each sized to its image, no cover or title page added
- A DRM-locked EPUB fails with the DRM message and exit code 1; a font-obfuscated EPUB converts
- FB2, `.fb2.zip`, TXT and HTML each convert with headings and images where present
- A folder scan skips `.txt`/`.html` unless `--include-text` is passed
- A fresh install renders E-reader on Letter; Phone, Paperback and A4 keep their sizes; Save as default persists
- AZW3/MOBI output unchanged apart from producer name and page size default
- Binary, module path, config folder and docs say ebook-to-pdf; `make ci` green
- No new modules in go.mod

## Quick Reference

### Key Files

| File | Role in this work |
|------|-------------------|
| `internal/ebook/book.go` | `Book`, `Resource` types (gains `Paged`, `"webp"`) |
| `internal/ebook/open.go` (new) | `Open`, `Decode`, `Detect`, `DetectFile` |
| `internal/ebook/mobi.go` (new) | Today's MOBI/KF8 decoding, moved |
| `internal/ebook/zipsafe.go` (new) | Safe zip access for EPUB, CBZ, `.fb2.zip` |
| `internal/ebook/epub.go`, `epub_markup.go`, `epub_toc.go`, `epub_fixed.go`, `epub_drm.go` (new) | EPUB reader |
| `internal/ebook/cbz.go`, `fb2.go`, `plaintext.go`, `htmlfile.go` (new) | Other readers |
| `internal/htmldoc/parse.go`, `doc.go` | The three markup hooks, new `Block` fields |
| `internal/pdfout/render.go`, `render_page.go`, `render_image.go`, `render_blocks.go` | Per-page sizes, full-page images, WebP |
| `internal/engine/convert.go`, `output.go` | Paged-book rules, extensions, folder scans |
| `internal/presets/presets.go` | `SizePreset`, Letter default |
| `internal/cli/flags.go`, `run.go` | Default page size layering, `--include-text` |
| `internal/tui/screens/layout.go`, `filepicker.go`, `batch.go`, `internal/tui/app.go` | Save as default, picker and batch rules |
| `go.mod`, `cmd/ebook-to-pdf/`, `.goreleaser.yaml`, `.github/workflows/*.yml`, `Makefile` | Rename |

### Environment Variables

| Variable | Purpose |
|----------|---------|
| `EBOOK_TO_PDF_CONFIG` | Relocates the configuration folder (replaces `AZW3_TO_PDF_CONFIG`) |

### Config

| Key | Meaning |
|-----|---------|
| `layout.page_size` | Saved default page size; US Letter when unset. Ignored by the size presets (Phone, Paperback, A4) |

### External Dependencies

None added. Every new import comes from modules already in `go.mod`
(`golang.org/x/net`, `golang.org/x/text`, `golang.org/x/image`) or the standard
library.

## Completion Summary

**Feature:** EPUB, comics, FB2, text and HTML support, and the ebook-to-pdf rename | **Completed:** 2026-10-25 | **Completion:** 100% (70 of 70 tasks)

### What Was Built

azw3-to-pdf is now **ebook-to-pdf**, a general ebook converter. `ebook.Open` detects the format from the content (then the name) and hands the file to one of six in-process readers: Kindle (moved unchanged), EPUB 2 and 3 (reflowable and fixed-layout, with DRM refusal), comic archives, FictionBook 2 (plain and zipped), plain text and single HTML files. Every reader produces the same `Book`, so the markup parser, renderer and engine stay format-agnostic apart from three reader hooks (TOC headings, full-page images, sized pages). The default page size is a saved setting, US Letter out of the box, and every doc describes the new formats.

### Files Created

| File | Purpose |
|------|---------|
| `internal/ebook/open.go` | Dispatcher: `Open`, `Decode`, `Detect`, `DetectFile` |
| `internal/ebook/mobi.go` | The Kindle reader, moved out of the old `Decode` |
| `internal/ebook/zipsafe.go` | Size, ratio and path guard for every archive |
| `internal/ebook/epub.go`, `epub_markup.go`, `epub_toc.go`, `epub_fixed.go`, `epub_drm.go` | EPUB reader |
| `internal/ebook/cbz.go`, `fb2.go`, `plaintext.go`, `htmlfile.go` | Comic, FictionBook, text and HTML readers |
| `internal/ebook/*_test.go` (open, zipsafe, zipfixture, epub, cbz, fb2, plaintext, htmlfile) | Reader tests and fixtures |
| `internal/engine/epub_convert_test.go`, `formats_convert_test.go` | End-to-end conversions for every new format |
| `cmd/ebook-to-pdf/main.go` | Renamed entry point |

### Files Modified

| File | Changes |
|------|---------|
| `go.mod`, `.goreleaser.yaml`, `.github/workflows/ci.yml`, `Makefile`, `.gitignore` | Rename to ebook-to-pdf |
| `internal/ebook/book.go` | `Paged`, WebP resources, package doc for every format |
| `internal/htmldoc/parse.go`, `doc.go` | The three reader hooks; `FullPage`, `PageWidth`, `PageHeight` on blocks |
| `internal/pdfout/render*.go`, `options.go` | Per-page sizes, full-page images, WebP; no blank page after a full-page image |
| `internal/engine/convert.go`, `output.go`, `batch.go` | Paged books skip cover and title pages; new extensions; `DiscoverBooks(..., includeText)` |
| `internal/presets/presets.go` | `SizePreset`, `DefaultPageSize` (Letter), `PageSizeFor` |
| `internal/config/paths.go`, `internal/logging/logging.go` | `EBOOK_TO_PDF_CONFIG`, `EBOOK_TO_PDF_LOG_LEVEL` |
| `internal/cli/*.go` | Saved default page size layering, `--include-text`, probe shows paged books |
| `internal/tui/app.go`, `screens/layout.go`, `filepicker.go`, `batch.go` and others | Save as default (`s`), new formats in the picker and batches |
| `README.md`, `AGENTS.md`, `CHANGELOG.md`, `docs/index.html`, `docs/CNAME`, `.claude/skills/ebook-to-pdf-*` | Every format, the rename and the Letter default |

### Dependencies Added

None. New imports come from modules already in `go.mod` (`golang.org/x/net`, `golang.org/x/text`, `golang.org/x/image`) or the standard library.

### Configuration Required

| Variable | Description | Example |
|----------|-------------|---------|
| `EBOOK_TO_PDF_CONFIG` | Relocates the configuration folder (replaces `AZW3_TO_PDF_CONFIG`) | `~/.config/ebook-to-pdf` |
| `EBOOK_TO_PDF_LOG_LEVEL` | Overrides the log level | `debug` |
| `layout.page_size` | Saved default page size; US Letter when unset | `a5` |

### Known Limitations / Blocked Items

- Fixed-layout text pages are reflowed, not positioned exactly; mixed books keep the last fixed page size.
- `.kfx`, `.cbr` and `.cb7` are not read.
- Owner steps still open (Phase 4 summary): rename the GitHub repository, update the remote URL, add the DNS `CNAME` for `ebook-to-pdf.equilllabs.com`, check `go install` after the first tag, and run `ebook-to-pdf-verify-output` on one real book of each new format.
- `make ci` itself and `go test -race` were not run on this Windows machine (no `make`, no cgo); their steps were run individually and CI runs both.

<!-- METRICS_JSON {"step": "document", "total_tasks": 70, "tasks_per_phase": [14, 19, 18, 19], "phase_count": 4, "parallel_groups_identified": 1, "verification_items_added": 1} -->
<!-- METRICS_JSON {"step": "finalize", "completion_rate_at_audit": 1, "tasks_completed": 70, "tasks_total": 70, "verification_failures_found": 1, "documentation_updates_needed": 2} -->
