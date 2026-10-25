# Phase 2: Reader seam and renderer hooks

**Status:** Complete
**Estimated Tasks:** 19

## Overview

Make room for new formats without adding any yet. `ebook.Open` becomes a
dispatcher that sniffs the file and hands it to a reader (MOBI moves into its
own file, the other readers start as stubs), archives get a safe zip layer,
`htmldoc` gains three small markup hooks, and the renderer learns per-page sizes
and full-page images. The engine, CLI and TUI learn the new extensions and the
folder-scan rule for text and HTML files.

## Prerequisites

- Phase 1 complete (module path is `github.com/jparkerweb/ebook-to-pdf`)
- Covers FR-8, FR-10, FR-12 and the shared parts of NFR-1 to NFR-4

## Tasks

### Book and the MOBI reader

- [x] **Task 2.1:** Extend the shared types in `internal/ebook/book.go`. Add `Paged bool` to `Book` with a comment: the book is pre-paginated (comics, fixed-layout EPUB), so each source page becomes exactly one PDF page and no cover or title page is added. Update the `Format` comment with the new names the readers will use ("EPUB 3", "EPUB 2 (fixed layout)", "Comic archive (CBZ)", "FictionBook 2", "Plain text", "HTML"). Add `"webp"` to the `Resource.Kind` comment, and teach `sniffImage` to return `"webp"` for `RIFF....WEBP` (bytes 0-3 `RIFF`, bytes 8-11 `WEBP`). Rewrite the package comment: the package reads ebooks of several formats into one `Book`. Keep `sniffImage`, `AuthorLine` and the types in `book.go`; they are shared by every reader.
  - Test: extend `TestSniffImage` in `internal/ebook/ebook_test.go` with a WebP header.

- [x] **Task 2.2:** Move the MOBI/KF8 reader into `internal/ebook/mobi.go` unchanged in behaviour. Rename today's `Decode` body to `decodeMOBI(data []byte) (*Book, error)` and move it there with `applyMetadata`, `splitFlows`, `loadResources`, `findKF8Boundary`, `formatName`, `compressionName`, `encodingName` and `localeName`. Leave `book.go` with the types, `IsImage`, `sniffImage` and `AuthorLine`. Existing tests (`TestDecodeMinimalBook`, `TestDecodeRejectsNonKindleFiles`) must still pass once Task 2.6 routes `Decode` through the dispatcher.

### Safe archives

- [x] **Task 2.3:** Create `internal/ebook/zipsafe.go`, the only way readers touch zip archives. `openZip(data []byte) (*zipArchive, error)` wraps `zip.NewReader(bytes.NewReader(data), int64(len(data)))`; a read failure returns `errBrokenArchive` wrapped with the cause ("the archive is damaged and cannot be read: %v"). Index entries by cleaned name: convert backslashes to `/`, `path.Clean`, drop a leading `/`, and skip any entry whose cleaned name is `..` or starts with `../` (never extract anything to disk). Methods: `names() []string` (files only, archive order), `has(name string) bool`, `read(name string) ([]byte, error)`, and `resolve(base, href string) string` that URL-unescapes `href`, strips `#fragment` and `?query`, joins it to `path.Dir(base)` and cleans it. `read` falls back to a case-insensitive match when the exact name is missing.

- [x] **Task 2.4:** Enforce the zip limits in `zipArchive.read` (NFR-4). Constants in `zipsafe.go`: `maxEntryBytes = 256 << 20`, `maxTotalBytes = 1 << 30`, `maxRatio = 200`. Read through `io.LimitReader(rc, maxEntryBytes+1)` and refuse when the limit is hit, never trusting the header's sizes. Keep a running total of bytes read across the archive and refuse past `maxTotalBytes` with "the book is larger than 1 GiB once unpacked". Refuse an entry over 1 MiB whose uncompressed size is more than `maxRatio` times its compressed size ("looks like a zip bomb"). Every refusal names the entry.
  - Test: create `internal/ebook/zipfixture_test.go` with `buildZip(t *testing.T, entries ...zipEntry) []byte` (`zipEntry{Name string; Data []byte; Store bool}`, written with `archive/zip`; `Store` uses `zip.Store`, the default `zip.Deflate`). Every later reader test builds its fixtures with it. In `internal/ebook/zipsafe_test.go` cover: `../evil.txt` and `/abs.txt` are not listed and cannot be read, a backslash name resolves, `resolve("OEBPS/text/ch1.xhtml", "../images/a%20b.png#x")` gives `OEBPS/images/a b.png`, a highly compressible 2 MiB entry is refused by the ratio cap, and a damaged archive returns `errBrokenArchive`.

### The dispatcher

- [x] **Task 2.5:** Create `internal/ebook/open.go` with a `Format int` type and constants `FormatUnknown`, `FormatMOBI`, `FormatEPUB`, `FormatCBZ`, `FormatFB2`, `FormatFB2Zip`, `FormatText`, `FormatHTML`, plus `func (f Format) String() string` for messages ("EPUB", "Kindle (MOBI/AZW3)", ...). Implement `Detect(name string, data []byte) Format`, content first, then extension:
  1. `len(data) >= 68` and `data[60:68]` is `BOOKMOBI` or `TEXtREAd`: MOBI.
  2. Starts with `PK\x03\x04`: open with `openZip`; a `mimetype` entry containing `application/epub+zip`, or a `META-INF/container.xml` entry: EPUB; else any entry ending in `.fb2`: FB2Zip; else at least one image entry (by extension `.jpg .jpeg .png .gif .webp .bmp`) and images make up at least half of the file entries that are not `ComicInfo.xml`, dotfiles or under `__MACOSX/`: CBZ; else Unknown.
  3. After skipping a UTF-8 BOM and whitespace, the first 4 KiB contain `<FictionBook`: FB2.
  4. By the lower-cased name: `.txt` is Text; `.html`, `.htm`, `.xhtml` are HTML.
  5. Otherwise Unknown.

  Content wins over the name, so a MOBI file named `.epub` is read as MOBI.

- [x] **Task 2.6:** Route everything through the dispatcher in `open.go`. `Open(path)` keeps its signature: read the file, `Detect(path, data)`, call `decodeHTMLFile(path, data)` for HTML and `decodeAs(format, data)` otherwise, then set `Path`, `FileSize`, and when `Title` is empty set it from the file name without its extension (and without `.fb2` for `.fb2.zip`). `Decode(data)` keeps its signature for every format except HTML: `Detect("", data)`, and when that is Unknown but the first 4 KiB hold no NUL byte, decode as plain text; an empty `Title` becomes "Untitled". Unknown formats return `ErrUnsupported` wrapped as "not a supported ebook (expected EPUB, Kindle, CBZ, FB2, TXT or HTML)"; `Open` keeps prefixing errors with the path, so every message names the file (FR-10). Create stub readers, one per file so Phases 3 and 4 each replace only their own: `epub.go` (`decodeEPUB`), `cbz.go` (`decodeCBZ`), `fb2.go` (`decodeFB2`, `decodeFB2Zip`), `plaintext.go` (`decodePlainText`), `htmlfile.go` (`decodeHTMLFile(path string, data []byte)`), each returning `fmt.Errorf("%s support is not finished yet", name)`. Add `DetectFile(path string) (Format, error)` for the picker and CLI: read the first 4 KiB for the Palm, FB2 and name checks, and use `zip.OpenReader` (central directory only, no full read) for zip files.

- [x] **Task 2.7:** Test the dispatcher in `internal/ebook/open_test.go`: the minimal MOBI from `buildMinimalBook` is detected as MOBI even when named `book.epub`; a zip with `mimetype` is EPUB; a zip with `book.fb2` is FB2Zip; a zip of `01.jpg`, `02.png`, `ComicInfo.xml` is CBZ; a zip of `notes.txt` and `a.doc` is Unknown; `<?xml ...?><FictionBook` is FB2; `x.txt` and `x.HTM` by name; random bytes give an error that contains the file name and "not a supported ebook"; a truncated zip gives the broken-archive message with the file name. `DetectFile` agrees with `Detect` for files written to `t.TempDir()`.

### Markup hooks

- [x] **Task 2.8:** Add the block fields and the heading hook in `internal/htmldoc`. In `doc.go` add to `Block`: `FullPage bool` (an image that fills its own page), and `PageWidth, PageHeight float64` (points; set only on `KindPageBreak` blocks that start a page of a given size). In `parser.start` (`parse.go`) read `data-ebook-heading`: when it parses to an integer, clamp it to 1..6 and make the element a `KindHeading` block of that level, overriding `blockKind` and promoting inline elements (a `<span>` or `<a>` target) to a block the way `props.Block` does today. This is how readers mark table-of-contents targets regardless of the book's CSS (NFR-3: no other parsing changes).

- [x] **Task 2.9:** Add the page hooks in `parse.go`. In `parser.image`, set `FullPage: true` when the `<img>` carries `data-ebook-fullpage`. In `parser.start`, before the namespace prefix is stripped (next to the `mbp:pagebreak` case), handle `ebook:page`: read `width` and `height` as points; when both are positive, add a sized break: flush, and if the last block is already a `KindPageBreak` (for example from an `<html>` boundary) set its `PageWidth`/`PageHeight` instead of appending another; otherwise append `Block{Kind: KindPageBreak, PageWidth: w, PageHeight: h}`. Unlike `emitBreak`, a sized break is added even at the very start of the document. `ebook:page` has no content, so treat it as void (`isVoid`).
  - Test in `internal/htmldoc/htmldoc_test.go`: `<span data-ebook-heading="2">Ch</span>` gives a level-2 heading; `data-ebook-heading="9"` clamps to 6; `<img recindex="3" data-ebook-fullpage>` gives `FullPage`; `<html><body>a</body></html><html><body><ebook:page width="600" height="800"/>b</body></html>` gives exactly one break, carrying 600 x 800; a document starting with `<ebook:page>` starts with a sized break; markup without the hooks parses exactly as before (existing tests unchanged).

### Renderer

- [x] **Task 2.10:** Give the renderer a current page geometry in `internal/pdfout`. Add `pageW, pageH float64` to `renderer` and a `setGeometry(w, h float64)` method in `render_page.go` that sets them and recomputes `left`, `right`, `top`, `bottom`, `contentWidth` and `contentHeight` from `opts.Margins` (move that arithmetic out of `Render`, which now calls `setGeometry(opts.PageSize.Width, opts.PageSize.Height)`). `newPage` adds the page with `r.pdf.AddPageWithOption(gopdf.PageOption{PageSize: &gopdf.Rect{W: r.pageW, H: r.pageH}})` so every page records its own size. `drawFurniture` already reads `left`, `top`, `bottom` and `contentWidth`, so headers and page numbers follow the page. Margins are the configured ones on every page size.

- [x] **Task 2.11:** Honour sized breaks in `renderer.block` (`render_blocks.go`). A `KindPageBreak` with `PageWidth > 0` always starts a new page of that size, whatever `ChapterBreaks` says: if the current page exists, is unused and already has that size, keep it; otherwise `setGeometry(w, h)` and `newPage()`. Pages that text overflows onto keep the size of the page they continue. A plain break behaves as today.

- [x] **Task 2.12:** Render full-page images in `render_image.go`. When `b.FullPage` is set, `image` calls a new `fullPageImage(b)`: if the current page was opened by a sized break and is still unused, draw the image scaled to fit that whole page (no margins), centred; otherwise add a page sized to the image's natural size at 0.75 pt per pixel and draw it edge to edge. Full-page image pages get no running header and no page number (add them with a `newBarePage` that skips `drawFurniture`), but still count in `r.page`. After drawing, mark the page used and full (`r.y = r.bottom`) so the next block starts a new page, and restore the geometry that was current before a page sized to the image, so a following text page keeps the book's size. Count the image in `result.Images`; a decode failure counts in `result.Dropped` as today.

- [x] **Task 2.13:** Decode WebP in `render_image.go`. Import `golang.org/x/image/webp` (already in `go.mod` through `golang.org/x/image`). Add `"webp"` cases to `decodeConfig` and `decodeImage`; `place` already re-embeds anything that is not JPEG or PNG as PNG. Update the `Image.Kind` comment in `render.go`.

- [x] **Task 2.14:** Test the renderer in `internal/pdfout/pdfout_test.go`. Render a `Doc` with a text paragraph, a 600 x 800 sized break and text, and a `FullPage` image of a 300 x 500 pixel PNG generated with `image/png`; assert the PDF holds three pages and `MediaBox` entries for the base size, 600 x 800 and 225 x 375. Assert page numbers are drawn on text pages and not on the full-page image page (render with `PageNumbers: true` and count the page-number text, or check the page's content stream is image-only). Decode a hard-coded 1 x 1 lossless WebP byte slice through `decodeConfig` and `decodeImage`.

### Engine, CLI and TUI

- [x] **Task 2.15:** Apply the paged-book rules in `engine.Convert` (`internal/engine/convert.go`). Add `func pdfOptionsFor(book *ebook.Book, opts pdfout.Options) pdfout.Options` that, for `book.Paged`, sets `Cover = false` and `TitlePage = false` (the book's own first page is its cover), and use it for the options passed to `pdfout.Render`. Leave every other option alone.
  - Test in `internal/engine/convert_test.go`: a paged book turns cover and title page off; an unpaged one keeps them.

- [x] **Task 2.16:** Update the extension rules in `internal/engine/output.go`. `BookExtensions` becomes `.azw3 .azw .mobi .prc .epub .cbz .fb2 .fb2.zip .txt .html .htm`; add `TextExtensions = []string{".txt", ".html", ".htm"}`. `IsBookFile` matches by lower-cased suffix (so `.fb2.zip` works while a plain `.zip` does not). Add `IsTextFile(path) bool`. Change `DiscoverBooks` to `DiscoverBooks(dir string, recursive, includeText bool) []string`, skipping text files unless `includeText` (FR-12). Add `IsConvertible(path string) bool`: `IsBookFile(path)` or `ebook.DetectFile(path)` gives a known format, for files named directly. Update the comments that say "Kindle".
  - Test: update `TestIsBookFile` and `TestDiscoverBooks` in `internal/engine/engine_test.go` (a folder with `a.epub`, `b.txt`, `c.html`, `d.fb2.zip`, `e.zip` gives 2 books without text and 4 with it); `IsConvertible` is true for a MOBI file renamed to `book.bin`.

- [x] **Task 2.17:** Wire `--include-text` through the CLI. In `internal/cli/flags.go` add `flagIncludeText` and register `--include-text` ("also convert .txt and .html files found when scanning folders"). In `collectInputs` (`internal/cli/run.go`) call `engine.DiscoverBooks(p, flagRecursive, flagIncludeText)`. When a folder yields nothing, say "no ebooks found in %s", adding "; it has .txt or .html files, add --include-text to convert them" when that is why. For a file named directly use `engine.IsConvertible`, and on failure say "%s is not a supported ebook (expected one of: %s)". In `completeBookFiles` (`completion.go`) offer each extension's last part once (`.fb2.zip` contributes `zip`), and update its comment.
  - Test in `internal/cli/flags_test.go` (or a new `run_test.go`): a folder with only `notes.txt` fails with the `--include-text` hint, and succeeds with the flag set; a directly named `notes.txt` is accepted without it.

- [x] **Task 2.18:** Update the CLI's format wording. `root.go` `Short` becomes "Turn ebooks into PDFs." and `Long` lists the formats: EPUB (.epub), Kindle (.azw3, .azw, .mobi, .prc), comic archives (.cbz), FictionBook (.fb2, .fb2.zip), plain text (.txt) and HTML (.html, .htm), noting that folder scans skip text and HTML without `--include-text`. `probe.go`: `Short` "Inspect an ebook without converting it"; add a `row("Layout", "fixed pages")` when `book.Paged`. Replace "Kindle book" with "ebook" in every remaining CLI message.

- [x] **Task 2.19:** Update the TUI's picking rules. `NewFilePickerModel` (`internal/tui/screens/filepicker.go`) keeps `AllowedTypes = engine.BookExtensions`, so text and HTML files can be picked directly (the picker matches by suffix, so `.fb2.zip` works). The space toggle uses `engine.IsConvertible`. The `a` key (add every book in the folder) and the typed-folder path both call `DiscoverBooks(..., false)` for text, matching the CLI scan rule; typed files use `IsConvertible`. Replace "is not a Kindle book" and "No Kindle books in %s" with "is not a supported ebook" and "No ebooks in %s". On the info screen (`internal/tui/screens/info.go`) skip rows whose value is empty (Compression and Encoding are blank for most new formats) and add a "Layout: fixed pages" row when `Paged`. Splash subtitle (`splash.go`) becomes "Ebooks, laid out as PDFs.". Update `internal/tui/app_test.go` and `internal/tui/screens/batch_test.go` for the new strings and the `DiscoverBooks` signature.

## Acceptance Criteria

- `make ci` passes; MOBI/AZW3 conversion produces the same blocks as before (existing ebook, htmldoc and engine tests unchanged apart from signatures and strings)
- `ebook.Open` on an `.epub`, `.cbz`, `.fb2`, `.txt` or `.html` file reaches that format's stub and returns its "not finished yet" error naming the file
- Unknown files and damaged archives fail with a message that names the file and says what to do
- Zip entries are never written to disk; the size, ratio and path rules have tests
- The renderer writes pages of different sizes in one PDF, and full-page images have no margins or furniture
- `--include-text` controls whether folder scans pick up `.txt`/`.html`; naming such a file always works, in the CLI and the TUI
- `go.mod` gains no modules; `htmldoc` changed only by the three hooks and the new `Block` fields

## Notes

- The stubs exist so Phases 3 and 4 can run in parallel: each replaces only its own reader file. Neither should edit `open.go`; if a reader needs a dispatcher change, record it as a plan change.
- A book that mixes fixed-layout and reflowable spine items keeps the last fixed page size for the reflowable pages that follow; acceptable for now and worth a line in the README (Task 4.15).
- TOC targets on full-page image pages produce no bookmark, since there is no text block to attach one to.

## Phase Completion Summary

**Date:** 2026-10-25
**Implementer:** Claude (Implement + Review), with four helpers for the htmldoc, pdfout, CLI and TUI tasks

**What was done:** `ebook.Open` and `Decode` now go through a dispatcher (`open.go`) that sniffs content first and the name second. The MOBI reader moved unchanged into `mobi.go`, and the five new formats have stub readers that return "not finished yet". `zipsafe.go` is the only way into zip files and enforces the path, size and ratio rules. `htmldoc` gained the three hooks. The renderer handles per-page sizes, sized breaks, full-page images without furniture, and WebP. The engine, CLI and TUI know the new extensions and the `--include-text` folder-scan rule.

**Files created:** `internal/ebook/open.go`, `mobi.go`, `zipsafe.go`, `epub.go`, `cbz.go`, `fb2.go`, `plaintext.go`, `htmlfile.go`, `open_test.go`, `zipsafe_test.go`, `zipfixture_test.go`, `internal/cli/run_test.go`

**Files modified:** `internal/ebook/book.go`, `ebook_test.go`; `internal/htmldoc/doc.go`, `parse.go`, `htmldoc_test.go`; `internal/pdfout/render.go`, `render_page.go`, `render_blocks.go`, `render_image.go`, `pdfout_test.go`; `internal/engine/convert.go`, `output.go`, `convert_test.go`, `engine_test.go`; `internal/cli/flags.go`, `run.go`, `completion.go`, `root.go`, `probe.go`; `internal/tui/screens/filepicker.go`, `info.go`, `splash.go`, `batch_test.go`; `internal/tui/app_test.go`

**Tests:** `go test ./...` passes, `go vet ./...` and `golangci-lint run ./...` are clean, the binary builds and runs `version`. `-race` was not run locally (no cgo on this machine); CI runs it.

**Issues and notes:**
> BUG: a full-page image on a page of its own size restored the book's geometry after setting the cursor, so following text could land on the small image page. Fixed by restoring first. Regression test: `TestTextAfterFullPageImage` in `internal/pdfout/pdfout_test.go`.
> SPEC NOTE: zip entries with an absolute name (`/abs.txt`, `C:...`) are dropped rather than having the slash stripped, as Task 2.4's test requires; the leading-slash strip applies to lookups and hrefs.
> SPEC NOTE: a sized break holds back its page's header and page number until ordinary content is drawn, so a full-page image that fills the page carries no furniture.
> SPEC NOTE: `TestDecodeRejectsNonKindleFiles` now expects "not a supported ebook" (string change only).
> SPEC NOTE: the splash screen's "reads .azw3, .azw, .mobi and .prc" line was left as is (not in Task 2.19); worth updating with the docs in Task 4.15.
> REVIEW: an independent review found 3 issues, all fixed: `inf`/`NaN` page sizes are now rejected in `pointsAttr` (test in `TestParseSizedPageHook`); the zip size caps are overridable variables with tests (`TestZipRefusesOversizedEntry`, `TestZipRefusesOversizedBook`); and a zip read is now bounded by the ratio cap up front, so a zip bomb is refused after about 1 MiB rather than 256 MiB.
> SPEC NOTE: an output name for `book.fb2.zip` is `book.fb2.pdf` (`ResolveOutput` strips one extension); not in this phase's tasks.
