# EPUB, comics, FB2, text and HTML support, and the ebook-to-pdf rename

**Created:** 2026-10-25
**Status:** Complete
**Confidence:** 91% (24/25 requirements clarity, 23/25 technical feasibility, 22/25 integration points, 22/25 risk assessment)
**Conversation:** [PLAN-CONVERSATION-20261025.md](./PLAN-CONVERSATION-20261025.md)

## 1. Executive Summary

Turn azw3-to-pdf into a general ebook to PDF converter named **ebook-to-pdf**.
Add EPUB (2 and 3, reflowable and fixed-layout), comic archives (CBZ),
FictionBook (FB2), plain text and HTML, each as an in-process reader that
produces the existing `ebook.Book`, so the markup parser, renderer and engine
stay format-agnostic. Also make the default page size a saved setting that is
US Letter out of the box.

## 2. Requirements

### 2.1 Functional

- [ ] **FR-1** EPUB 2 and 3 (reflowable) convert: reading order from the spine, metadata from the package file, cover, the book's CSS, images, and chapter headings from the table of contents.
- [ ] **FR-2** Fixed-layout EPUB: one book page per PDF page at the book's declared size; full-page images exact, positioned text reflowed onto its page.
- [ ] **FR-3** DRM-locked EPUBs are refused with an error naming the file and suggesting a DRM-free copy; font-obfuscated books still convert.
- [ ] **FR-4** Comic archives (`.cbz`): images in natural page order (2 before 10), one per PDF page, each page sized to its image; title from `ComicInfo.xml` when present.
- [ ] **FR-5** FictionBook (`.fb2`, `.fb2.zip`): sections, titles, emphasis, embedded images and metadata.
- [ ] **FR-6** Plain text (`.txt`): UTF-8 or Windows-1252, paragraphs from blank lines, chapter headings recognised from lines like "Chapter 1".
- [ ] **FR-7** HTML (`.html`, `.htm`): a single file, with images next to it on disk.
- [ ] **FR-8** The format is detected from the file's contents, not just its name, everywhere a book is picked: CLI, batch, the TUI file picker, `probe` and the info screen.
- [ ] **FR-9** Rename to **ebook-to-pdf**: binary, `cmd/` folder, Go module path, config and cache folders, `EBOOK_TO_PDF_CONFIG`, GoReleaser, CI, README, AGENTS.md, changelog, local skills, help and splash text, and the promo page moved to `ebook-to-pdf.equilllabs.com`. Old settings are not carried over.
- [ ] **FR-10** Every new failure (unknown format, broken archive, locked book) says what to do and names the file.
- [ ] **FR-11** Default page size is a saved setting, US Letter when unset. It wins over presets except the size presets (Phone, Paperback, A4), which keep their own page. `--page-size` and the layout screen still override for one run. The TUI layout screen gets a "Save as default" action that writes `layout.page_size`.
- [ ] **FR-12** Folder scans (`--recursive`, batch screen) skip `.txt` and `.html` unless `--include-text` is passed; naming such a file directly always works.

### 2.2 Non-Functional

- [ ] **NFR-1** Pure Go and in-process: no Calibre, no external tools, no cgo; zero new modules in `go.mod`.
- [ ] **NFR-2** One reader per format behind the shared `Book` shape.
- [ ] **NFR-3** `htmldoc` stays "not a browser": only three small hooks are added.
- [ ] **NFR-4** Archives are read safely: never extracted to disk, per-entry and total size caps (1 GiB total), compression ratio cap, cleaned paths; HTML images confined to the file's folder tree; no network access.
- [ ] **NFR-5** AZW3/MOBI conversion speed unchanged; EPUB in the same range.
- [ ] **NFR-6** `make ci` stays green; Go stays 1.25.

### 2.3 Out of Scope

- Removing or bypassing DRM of any kind (declined: circumvention, and against the project's rule)
- KFX, `.cbr`
- Exact fixed-layout positioning (absolute CSS)
- Migrating settings from the old `azw3-to-pdf` config folder
- Keeping `azw3-to-pdf` as an alias binary
- A `config` CLI command for the default page size
- Renaming the GitHub repo and adding the DNS record (manual steps for the owner, listed in Task 4.6)
<!-- VERIFICATION: pointed at Task 4.6 after the build phases were regrouped from seven to four -->


### 2.4 Testing Strategy

| Aspect | Choice |
|--------|--------|
| Types | Unit |
| Phase Testing | No |
| Coverage | Critical paths |

Fixture books are generated inside the tests with `archive/zip`; no copyrighted samples are checked in.

## 3. Tech Stack

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

## 4. Architecture

### 4.1 Pattern

**Reader per format, one shared `Book`.** `ebook.Open` sniffs the file and calls
a format reader; every reader returns the existing `Book` (markup, CSS flows,
metadata, 1-based numbered images). Rejected: per-format layout blocks
(duplicates the CSS handling in `css.go`/`scale.go`), and converting everything
to EPUB first (extra lossy hop).

### 4.2 System Context

```
file bytes
  -> internal/ebook  Open -> Detect -> mobi | epub | cbz | fb2 | plaintext | htmlfile  -> Book
  -> internal/htmldoc  markup + CSS (+ 3 hooks) -> blocks
  -> internal/pdfout   blocks -> pages (per-page sizes, full-page images)
  -> internal/engine   orchestration, paged-book rules, folder scans
```

### 4.3 Components

| Component | Where | Responsibility |
|-----------|-------|----------------|
| Dispatcher | `ebook/open.go` | `Open`, `Detect`: Palm `BOOKMOBI`/`TEXtREAd`; zip with `mimetype` (EPUB), with a `.fb2` (FB2), mostly images (CBZ); `<FictionBook` XML; `.txt`/`.html` by extension |
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

### 4.4 Data Model

- `Book.Paged bool` (pre-paginated: comics, fixed-layout EPUB)
- `Book.Format` new names: "EPUB 3", "EPUB 2 (fixed layout)", "Comic archive (CBZ)", "FictionBook 2", "Plain text", "HTML"
- `Resource.Kind` adds `"webp"`
- `htmldoc.Block` adds `FullPage bool`, `PageWidth`, `PageHeight` (points, on page breaks)
- `presets.Preset` adds `SizePreset bool`

### 4.5 API Design

- `ebook.Open(path string) (*Book, error)`: signature unchanged; five callers untouched
- `ebook.Decode(data []byte) (*Book, error)`: every format except HTML
- `engine.DiscoverBooks(dir string, recursive, includeText bool) []string`
- CLI: `--include-text`

## 5. Implementation Phases

Four phases, each changing one layer so it can be reviewed on its own. The
codebase-health refactor on `refactor/duplication` is committed before Phase 1,
since the rename touches every file it changed.

### Phase 1: Rename and default page size
**Goal:** The tool is ebook-to-pdf everywhere, and a fresh install renders on US Letter with a savable default. **Dependencies:** the codebase-health refactor committed. **Covers:** FR-9, FR-11
- [ ] Task 1.1: Module path, `cmd/ebook-to-pdf`, every import
- [ ] Task 1.2: Config/cache folder, `EBOOK_TO_PDF_CONFIG`, temp-file prefix, PDF Creator/Producer
- [ ] Task 1.3: `.goreleaser.yaml`, CI build step, help and splash text
- [ ] Task 1.4: Local skills, `docs/CNAME`, promo page
- [ ] Task 1.5: `SizePreset` on Phone, Paperback, A4; Letter when `layout.page_size` unset
- [ ] Task 1.6: Save as default on the TUI layout screen, writing `layout.page_size`

### Phase 2: Reader seam and renderer hooks
**Goal:** Any reader can plug in, and the renderer can lay out paged books. **Dependencies:** Phase 1. **Covers:** FR-8, FR-10, FR-12
- [ ] Task 2.1: Move MOBI to `mobi.go`; `Open` dispatcher and `Detect`; zip guard
- [ ] Task 2.2: `Book.Paged`; the three markup hooks in `htmldoc`
- [ ] Task 2.3: Per-page sizes and full-page images in `pdfout`; WebP
- [ ] Task 2.4: Engine paged-book rules; `BookExtensions`, folder scan rules, `--include-text`

### Phase 3: EPUB, reflowable and fixed-layout
**Goal:** EPUB 2 and 3 convert, fixed-layout included, and locked books are refused. **Dependencies:** Phase 2. **Covers:** FR-1, FR-2, FR-3
- [ ] Task 3.1: container.xml, OPF, spine, metadata, cover
- [ ] Task 3.2: Markup assembly: image rewriting, SVG unwrapping, CSS gathering, duplicate cover skip
- [ ] Task 3.3: nav/NCX headings
- [ ] Task 3.4: DRM detection
- [ ] Task 3.5: Fixed-layout spine pages (full-page image or sized text page)

### Phase 4: Comics, FictionBook, text, HTML and docs
**Goal:** The remaining formats convert and the docs describe all of them. **Dependencies:** Phase 2 (Tasks 4.5 and 4.6 need every phase). **Covers:** FR-4, FR-5, FR-6, FR-7
- [ ] Task 4.1: CBZ reader
- [ ] Task 4.2: FB2 and `.fb2.zip`
- [ ] Task 4.3: Plain text
- [ ] Task 4.4: HTML
- [ ] Task 4.5: README format table and page size notes, AGENTS.md, CHANGELOG (A5 to Letter under Changed)
- [ ] Task 4.6: Manual checklist for the owner: rename the GitHub repo, add the DNS record, update the git remote

## 6. Risks and Mitigations

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

## 7. Success Criteria

- [ ] A reflowable EPUB 2 and EPUB 3 each convert with spine order, metadata, cover, images and TOC headings as bookmarks
- [ ] A fixed-layout EPUB and a CBZ produce one PDF page per book page, each sized to its image, no cover or title page added
- [ ] A DRM-locked EPUB fails with the DRM message and exit code 1; a font-obfuscated EPUB converts
- [ ] FB2, `.fb2.zip`, TXT and HTML each convert with headings and images where present
- [ ] A folder scan skips `.txt`/`.html` unless `--include-text` is passed
- [ ] A fresh install renders E-reader on Letter; Phone, Paperback and A4 keep their sizes; Save as default persists
- [ ] AZW3/MOBI output unchanged apart from producer name and page size default
- [ ] Binary, module path, config folder and docs say ebook-to-pdf; `make ci` green
- [ ] No new modules in go.mod

## 9. Assumptions

- EPUB embedded fonts are ignored (the renderer picks system fonts), so font obfuscation needs no decoding
- TUI folder batches follow the same rule as CLI scans (skip `.txt`/`.html`); the TUI file picker lists them since picking a file is naming it directly
- The uncommitted codebase-health refactor on `refactor/duplication` lands before Phase 1

## Planning Metrics
<!-- METRICS_JSON {"confidence": 91, "clarification_rounds": 2, "functional_requirements_count": 12, "non_functional_requirements_count": 6, "risk_count": 9, "phase_count": 4, "verification_gaps_found": 1, "confidence_breakdown": {"requirements": 24, "feasibility": 23, "integration": 22, "risk": 22}} -->

confidence: 91
clarification_rounds: 2
functional_requirements_count: 12
non_functional_requirements_count: 6
risk_count: 9
phase_count: 4
verification_gaps_found: 1
