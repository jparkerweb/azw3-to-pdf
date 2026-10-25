# Phase 4: Comics, FictionBook, text, HTML and docs

**Status:** Complete
**Estimated Tasks:** 19

## Overview

Replace the remaining four stubs with real readers: comic archives (one
full-page image per PDF page), FictionBook 2 (plain and zipped), plain text
with conservative chapter detection, and single HTML files with their on-disk
images. Then bring every document up to date: README, AGENTS.md, the promo
page, the local skills and the changelog, and hand the owner the manual steps
the rename still needs.

## Prerequisites

- Phase 2 complete (dispatcher, stubs, `zipsafe.go`, `buildZip`, markup hooks, sized pages) VERIFIED
- Tasks 4.1 to 4.14 may run in parallel with Phase 3; they touch only `internal/ebook/cbz.go`, `fb2.go`, `plaintext.go`, `htmlfile.go`, their tests, and the new `internal/engine/formats_convert_test.go` VERIFIED (Phase 3 is complete, so they run after it)
- Tasks 4.15 to 4.19 start only after Phase 3 is complete, since they describe every format VERIFIED (Phase 3 complete)
- Covers FR-4, FR-5, FR-6, FR-7, and the documentation parts of FR-9 and FR-11

## Tasks

### Comic archives

- [x] **Task 4.1:** Write the comic reader in `internal/ebook/cbz.go`, replacing the stub `decodeCBZ`. Open with `openZip`. Keep file entries whose extension is `.jpg`, `.jpeg`, `.png`, `.gif`, `.webp` or `.bmp`, skipping any path with a segment starting with `.` and anything under `__MACOSX/`. Sort with a `naturalLess(a, b string) bool` helper: compare case-insensitively, splitting runs of digits and comparing them numerically (so `2.jpg` comes before `10.jpg` and `ch1/p10` after `ch1/p9`), with the plain string as tie-breaker. Read each image, keep it only when `sniffImage` recognises it, number from 1 in sorted order, and emit `<html><body><img recindex="N" data-ebook-fullpage="1"></body></html>` per image. Set `Paged = true`, `Format = "Comic archive (CBZ)"`, `Cover` to the first image, `TextBytes` to 0. No usable images fails with "this comic archive contains no images".

- [x] **Task 4.2:** Read `ComicInfo.xml` in `cbz.go` when present (case-insensitive name, at the archive root): `Title` (or `Series` plus ` #Number` when `Title` is empty), `Writer` (comma-separated) into `Authors`, `Summary` into `Description`, `Publisher`, `Year`/`Month` into `Published`, `LanguageISO` into `Language`, `Genre` (comma-separated) into `Subjects`. A malformed `ComicInfo.xml` is ignored, not an error. With no title, `Open` falls back to the file name (Task 2.6).

- [x] **Task 4.3:** Test the comic reader in `internal/ebook/cbz_test.go`: `naturalLess` orders `p1, p2, p10, P11, p100` and nested folders; a fixture with `10.png`, `2.png`, `1.png`, `.hidden.png`, `__MACOSX/._1.png`, `notes.txt` and a `ComicInfo.xml` yields three pages in the order 1, 2, 10, the ComicInfo title and writers, `Paged`, and a cover; a fixture with no images fails with the message above.

### FictionBook

- [x] **Task 4.4:** Start the FB2 reader in `internal/ebook/fb2.go`, replacing the stub `decodeFB2`. Stream the XML with `xml.NewDecoder`, setting `CharsetReader` to look the declared encoding up with `golang.org/x/text/encoding/htmlindex` (covers Windows-1251, KOI8-R, Windows-1252); with no declaration, use UTF-8 when the bytes are valid UTF-8, else Windows-1251. Read `description/title-info`: `book-title` into `Title`; each `author` into `Authors` as "first middle last" (or `nickname`); `annotation` as plain text into `Description`; `date`, `lang`, and each `genre` into `Subjects`; `coverpage/image` `href` (any namespace prefix) as the cover's binary id. Read `publish-info`: `publisher`, `isbn`, and `year` when `date` was empty. Set `Format = "FictionBook 2"` and `Encoding` to the encoding used. A file that is not well-formed XML fails with "this FictionBook file is damaged: <xml error>".

- [x] **Task 4.5:** Convert the FB2 body to HTML in `fb2.go`, escaping all text with `html.EscapeString`. Track `<section>` nesting: a section's `title` becomes `<hN>` with N the nesting depth clamped to 1..6, and its paragraphs joined with `<br>`; each top-level section starts with `<mbp:pagebreak/>` (already a page break in `htmldoc`). Map `p` to `<p>`, `emphasis` to `<em>`, `strong` to `<strong>`, `strikethrough` to `<s>`, `sup`, `sub`, `code`, `subtitle` to `<p><strong>`, `empty-line` to `<br>`, `epigraph` and `cite` to `<blockquote>` (with `text-author` as a `<p>` inside), `poem`/`stanza` to a `<blockquote>` holding each `v` as a line of one `<p>` separated by `<br>`, `table` rows as paragraphs. External `<a href>` links keep their URL; internal (`#...`) links keep only their text. `<image href="#id">` becomes `<img recindex="N">` using Task 4.6's numbering. A second `<body name="notes">` is appended after a page break under an `<h1>Notes</h1>`.

- [x] **Task 4.6:** Load FB2 images in `fb2.go`. Each `<binary id content-type>` is base64 (whitespace removed before decoding); number binaries from 1 in document order, keep those that `sniffImage` recognises, and map `id` to index for `<image>` references, which may come before the binaries since they sit at the end of the file (collect binaries first, or build the HTML with placeholders resolved after the stream ends). `Cover` is the binary named by `coverpage`; a cover image that also appears as the first body image is not repeated.

- [x] **Task 4.7:** Unwrap `.fb2.zip` in `fb2.go`, replacing the stub `decodeFB2Zip`: open with `openZip`, take the first entry ending in `.fb2` (case-insensitive), read it through the zip guard, and call `decodeFB2`. No `.fb2` entry fails with "this zip archive does not contain a .fb2 book".

- [x] **Task 4.8:** Test FB2 in `internal/ebook/fb2_test.go`: a UTF-8 book with metadata, nested sections (heading levels 1 and 2), emphasis, a poem, an epigraph, a cover and one body image as base64 binaries placed after the body, and a notes body; assert metadata, cover, `recindex` on the body image, and that `htmldoc.Parse` finds the headings at the right levels and the page break between top-level sections. A Windows-1251 book (encode the fixture text with `charmap.Windows1251`) reads back as the original Cyrillic. The same UTF-8 book zipped as `book.fb2.zip` decodes identically through `Decode`. Malformed XML fails with the damaged message.

### Plain text

- [x] **Task 4.9:** Decode text in `internal/ebook/plaintext.go`, replacing the stub `decodePlainText`. Strip a UTF-8 BOM; use the bytes as UTF-8 when `utf8.Valid`, otherwise decode with `charmap.Windows1252`, and record which in `Encoding`. Normalise `\r\n` and `\r` to `\n`. When a Project Gutenberg marker is present (`*** START OF THE PROJECT GUTENBERG EBOOK` or `*** START OF THIS PROJECT GUTENBERG EBOOK`, case-insensitive, up to the end of that line), take `Title:` and `Author:` lines from the header before it, and keep only the text between it and the matching `*** END OF` line, dropping the licence. Set `Format = "Plain text"`.

- [x] **Task 4.10:** Build paragraphs and headings in `plaintext.go`. Split on one or more blank lines; inside a paragraph join hard-wrapped lines with a space, escaping every paragraph with `html.EscapeString`. A paragraph is a heading only when it is a single line of at most 60 characters, does not end with `.`, `,`, `;` or `:` (except a heading made only of a Roman numeral, such as `IV.`), and matches, case-insensitively, `^(chapter|book|part|volume|prologue|epilogue|introduction|preface|foreword|afterword|appendix)\b` followed by nothing, a number, a Roman numeral or a number word up to `twenty`, optionally followed by a short title after `.`, `:` or a dash; or is a lone Roman numeral of at most seven characters other than `I` alone. Headings become `<mbp:pagebreak/><h2>...</h2>`, other paragraphs `<p>...</p>`. A file with no text after trimming fails with "this text file is empty".

- [x] **Task 4.11:** Test plain text in `internal/ebook/plaintext_test.go`: a Gutenberg-style file (header with Title and Author, START and END markers, licence after) gives the title, author and only the body; Windows-1252 bytes (`0x93`/`0x94` quotes) decode to curly quotes; headings are found for `CHAPTER I`, `Chapter 12: The Return`, `Part Two`, `XIV`, `Prologue`; and not for `Chapter and verse were quoted at length.`, `I`, `I said nothing.`, `Book me a table for two, please`, or a 90-character line starting with "Chapter". Hard-wrapped lines join into one paragraph.

### HTML

- [x] **Task 4.12:** Write the HTML reader in `internal/ebook/htmlfile.go`, replacing the stub `decodeHTMLFile(path, data)`. Decode with `charset.DetermineEncoding(data, "text/html")` from `golang.org/x/net/html/charset` and record the encoding name. Parse with `html.Parse`; take `<title>` as `Title`, `<meta name="author">` (and `dc.creator`) as `Authors`, `description`, `keywords` (as `Subjects`), and `<html lang>` as `Language`. For each `<img src>`: decode `data:image/...;base64,` URIs in place; ignore `http:`, `https:` and protocol-relative URLs (no network, NFR-4); otherwise URL-unescape, join to the file's folder, `filepath.Clean`, and keep it only when `filepath.Rel(folder, target)` does not start with `..` and the target is a regular file under 64 MiB, then read it, `sniffImage` it, and rewrite to `recindex`. Gather `<style>` text and local `<link rel="stylesheet">` files (same folder-tree rule) into `Flows`. Remove `<script>`, render back with `html.Render` into `Book.HTML`, `Format = "HTML"`.

- [x] **Task 4.13:** Test the HTML reader in `internal/ebook/htmlfile_test.go` with files written to `t.TempDir()`: `book/index.html` referencing `img/a.png` (kept), `../outside.png` (dropped though it exists), `https://example.com/x.png` (dropped), a `data:` PNG (kept) and a missing file (dropped); a `<meta charset="windows-1252">` page decodes its quotes; title, author and stylesheet are picked up; through `Open`, the title falls back to the file name when `<title>` is missing.

### End to end

- [x] **Task 4.14:** Test conversions in a new `internal/engine/formats_convert_test.go`: build a CBZ of three PNGs (generated with `image/png`, sizes 400 x 600, 600 x 400 and 400 x 600 pixels), an FB2, a TXT and an HTML file in `t.TempDir()`, run `engine.Convert` on each with default options, and assert: the CBZ gives exactly three pages with no dropped images (no cover or title page); the FB2 and TXT give `Headings >= 2`; the HTML keeps its local image. Conversion speed (NFR-5) is not a unit test: compare an AZW3 and an EPUB conversion time with the `ebook-to-pdf-verify-output` skill on real books.
  <!-- VERIFICATION: Added (NFR-5 had no task in the plan draft) -->

### Documentation

- [x] **Task 4.15:** Update `README.md`: the opening paragraph and a format table (format, extensions, what converts, limits: EPUB 2/3 reflowable and fixed-layout, Kindle, CBZ, FB2 and `.fb2.zip`, TXT, HTML); the DRM policy (locked EPUB and Kindle books are refused, buy or download DRM-free copies); that fixed-layout text pages are reflowed, not positioned exactly, and mixed books keep the last fixed size; comics and fixed-layout books get one page per book page with no added cover or title page; folder scans and `--include-text`; the page size section (US Letter default, `layout.page_size`, size presets keep their own page, Save as default with `s` on the layout screen, `--page-size` for one run); the presets table's page size column now saying "default (Letter)" for E-reader, Compact, Print, Large print and Manuscript.

- [x] **Task 4.16:** Update `AGENTS.md`: "What this is" covers every format; the architecture diagram shows the dispatcher and readers; a new `internal/ebook` subsection per reader with its gotchas (zip guard is the only way into archives, `<title/>` expansion in EPUB, TOC targets marked with `data-ebook-heading`, font obfuscation is not DRM, FB2 binaries come after the body, natural sort in CBZ, conservative TXT headings); the three `htmldoc` hooks listed as the only format-specific markup the parser knows; renderer per-page sizes and full-page images; "Adding things" gains "A format: a reader file in `internal/ebook`, a `Format` constant and `Detect` rule in `open.go`, extensions in `engine.BookExtensions`, a README row"; the default page size rule and `SizePreset`.

- [x] **Task 4.17:** Update the promo page and local skills. In `docs/index.html` change the headline, description and `og:` text from Kindle-only to ebooks, list the formats, and use EPUB examples alongside AZW3 in the terminal snippets. In `.claude/skills/ebook-to-pdf-verify-output/SKILL.md` add how to check EPUB, fixed-layout EPUB and CBZ output (one page per book page, no cover page). In `.claude/skills/ebook-to-pdf-add-preset/SKILL.md` document `SizePreset` and that non-size presets take the saved default page size. In `.claude/skills/ebook-to-pdf-update-docs/SKILL.md` add the format table to what it audits.

- [x] **Task 4.18:** Add the changelog entry to `CHANGELOG.md` under `## [Unreleased]` (Keep a Changelog): **Added** EPUB 2 and 3 (reflowable and fixed-layout), CBZ, FB2 and `.fb2.zip`, TXT and HTML input; content-based format detection; `--include-text`; WebP images; Save as default on the layout screen. **Changed** the project, binary, module path and config folder are now `ebook-to-pdf` (`EBOOK_TO_PDF_CONFIG`, `EBOOK_TO_PDF_LOG_LEVEL`), old settings are not migrated; the default page size is US Letter (E-reader and Compact were A5, Large print was A4), and how to get A5 back (choose A5 on the layout screen and press `s`, or set `layout.page_size: a5`); the promo page moved to `ebook-to-pdf.equilllabs.com`. Use no em dashes.

- [x] **Task 4.19:** Hand the owner the manual steps, as a checklist in this phase's completion summary and in the final message: rename the GitHub repository `jparkerweb/azw3-to-pdf` to `jparkerweb/ebook-to-pdf` (GitHub redirects old URLs) before the next tag; run `git remote set-url origin https://github.com/jparkerweb/ebook-to-pdf.git`; add a DNS `CNAME` record for `ebook-to-pdf.equilllabs.com` pointing at `jparkerweb.github.io` and set the custom domain in the repository's Pages settings; check that `go install github.com/jparkerweb/ebook-to-pdf/cmd/ebook-to-pdf@latest` works after the first tag. Run `make ci` one last time.

## Acceptance Criteria

- A CBZ produces one PDF page per image in natural order, each sized to its image, with no cover or title page
- FB2, `.fb2.zip`, TXT and HTML each convert with headings and images where present; text encodings are honoured
- HTML images outside the file's folder tree and remote images are never read
- README, AGENTS.md, the promo page, the local skills and CHANGELOG describe every format, the rename and the Letter default
- `make ci` passes and `go.mod` gains no modules
- The owner has the manual checklist

## Notes

- Tasks 4.1 to 4.14 must not edit `open.go`, `zipsafe.go` or Phase 3's files; if a reader needs a dispatcher change, record it as a plan change instead.
- The Gutenberg trim keeps the book usable for the most common source of free text files; everything else in the file is kept as written.
- Before release, run the `ebook-to-pdf-verify-output` skill on one real file of each new format the owner supplies.

## Phase Completion Summary

**Date:** 2026-10-25
**Implementer:** Claude (Implement + Review), with two helpers for the FictionBook and HTML readers

**What was done:** The four remaining stubs are real readers. `cbz.go` sorts images naturally, skips dot files and `__MACOSX/`, emits one full-page `<img>` per image and reads `ComicInfo.xml`. `fb2.go` reads FictionBook 2 with its declared encoding (else UTF-8, else Windows-1251), turns sections, poems, epigraphs, tables and notes into HTML, resolves base64 binaries after the stream ends, and unwraps `.fb2.zip`. `plaintext.go` decodes UTF-8 or Windows-1252, trims Project Gutenberg headers and licences, joins hard-wrapped lines and finds headings with conservative patterns. `htmlfile.go` detects the charset, reads metadata, keeps only images and stylesheets inside the file's folder tree (never remote), decodes `data:` images and drops scripts. README, AGENTS.md, the promo page, three local skills and the changelog now describe every format, the rename and the Letter default.

**Files created:** `internal/ebook/cbz_test.go`, `internal/ebook/fb2_test.go`, `internal/ebook/plaintext_test.go`, `internal/ebook/htmlfile_test.go`, `internal/engine/formats_convert_test.go`

**Files modified:** `internal/ebook/cbz.go`, `fb2.go`, `plaintext.go`, `htmlfile.go` (stubs replaced), `internal/ebook/open_test.go`, `internal/pdfout/render_blocks.go`, `README.md`, `AGENTS.md`, `CHANGELOG.md`, `docs/index.html`, `.claude/skills/ebook-to-pdf-verify-output/SKILL.md`, `.claude/skills/ebook-to-pdf-add-preset/SKILL.md`, `.claude/skills/ebook-to-pdf-update-docs/SKILL.md`

**Tests:** `go test ./...` all pass; `golangci-lint run ./...` 0 issues; gofmt clean; the binary builds and runs `version`. `make` is not installed on this machine, so the `ci` steps were run one by one, and `-race` was not run (it needs cgo on Windows; CI runs it). `go.mod` gained no modules.

**Issues and decisions:**

> BUG: a three-image CBZ rendered five pages. Each image's `<html>` boundary is an unsized page break, and after a full-page image (which leaves the cursor at the page bottom) the renderer opened a blank page for it. `render_blocks.go` now skips an unsized break when the page has no room left, since whatever follows starts a new page anyway. Regression test: `TestConvertCBZ` in `internal/engine/formats_convert_test.go`.

> SPEC NOTE: `open_test.go` and `render_blocks.go` are outside this phase's file list. `TestOpenReachesFormatReaders` asserted the stubs' "not finished" errors, so it now checks that each file reaches the right reader (by `Format`); the renderer change is the bug fix above.

> SPEC NOTE (FB2): the notes body's own `<title>` is replaced by the added `<h1>Notes</h1>`, its sections sit one level deeper, with no page break per note; other unnamed extra bodies are written like the main body; poem and stanza titles are bold paragraphs, not headings, so the outline stays clean; table cells in a row are joined by a space; binaries that do not decode or sniff as images get no number; unknown elements keep their text.

> SPEC NOTE (HTML): local files are checked with `Lstat`, so a symbolic link cannot lead outside the folder; whitespace inside a `data:` URI is ignored; no cover is set and `TextBytes` is left at 0, as the task asks for neither.

> SPEC NOTE (TXT): the lone Roman numeral rule accepts only well-formed numerals (so `DID` is not one), case-insensitively.

**Review (Implement + Review):** a fresh helper reviewed the 18 changed files and found 0 critical, 3 warnings and 0 suggestions, all confirmed. All 3 were fixed: HTML references are now resolved with `filepath.EvalSymlinks` and checked for containment again, so a linked folder inside the tree cannot lead outside it (checked by hand with a Windows junction; `TestHTMLFileSymlinkedFolderStaysInFolder` runs where symlinks can be made, such as CI); HTML books set `TextBytes`; `.xhtml` joins `engine.BookExtensions` and `TextExtensions`, so scans and the picker accept it as the README says, and HTML pages now get `expandSelfClosing` so an `.xhtml` page's `<title/>` no longer swallows it (`TestHTMLFileXHTMLShortTags`). The changelog's HTML line lists `.xhtml`. Tests, lint and build pass again.

**Files modified by the review fixes:** `internal/ebook/htmlfile.go`, `internal/ebook/htmlfile_test.go`, `internal/engine/output.go`, `CHANGELOG.md`

**Manual steps for the owner (Task 4.19):**

- [ ] Rename the GitHub repository `jparkerweb/azw3-to-pdf` to `jparkerweb/ebook-to-pdf` before the next tag (GitHub redirects the old URLs)
- [ ] Run `git remote set-url origin https://github.com/jparkerweb/ebook-to-pdf.git`
- [ ] Add a DNS `CNAME` record for `ebook-to-pdf.equilllabs.com` pointing at `jparkerweb.github.io`, and set that custom domain in the repository's Pages settings
- [ ] After the first tag, check that `go install github.com/jparkerweb/ebook-to-pdf/cmd/ebook-to-pdf@latest` works
- [ ] Before release, run the `ebook-to-pdf-verify-output` skill on one real file of each new format (EPUB, fixed-layout EPUB, CBZ, FB2, TXT, HTML)
