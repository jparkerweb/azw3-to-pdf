# Phase 3: EPUB, reflowable and fixed-layout

**Status:** Complete
**Estimated Tasks:** 18

## Overview

Replace the EPUB stub with a real reader. Reflowable EPUB 2 and 3 books become
one `Book`: spine documents concatenated in reading order, images renumbered to
`recindex`, the book's CSS gathered into `Flows`, table-of-contents targets
marked as headings, and the cover found from the package file. Fixed-layout
books become one sized PDF page per spine item. DRM-locked books are refused
with a clear message; font obfuscation is not DRM and converts.

## Prerequisites

- Phase 2 complete: `ebook/open.go` dispatches EPUB to `decodeEPUB`, `zipsafe.go` and `buildZip` exist, `htmldoc` understands `data-ebook-heading`, `data-ebook-fullpage` and `<ebook:page>`, the renderer handles sized pages (VERIFIED)
- Covers FR-1, FR-2, FR-3
- May run in parallel with Phase 4 (see overview); touch only `internal/ebook/epub*.go`, their tests, and the new `internal/engine/epub_convert_test.go`

## Tasks

### Package file and resources

- [x] **Task 3.1:** Read the package in `internal/ebook/epub.go`. Read `META-INF/container.xml` and take the first `rootfile` whose `media-type` is `application/oebps-package+xml` (or the first `rootfile` at all); a missing file or rootfile fails with "this EPUB has no package file (META-INF/container.xml is missing or empty)". Parse the OPF with `encoding/xml` into small structs: `package@version`, `metadata` (`dc:title`, `dc:creator`, `dc:publisher`, `dc:description`, `dc:identifier` with `opf:scheme`, `dc:date`, `dc:rights`, `dc:language`, `dc:subject`, and `meta` both as `name`/`content` and as `property` with text), `manifest/item` (`id`, `href`, `media-type`, `properties`), `spine` (`toc` attribute, `itemref` with `idref`, `linear`, `properties`) and `guide/reference` (`type`, `href`). Resolve every `href` against the OPF's folder with `zipArchive.resolve`. Match XML names on their local part so prefixed and unprefixed books both parse.

- [x] **Task 3.2:** Fill the `Book` metadata from the OPF in `epub.go`: `Title` from the first `dc:title`; `Authors` from every `dc:creator`; `Publisher`, `Description` (tags stripped to plain text), `Published` (first `dc:date`), `Rights`, `Language`, `Subjects`; `ISBN` from an identifier whose scheme is ISBN or whose text starts with `urn:isbn:` (prefix removed). `Format` is `"EPUB 3"` or `"EPUB 2"` from `package@version` (a version starting with `3`), with `" (fixed layout)"` appended when Task 3.11 says the book is pre-paginated; `Encoding` is `"UTF-8"`; `TextBytes` is the length of the assembled markup.

- [x] **Task 3.3:** Number the images and find the cover in `epub.go`. Walk the manifest in order; every item with an image media type (`image/jpeg`, `image/png`, `image/gif`, `image/webp`, `image/bmp`; skip `image/svg+xml`) gets the next 1-based index and a `Resource` whose `Kind` comes from `sniffImage` on its bytes (skip items that do not sniff as an image, so a mislabelled file is not handed to the renderer). Keep a map from resolved zip path to index for the markup rewriting. `Cover` is, in order: the item whose `properties` contain `cover-image` (EPUB 3); the item whose `id` matches `<meta name="cover" content="...">` (EPUB 2); the first image referenced by a `guide` reference of type `cover`; else nil.

### Reflowable markup

- [x] **Task 3.4:** Assemble the spine in a new `internal/ebook/epub_markup.go`. For each `itemref` in order (linear ones first, then those with `linear="no"`), take the manifest item; skip items that are not `application/xhtml+xml` or `text/html`. Before parsing, expand self-closing non-void tags (`<title/>`, `<div/>`, `<a id="x"/>`, `<span/>` and so on, matched with a regexp and rewritten to `<tag ...></tag>`), because the HTML parser treats `<title/>` as an open title that swallows the rest of the document. Parse with `html.Parse` from `golang.org/x/net/html`, apply Tasks 3.5 to 3.7 and 3.9 to the tree, render it back with `html.Render`, and append it to `Book.HTML`. Each spine document keeps its own `<html>` element, which `htmldoc` already treats as a page break. A spine with no readable document fails with "this EPUB has no readable chapters".

- [x] **Task 3.5:** Rewrite images in `epub_markup.go`. For every `<img src>` and every SVG `<image>` (`href` or the `xlink`-namespaced `href` attribute), resolve the reference against the spine document's path and replace it with `recindex="N"` from Task 3.3's map; keep `alt`. An `<svg>` element that wraps an `<image>` (the usual EPUB cover page) is replaced by a single `<img recindex="N">`, since `htmldoc` drops `<svg>`. References that do not resolve to a numbered image (remote URLs, missing files, SVG files) are removed. Remove `<script>` elements.

- [x] **Task 3.6:** Gather the book's CSS into `Book.Flows` in `epub_markup.go`. For each spine document, in order, collect `<link rel="stylesheet" href>` targets (resolved against the document, read once each, de-duplicated by path) and the text of every `<style>` element, and append them to `Flows` in first-seen order. Skip `@import`; `htmldoc.ParseCSS` already ignores selectors it does not understand.

- [x] **Task 3.7:** Skip the duplicate cover page in `epub_markup.go`. When `Book.Cover` is set and the book is not pre-paginated, drop a spine document whose body has no text (whitespace only) and whose only image is the cover resource, when it is the first spine item or the target of a `guide` reference of type `cover` or an EPUB 3 landmark `epub:type="cover"`. The renderer's own cover page shows that image instead, so it would otherwise appear twice. Fixed-layout books keep every page, since the engine turns their cover page off (Task 2.15).

### Table of contents

- [x] **Task 3.8:** Read the table of contents in a new `internal/ebook/epub_toc.go`. EPUB 3: the manifest item whose `properties` contain `nav`; parse it with `html.Parse`, find the `<nav>` whose `epub:type` contains `toc`, and walk its nested `<ol>`/`<li>`/`<a href>` recording each target with its depth (top level is 1). EPUB 2, or EPUB 3 without a nav: the NCX named by `spine@toc` (or the item with media type `application/x-dtbncx+xml`); parse `navMap/navPoint` recursively, `content@src` with the navPoint depth. Resolve targets against the nav or NCX path and return `map[string][]tocTarget` keyed by document path (`tocTarget{Fragment string; Depth int}`). A broken or missing TOC returns an empty map, never an error.

- [x] **Task 3.9:** Mark TOC targets as headings during markup assembly (Task 3.4). For each target in the current document: when it has a fragment and an element with that `id` exists and is a block element (`h1`-`h6`, `p`, `div`, `section`, `header`, `article`, `blockquote`, `li`, `figure`), set `data-ebook-heading` to the depth clamped to 1..6. Otherwise (no fragment, id missing, or an inline element such as an anchor `<a id>` or `<span>`), mark the first block element in `<body>` that has text, unless an earlier target already marked it. Do not mark the same element twice; the shallowest depth wins. The existing heading detection in `htmldoc` still runs on everything else.

### DRM

- [x] **Task 3.10:** Refuse locked books in a new `internal/ebook/epub_drm.go`, checked by `decodeEPUB` before anything else is read. Locked when the archive has `META-INF/rights.xml` (Adobe ADEPT), `META-INF/sinf.xml` (Apple FairPlay), or `META-INF/license.lcpl` (Readium LCP), or when `META-INF/encryption.xml` has any `EncryptedData` whose `EncryptionMethod@Algorithm` is not `http://www.idpf.org/2008/embedding` or `http://ns.adobe.com/pdf/enc#RC` (the two font-obfuscation schemes, which only scramble embedded fonts that the renderer ignores anyway). Return a wrapped `ErrDRMLocked` sentinel: "this EPUB is locked with <scheme> DRM, which ebook-to-pdf does not remove; convert a DRM-free copy (many stores and publishers sell them)". `Open` prefixes the path, and a failed conversion already exits with code 1 through `cmd/ebook-to-pdf/main.go`. Do not attempt any decryption (out of scope by decision).

### Fixed layout

- [x] **Task 3.11:** Detect fixed layout in a new `internal/ebook/epub_fixed.go`. The book is pre-paginated when the OPF has `<meta property="rendition:layout">pre-paginated</meta>`, or, for EPUB 2, `META-INF/com.apple.ibooks.display-options.xml` sets the option `fixed-layout` to `true`. An `itemref` whose `properties` contain `rendition:layout-pre-paginated` or `rendition:layout-reflowable` overrides the book setting for that page. Set `Book.Paged` when the book-level setting is pre-paginated.

- [x] **Task 3.12:** Work out each fixed page's size in `epub_fixed.go`, in pixels, then convert at 0.75 pt per pixel: the document's `<meta name="viewport" content="width=W, height=H">` (tolerate missing spaces and a `px` suffix); else the `width`/`height` or `viewBox` of an `<svg>` that is the body's only child; else the OPF's `<meta property="rendition:viewport">` in the same `width=..., height=...` form; else the pixel size of the page's only image (`image.DecodeConfig`, WebP through `golang.org/x/image/webp`); else the previous page's size; else the first page's size found later in the spine; else 600 x 800 pixels.

- [x] **Task 3.13:** Emit fixed pages from `epub_markup.go` for pre-paginated spine items. A page whose body has no text and exactly one image (an `<img>` or an `<svg>`-wrapped `<image>`) becomes `<html><body><ebook:page width="W" height="H"></ebook:page><img recindex="N" data-ebook-fullpage="1"></body></html>` with `W` and `H` in points: one PDF page, image fitted to it. Any other page becomes the same `<ebook:page>` followed by the body's content (images rewritten as in Task 3.5), so its text is reflowed onto one page of the right size; absolute CSS positioning is ignored (out of scope). Reflowable items in a pre-paginated book go through Task 3.4 unchanged.

### Tests

- [x] **Task 3.14:** Test a reflowable EPUB 3 in `internal/ebook/epub_test.go`, built with `buildZip`: `mimetype` (stored), `META-INF/container.xml`, `OEBPS/content.opf` (version 3.0, title, two creators, an ISBN identifier, a `cover-image` item, a `nav` item, a linked CSS file), a cover XHTML page with an `<svg><image xlink:href>`, two chapter XHTML files with images in a sibling `images/` folder using `../images/` paths, and a nav with a nested entry pointing at `ch2.xhtml#sec`. Assert: metadata and `Format == "EPUB 3"`; `Cover` set; the cover page is not in `HTML`; chapter order follows the spine, not the manifest; every `<img>` carries a valid `recindex`; the CSS is in `Flows`; parsing `HTML` with `htmldoc.ParseWithCSS` gives headings for both nav targets at the right levels; a chapter containing `<title/>` still yields its body text.

- [x] **Task 3.15:** Test an EPUB 2 in `epub_test.go`: version 2.0 OPF, `<meta name="cover" content="cover-img">`, an NCX referenced by `spine@toc` with two levels of `navPoint`, an `<a id="c1"/>` inline anchor as a target (the first block of that file becomes the heading), an `<img>` whose `src` contains `%20`, and a `linear="no"` item that ends up after the linear ones. Assert metadata, cover, headings and order.

- [x] **Task 3.16:** Test DRM handling in `epub_test.go`: archives containing `META-INF/rights.xml`, `META-INF/sinf.xml`, `META-INF/license.lcpl`, and an `encryption.xml` with an AES `EncryptionMethod` each fail with `errors.Is(err, ErrDRMLocked)` and a message naming the scheme; an `encryption.xml` listing only `http://www.idpf.org/2008/embedding` entries for font files decodes successfully. Through `Open` on a file in `t.TempDir()`, the error names the file.

- [x] **Task 3.17:** Test fixed layout in `epub_test.go`: a pre-paginated EPUB 3 with three spine pages, an image-only page with a 1200 x 1600 viewport, a text page with a 1000 x 1400 viewport, and an image-only page without a viewport whose PNG is 800 x 1000. Assert `Paged`, `Format` ends with "(fixed layout)", and that `htmldoc.ParseWithCSS(book.HTML, nil)` gives sized breaks of 900 x 1200, 750 x 1050 and 600 x 750 points, with `FullPage` images on the first and third pages. Also: an EPUB 2 with the Apple display-options file is pre-paginated, and an `itemref` with `rendition:layout-reflowable` in a pre-paginated book is emitted without `<ebook:page>`.

- [x] **Task 3.18:** Test end to end in a new `internal/engine/epub_convert_test.go`: write the reflowable EPUB 3 fixture from Task 3.14 and the fixed-layout fixture from Task 3.17 to `t.TempDir()` (the `engine` package cannot see `ebook` test helpers, so build small copies of the fixtures with `archive/zip` there), run `engine.Convert` with default options, and assert the reflowable PDF has `Headings >= 2` and `Images >= 2`, and the fixed-layout PDF has exactly three pages (no cover or title page) and no dropped images.

## Acceptance Criteria

- Reflowable EPUB 2 and EPUB 3 convert with spine order, metadata, cover, images and TOC headings as bookmarks
- A fixed-layout EPUB produces one PDF page per spine page at its declared size, with no cover or title page added
- A DRM-locked EPUB fails with the DRM message, names the file and exits with code 1; a font-obfuscated EPUB converts
- Nothing is extracted to disk; every archive read goes through `zipsafe.go`
- `make ci` passes and `go.mod` gains no modules
- Only `internal/ebook/epub*.go`, their tests and `internal/engine/epub_convert_test.go` changed

## Notes

- `css.go` understands a small subset of CSS on purpose (NFR-3). EPUB headings styled only through selectors it ignores are why TOC targets are marked independently (Task 3.9).
- After this phase, run the `ebook-to-pdf-verify-output` skill on one real reflowable EPUB and one fixed-layout EPUB the owner supplies; no copyrighted samples are checked in.
- The self-closing tag expansion in Task 3.4 is a format quirk worth a comment in the code: XHTML allows `<title/>`, the HTML parser does not.

## Phase Completion Summary

**Date:** 2026-10-25
**Implementer:** Claude (Implement + Review)

**What was done:** The EPUB stub is replaced by a real reader. `decodeEPUB` refuses DRM-locked books first, finds the package through `container.xml`, reads the OPF metadata, numbers the manifest images, finds the cover, then assembles the spine in reading order with images rewritten to `recindex`, SVG cover images unwrapped, scripts dropped, the duplicate cover page skipped, CSS gathered into `Flows` and table-of-contents targets marked as headings. Pre-paginated books emit one sized `<ebook:page>` per spine item, picture pages as full-page images.

**Files created:** `internal/ebook/epub_markup.go`, `internal/ebook/epub_toc.go`, `internal/ebook/epub_fixed.go`, `internal/ebook/epub_drm.go`, `internal/ebook/epub_test.go`, `internal/engine/epub_convert_test.go`

**Files modified:** `internal/ebook/epub.go` (stub replaced), `internal/ebook/open_test.go` (EPUB case removed from `TestOpenReachesFormatReaders`, which asserted the stub's "not finished" error)

**Tests:** `go test ./...` all pass; `golangci-lint run ./...` 0 issues; gofmt clean; the binary builds and runs `version`. `make` is not installed on this machine, so the `ci` steps were run one by one, and `-race` was not run (it needs cgo on Windows; CI runs it). `go.mod` gained no modules.

**Issues and decisions:**

> BUG: `xml.HTMLAutoClose` closed EPUB 3's `<meta property="...">value</meta>` before its value, so the OPF failed to parse. Auto-closing is off in `decodeXML`. Regression test: `TestEPUB3Reflowable` and the fixed-layout tests.

> SPEC NOTE: `open_test.go` is outside this phase's file list, but its Phase 2 test asserted the EPUB stub's error, so the EPUB entry was removed from it.

> SPEC NOTE: a TOC target that wraps a chapter (`<section id>`, `<div id>`) marks the first block of text inside it, and an inline target (`<a id>`, `<span id>`) marks the block around it; the first-text-block fallback, which picks the innermost block around the document's first text, is used only when neither applies. Marking the wrapper itself, as Task 3.9 reads literally, gives no heading at all, because the markup parser flushes it empty when the inner block opens.

> SPEC NOTE: `@import` statements are removed from the gathered CSS, because the stylesheet parser would otherwise read `@import ...; .next {` as one at-rule and drop the following rule.

> SPEC NOTE: an `encryption.xml` that encrypts anything but fonts, with no rights or licence file beside it, is reported as "locked with an unrecognised DRM".

> SPEC NOTE: table-of-contents heading marks are applied to reflowable documents only; fixed pages are emitted as Task 3.13 describes.

**Review (Implement + Review):** a fresh helper reviewed the 8 changed files and found 0 critical, 3 warnings and 3 suggestions, all confirmed. All 6 were fixed: TOC targets on wrapper and inline elements, a UTF-8 BOM stripped before parsing, chapter read errors now fail the book naming the chapter (a missing chapter is still skipped), `image/jpg` accepted, and new tests for script removal, dropped remote images, guide and landmark cover pages, page size fallbacks, the BOM and read errors. Lint, tests and build pass again.

Still to do after sign-off (phase Notes): run `ebook-to-pdf-verify-output` on one real reflowable and one fixed-layout EPUB that the owner supplies.
