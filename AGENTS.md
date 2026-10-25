# AGENTS.md

Project documentation for AI coding agents working on `ebook-to-pdf`.

## What this is

A Go CLI that converts ebooks into PDFs: EPUB 2 and 3 (reflowable and
fixed-layout), Kindle (`.azw3`, `.azw`, `.mobi`, `.prc`), comic archives
(`.cbz`), FictionBook 2 (`.fb2`, `.fb2.zip`), plain text (`.txt`) and single
HTML files. Everything is in-process: every format reader, the layout engine
and the PDF writer. There is no dependency on Calibre or any other external tool, and
adding one would defeat the point of the project.

## Commands

```sh
make build     # build ./ebook-to-pdf
make test      # go test ./...
make lint      # golangci-lint run ./...
make ci        # lint, test and build, exactly what CI does
make run       # build and open the terminal interface
make fmt       # gofmt -w .
make snapshot  # cross-platform build with goreleaser
```

`.github/workflows/ci.yml` runs golangci-lint, `go test -race` with a coverage
artifact, and a build that executes `ebook-to-pdf version` to prove the binary
works. All three must be green before a pull request merges, so run `make ci`
first. golangci-lint runs on its defaults, with no configuration file.

`.github/workflows/release.yml` fires on a `v*` tag: it runs the tests and then
GoReleaser, which publishes archives for Linux, macOS and Windows on amd64 and
arm64 with the version, commit and date compiled in. Release notes are grouped
from conventional commit prefixes, so `feat:`, `fix:` and `perf:` messages land
in the right section of the changelog.

Go is pinned to 1.25 in both workflows and in `go.mod`; keep them in step.

Two environment variables exist: `EBOOK_TO_PDF_CONFIG` relocates the
configuration directory (`internal/config/paths.go`; tests use it to keep
away from the real settings), and `EBOOK_TO_PDF_LOG_LEVEL` overrides the log
level (`internal/logging/logging.go`).

## Architecture

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

### internal/ebook

`open.go` is the dispatcher: `Open`, `Decode`, `Detect` and `DetectFile` work
the format out from the content first (Palm `BOOKMOBI`/`TEXtREAd`; a zip with an
EPUB `mimetype` or `container.xml`, with a `.fb2` entry, or mostly images;
`<FictionBook` XML) and the extension second (`.txt`, `.html`). Every reader
returns the same `Book`: metadata, markup in reading order, CSS in `Flows`, and
images numbered from 1 and addressed by `recindex`. `Book.Paged` marks a
pre-paginated book (comics, fixed-layout EPUB).

Points that hold for every reader:

- **The zip guard is the only way into archives.** EPUB, CBZ and `.fb2.zip`
  are opened with `openZip` and read through `zipArchive.read` in
  `zipsafe.go`, which caps entry and total size and the compression ratio and
  drops names that climb out of the archive. Never use `archive/zip` directly.
- **Only images `sniffImage` recognises are numbered.** A manifest or a file
  name is never trusted on its own.

#### Kindle (`mobi.go`)

Parses the Palm database container and the MOBI header (`header.go` documents
the field offsets, which are relative to the start of record 0 — the PalmDOC
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

#### EPUB (`epub*.go`)

- **`<title/>` must be expanded.** XHTML allows `<tag/>` for any element, but
  the HTML parser ignores the slash, so `<title/>` swallows the rest of the
  chapter. `expandSelfClosing` rewrites non-void short tags first.
- **TOC targets are marked with `data-ebook-heading`.** EPUB CSS often styles
  chapter titles with selectors `css.go` ignores, so the nav or NCX targets are
  marked as headings directly, whatever the stylesheet says.
- **Font obfuscation is not DRM.** An `encryption.xml` that lists only IDPF or
  Adobe font obfuscation converts; `rights.xml`, `sinf.xml`, `license.lcpl` or
  any other encrypted entry is refused (`epub_drm.go`).
- **Fixed layout** emits one `<ebook:page width height>` per spine item; a
  page that is one picture becomes a full-page image, anything else is
  reflowed onto a page of that size.

#### Comic archives (`cbz.go`)

- **Natural sort.** Pages are ordered by `naturalLess`: case-insensitive,
  digit runs compared as numbers, so `2.jpg` comes before `10.jpg`. Dot files
  and `__MACOSX/` are skipped.
- Each image is one `<img data-ebook-fullpage>`, so it gets a page of its own
  at its natural size. `ComicInfo.xml` at the root fills the metadata; a
  malformed one is ignored.

#### FictionBook (`fb2.go`)

- **Binaries come after the body.** Images are base64 `<binary>` elements at
  the end of the file, so `<image>` references are resolved once the whole
  document has been read.
- Section nesting gives the heading level; each top-level section starts with
  `<mbp:pagebreak/>`. The declared encoding is honoured (Windows-1251 and
  KOI8-R are common); with none, UTF-8 if valid, else Windows-1251.

#### Plain text (`plaintext.go`)

- **Headings are conservative on purpose.** A heading is a single short line
  such as `CHAPTER IV`, `Part Two` or a lone Roman numeral other than `I`. A
  false heading breaks the page mid-text, which is worse than a missed
  bookmark, so add negative tests with any new pattern.
- UTF-8 if valid, else Windows-1252. Project Gutenberg headers and licences
  are trimmed between the `*** START OF` and `*** END OF` markers.

#### HTML (`htmlfile.go`)

- Images and stylesheets are read only from the file's own folder tree, never
  from outside it and never over the network. `data:` images are decoded in
  place.

### internal/htmldoc

A streaming tokenizer (`golang.org/x/net/html`) turns markup into `Block`s
holding styled `Span`s. It is not a browser and should not grow into one.

The book's own stylesheet matters a great deal: Kindle books style headings
with classes rather than heading tags, so `css.go` parses the small set of
declarations that change layout (`text-align`, `font-size`, `margin-left`,
`text-indent`, `display:block`, `page-break-*`, weight and style). Without it,
chapter titles render as body text and run-in headings collide with the text
after them.

Three hooks are the only format-specific markup the parser knows, and readers
are the only ones who write them:

- `data-ebook-heading="N"` on any element makes it a level-N heading.
- `data-ebook-fullpage` on an `<img>` gives the image a page of its own.
- `<ebook:page width height>` starts a page of that size in points.

`css.go` only records a block's raw CSS size (`Block.Scale`). Clamping,
dampening and the heading defaults all live in `scale.go`'s `EffectiveScale`,
which the renderer calls; change scaling there and nowhere else.

### internal/pdfout

Lays blocks out with `github.com/signintech/gopdf`. Text is positioned by
baseline from the top of the page. Fonts are resolved at render time: a system
TrueType face if one is found, otherwise the Go faces embedded through
`golang.org/x/image/font/gofont`. `.ttc` and `.otf` files are rejected because
gopdf cannot embed them.

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

### internal/tui

Bubble Tea v2 (`charm.land/*`) with one model per screen, all implementing
`style.ScreenModel`. The top-level `App` owns navigation, the shared state and
the viewport; screens communicate through `internal/tui/messages`, never by
calling each other.

Conversions run in a goroutine and report through a one-slot buffered channel
with a non-blocking send, so a slow terminal never slows the converter down.
Batches go through `engine.RunBatch`, the same runner headless mode uses with
`--jobs`; the TUI always passes a concurrency of 1.

`app_test.go` renders every screen headlessly; add to it when adding a screen.

## Adding things

- **A format**: a reader file in `internal/ebook`, a `Format` constant and
  `Detect` rule in `open.go`, its extensions in `engine.BookExtensions`, and a
  row in the README's format table.
- **A page size**: `internal/pdfout/options.go`, the `pageSizes` map.
- **A preset**: `internal/presets/presets.go`, the `registry` slice. Set
  `SizePreset` only when the page size is the point of the preset (Paperback,
  A4, Phone); every other preset takes the user's default page size.
- **A layout setting**: add it to `pdfout.Options`, `OptionsOverrides`,
  `ApplyOverrides` and `OverridesFrom` in `internal/pdfout/options.go`
  (`TestOverridesCoverEveryOption` fails if one is missed), then to the flag set
  and `flagOverrides` in `internal/cli/flags.go` and the `layoutFields` table in
  `internal/tui/screens/layout.go`.
- **A screen**: a model in `internal/tui/screens` implementing
  `style.Screen` (`FooterHint`, `Bindings`, `PushState`, alongside
  `style.ScreenModel`'s `Init`/`Update`/`View`), a `messages.Screen` constant,
  and registration in `NewApp`.

## Default page size

The default page size is US Letter (`presets.DefaultPageSize`) unless the user
saved another as `layout.page_size`, from the layout screen's `s` key or the
config file. A preset with `SizePreset` keeps its own size whatever is saved;
`--page-size` overrides both for one run without saving. Resolve it through
`Preset.PageSizeFor` (which `OptionsWithPageSize` and `SummaryFor` use), never
by reading `Preset.PageSize` directly.

## Local skills

`.claude/skills/` holds the repeatable workflows for this repo. Prefer them
over improvising the same steps:

| Skill | Use it for |
|-------|-----------|
| `ebook-to-pdf-release` | Cutting a version: changelog, commit, tag, push, watch the pipeline |
| `ebook-to-pdf-update-docs` | Auditing README, AGENTS.md, the changelog and the skills against the code |
| `ebook-to-pdf-verify-output` | Converting a real book and looking at the rendered pages |
| `ebook-to-pdf-add-preset` | Adding or changing a layout preset or page size |

`settings.local.json` turns off gitignore filtering so that sample books and
generated PDFs, which are gitignored on purpose, can still be found when
testing.

## Releases

`CHANGELOG.md` follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/)
and semantic versioning, with the newest version at the top under an `Added`,
`Changed`, `Fixed` or `Removed` heading. Anything a user would notice belongs
in it before the pull request merges; refactors and test-only changes do not.

Cutting a release is: land the changelog entry with its date, then
`git tag vX.Y.Z && git push origin vX.Y.Z`. The tag drives everything else, so
the version in the binary comes from git rather than from a constant in the
source, and the compare links at the foot of the changelog need the new version
added.

## Conventions

- Errors say what the user should do about them, and name the file involved.
- Comments explain why, not what. Format quirks deserve a comment; obvious Go
  does not.
- No em dashes in prose, comments or commit messages.
- Never add Claude Code attribution to commits unless the harness asks for it.
