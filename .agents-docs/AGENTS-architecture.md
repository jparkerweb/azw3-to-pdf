# Architecture
> Part of [AGENTS.md](../AGENTS.md): project guidance for AI coding agents.

The conversion is a pipeline, one package per stage. Each stage can be read and
tested without the ones after it.

```
file bytes
  → internal/ebook     Open → Detect → mobi | epub | cbz | fb2 | plaintext | htmlfile  →  Book
  → internal/htmldoc   markup + CSS (+ 3 reader hooks)  →  flat list of layout blocks
  → internal/pdfout    blocks  →  pages (per-page sizes, full-page images)
  → internal/engine    job orchestration, paged-book rules, folder scans, output naming
```

`internal/cli` chooses between the terminal interface (`internal/tui`) and
headless conversion. `internal/presets` holds the named layouts, and
`internal/config` the YAML settings file.

Where the rest lives:

| File | Holds |
| --- | --- |
| `engine/convert.go` | `Convert`: renders to a temp file, then renames it into place (`replaceFile` copes with Windows); `pdfOptionsFor` turns the cover and title page off for a `Book.Paged` book |
| `engine/output.go` | `ResolveOutput`, `ConflictMode`, `ErrSkipped`, `BookExtensions`, `TextExtensions`, `IsConvertible`, `DiscoverBooks` |
| `engine/batch.go` | `RunBatch`, and the `convertFn` test seam |
| `engine/open*.go` | `OpenFolder`/`OpenFile`, launching the OS viewer for `--open` |
| `cli/flags.go` | `buildPDFOptions`: preset, then config (`configOverrides`), then flags (`flagOverrides`) |
| `cli/headless.go` | `runHeadless` and `dryRun`; `probe.go`, `presets.go` and `completion.go` are the other subcommands |
| `config/paths.go` | `Dir`, `CacheDir`, `LogDir` |
| `logging/logging.go` | `Setup`: a log file in TUI mode, stderr headless |
| `presets/presets.go` | `PageSizeFor`, `OptionsWithPageSize`, `SummaryFor`, `Recommend` (a preset from image count and text size) |
| `tui/screens/helpers.go` | Shared screen helpers: `LoadBook`, `goTo`, `back`, `status`, `tick` |

## Gotchas across packages

- **`ApplyOverrides` does not normalize.** Call `Options.Normalize` once, after
  the last layer (preset, config, flags) has been applied.
- **Custom page sizes are saved in points** (`tui/app.go`): a custom
  `PageSize.Name` has no unit and would read back as millimetres.
- **`--dry-run` still creates the output folder**, because it goes through
  `ResolveOutput`, which runs `os.MkdirAll`.
- **Batch esc aborts the book in progress.** The TUI says "Stopping after the
  current book", but it cancels the shared context, so the current book is
  reported as failed.
- **The batch screen ignores `--output`**: each book gets its own destination.
- **DRM errors differ by format.** EPUB DRM unwraps to `ebook.ErrDRMLocked`;
  Kindle DRM is a plain error from `text.go`, so `errors.Is` is false for it.

## internal/ebook

`open.go` is the dispatcher: `Open`, `Decode`, `Detect` and `DetectFile` work
the format out from the content first (Palm `BOOKMOBI`/`TEXtREAd`; a zip with an
EPUB `mimetype` or `container.xml`, with a `.fb2` entry, or mostly images;
`<FictionBook` XML) and the extension second (`.txt`, `.html`). Every reader
returns the same `Book`: metadata, markup in reading order, CSS in `Flows`, and
images numbered from 1 and addressed by `recindex` (KF8 books use
`kindle:embed:XXXX` instead, which the parser also resolves). The comment on
`Book.Flows` calling them diagnostics-only is stale: the engine passes them to
`ParseCSS`. `Book.Paged` marks a book pre-paginated as a whole (comics,
fixed-layout EPUB). A reflowable EPUB with a few pre-paginated spine items gets
`<ebook:page>` for those pages but stays unpaged, so it still gets a cover and
title page.

Points that hold for every reader:

- **The zip guard is the only way into archives.** EPUB, CBZ and `.fb2.zip`
  are opened with `openZip` and read through `zipArchive.read` in
  `zipsafe.go`, which caps entry and total size and the compression ratio and
  drops names that climb out of the archive. Never use `archive/zip` directly;
  the one exception, `DetectFile`, opens the file with `zip.OpenReader` but
  wraps it in `newZipArchive` straight away. The size caps are variables only
  so tests can lower them, and sizes count bytes actually read, never header
  claims.
- **Only images `sniffImage` recognises are numbered.** A manifest or a file
  name is never trusted on its own.

### Kindle (`mobi.go`)

Parses the Palm database container and the MOBI header (`header.go` documents
the field offsets, which are relative to the start of record 0: the PalmDOC
header and the MOBI header share one address space).

Points worth knowing:

- **Two decompressors.** PalmDOC (LZ77, `palmdoc.go`) and HUFF/CDIC
  (`huffcdic.go`). Most modern KF8 books use HUFF/CDIC.
- **HUFF/CDIC arithmetic must be 64-bit.** The reference algorithm shifts code
  ranges left by up to 32 bits and compares them against a 32-bit code; doing
  that in `uint32` silently wraps and produces plausible-looking text with
  wrong words scattered through it. The `dict2` tables are also indexed by code
  length starting at 1, not 0.
- **Trailing entries.** Every text record can carry trailer bytes described by
  the extra-data flags; they are stripped before decompression.
- **`textLength` is not the end of the stream.** In KF8 it measures the main
  flow only; the stylesheet flows follow it. Flow boundaries come from the FDST
  record.
- **Reading order.** The KF8 raw text is read straight through rather than
  reassembled from the skeleton and fragment indices. Paragraph order is
  correct either way; the only artefacts are stray `<html>`/`<body>` wrappers
  between parts, which the markup parser ignores (and uses as page breaks).
- **DRM is refused, not worked around.** An encrypted book produces a clear
  error.

### EPUB (`epub*.go`)

- **`<title/>` must be expanded.** XHTML allows `<tag/>` for any element, but
  the HTML parser ignores the slash, so `<title/>` swallows the rest of the
  chapter. `expandSelfClosing` rewrites non-void short tags first; the HTML
  reader uses it too.
- **`decodeXML` is lenient on purpose**: non-strict, with HTML entities.
  Auto-closing stays off because it breaks EPUB 3 `<meta>` values.
- **The cover page is dropped.** The first spine document, or one marked as
  the cover, is skipped when it holds only the cover image, because the
  renderer draws its own cover. A listed chapter that is missing is skipped;
  one that cannot be read fails the whole book.
- **TOC targets are marked with `data-ebook-heading`.** EPUB CSS often styles
  chapter titles with selectors `css.go` ignores, so the nav or NCX targets are
  marked as headings directly, whatever the stylesheet says.
- **Font obfuscation is not DRM.** An `encryption.xml` that lists only IDPF or
  Adobe font obfuscation converts; `rights.xml`, `sinf.xml`, `license.lcpl` or
  any other encrypted entry is refused (`epub_drm.go`).
- **Fixed layout** emits one `<ebook:page width height>` per spine item; a
  page that is one picture becomes a full-page image, anything else is
  reflowed onto a page of that size.

### Comic archives (`cbz.go`)

- **Natural sort.** Pages are ordered by `naturalLess`: case-insensitive,
  digit runs compared as numbers, so `2.jpg` comes before `10.jpg`. Dot files
  and `__MACOSX/` are skipped.
- Each image is one `<img data-ebook-fullpage>`, so it gets a page of its own
  at its natural size. `ComicInfo.xml` at the root fills the metadata; a
  malformed one is ignored.

### FictionBook (`fb2.go`)

- **Binaries come after the body.** Images are base64 `<binary>` elements at
  the end of the file, so `<image>` references are resolved once the whole
  document has been read.
- Section nesting gives the heading level; each top-level section starts with
  `<mbp:pagebreak/>`. The declared encoding is honoured (Windows-1251 and
  KOI8-R are common); with none, UTF-8 if valid, else Windows-1251.

### Plain text (`plaintext.go`)

- **Headings are conservative on purpose.** A heading is a single short line
  such as `CHAPTER IV`, `Part Two` or a lone Roman numeral other than `I`. A
  false heading breaks the page mid-text, which is worse than a missed
  bookmark, so add negative tests with any new pattern.
- UTF-8 if valid, else Windows-1252. Project Gutenberg headers and licences
  are trimmed between the `*** START OF` and `*** END OF` markers.

### HTML (`htmlfile.go`)

- Images and stylesheets are read only from the file's own folder tree, never
  from outside it and never over the network. Paths are checked again after
  symlinks are resolved, and each resource is capped at 64 MiB. `data:` images
  are decoded in place.

## internal/htmldoc

A streaming tokenizer (`golang.org/x/net/html`) turns markup into `Block`s
holding styled `Span`s. It is not a browser and should not grow into one.

The book's own stylesheet matters a great deal: Kindle books style headings
with classes rather than heading tags, so `css.go` parses the small set of
declarations that change layout (`text-align`, `font-size`, left and right
margin or padding, `text-indent`, `display:block`, `page-break-*`, a monospace
`font-family`, weight and style). Without it, chapter titles render as body
text and run-in headings collide with the text after them.

Three hooks are the format-specific markup the readers write for the parser:

- `data-ebook-heading="N"` on any element makes it a level-N heading.
- `data-ebook-fullpage` on an `<img>` gives the image a page of its own.
- `<ebook:page width height>` starts a page of that size in points.

The parser also understands Kindle markup, which the FB2 and plain-text readers
reuse: `<mbp:pagebreak/>`, a repeated `<html>` as a page break, `recindex` and
`kindle:embed:` image references, and `hN` class names as headings.

`css.go` only records a block's raw CSS size (`Block.Scale`). Clamping,
dampening and the heading defaults all live in `scale.go`'s `EffectiveScale`,
which the renderer calls; change scaling there and nowhere else. Do not tune
its constants: they are kept for output parity.

## internal/pdfout

Lays blocks out with `github.com/signintech/gopdf`. Text is positioned by
baseline from the top of the page. Fonts are resolved at render time: a system
TrueType face if one is found, otherwise the Go faces embedded through
`golang.org/x/image/font/gofont`. Collections and CFF-outline OpenType fonts
are rejected because gopdf cannot embed them; the check reads the file's magic
bytes, so an `.otf` with TrueType outlines is accepted.

`render.go` holds only the exported types and the `Render` entry point; the
work is split by concern into `render_text.go`, `render_page.go`,
`render_blocks.go`, `render_image.go` and `render_cover.go`.

Every page records its own size, so one PDF can mix them; margins are
recomputed per page. A full-page image is drawn without margins, header or
page number, filling a page a sized break has just opened or else a page of its
natural size at 96 dpi. A plain page break straight after a full-page image
does not open another page.

Word wrapping measures each word in its own style, so spaces that fall between
two differently styled runs have to be tracked explicitly
(`splitKeepingSpaces`); losing them glues `<b>Note:</b> text` together.

## internal/tui

Bubble Tea v2 (`charm.land/*`) with one model per screen, all implementing
`style.ScreenModel`. The top-level `App` owns navigation, the shared state and
the viewport; screens communicate through `internal/tui/messages`, never by
calling each other.

Conversions run in a goroutine. Progress goes through a one-slot buffered
channel with a non-blocking send, so a slow terminal never slows the converter
down; the done message and batch results use blocking sends (batch results on
a channel sized for every book).
Batches go through `engine.RunBatch`, the same runner headless mode uses with
`--jobs`; the TUI always passes a concurrency of 1.

`app_test.go` renders every screen headlessly; add to it when adding a screen.
