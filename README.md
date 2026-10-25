# ebook-to-pdf

Turn ebooks into PDFs, from a terminal interface or a single command: EPUB,
Kindle, comic archives, FictionBook, plain text and HTML.

This project was renamed from azw3-to-pdf. Settings saved under the old name are not carried over.

<img src="ebook-to-pdf.jpg" style="max-width:800px;" alt="banner">

`ebook-to-pdf` reads `.epub`, `.azw3`, `.azw`, `.mobi`, `.prc`, `.cbz`, `.fb2`, `.fb2.zip`, `.txt`, `.html`, `.htm` and `.xhtml` files and lays them out as PDFs.  
Everything is built in: every format reader, the layout engine and the PDF writer are all part of the binary.  
There is no Calibre, no Python, no Ghostscript and nothing else to install.

- **A file browser in the terminal.** Run it with no arguments and pick a book.
- **Batch conversion.** Select several books, or point it at a folder.
- **Layout presets.** E-reader, paperback, print, large print and more, or tune
  every setting yourself.
- **Faithful pages.** Chapter breaks, headings, illustrations, the cover,
  hanging indents and justified text, taken from the book's own stylesheet.
- **Real PDFs.** Selectable text, outline bookmarks from the book's headings,
  page numbers and document metadata.

## Install

```sh
go install github.com/jparkerweb/ebook-to-pdf/cmd/ebook-to-pdf@latest
```

Or clone and build:

```sh
git clone https://github.com/jparkerweb/ebook-to-pdf.git
cd ebook-to-pdf
make build
```

## Use

Open the interface and browse for a book:

```sh
ebook-to-pdf
```

Convert one book from the shell:

```sh
ebook-to-pdf book.azw3 --no-tui
```

Convert a whole folder into a separate output directory, four at a time:

```sh
ebook-to-pdf ~/Books --recursive --no-tui --output-dir ~/PDFs --jobs 4
```

A folder scan picks up every ebook format except plain text and HTML, since a
folder of books often holds `.txt` notes and saved web pages that are not
books. Add `--include-text` to convert those too:

```sh
ebook-to-pdf ~/Books --no-tui --include-text
```

Look inside a book without converting it:

```sh
ebook-to-pdf probe book.epub --verbose
```

Every flag is listed by `ebook-to-pdf --help`, and
`ebook-to-pdf completion bash` (or `zsh`, `fish`, `powershell`) prints a shell
completion script.

### In the interface

| Key | Does |
| --- | --- |
| `enter` | Open a folder, choose a book, confirm |
| `esc` | Go back |
| `space` | Add the highlighted book to a batch |
| `a` / `x` | Add every book in the folder to the batch / clear the batch |
| `b` | Convert the batch |
| `tab` | Type a path instead of browsing |
| `d` | Switch drive (Windows) |
| `h` | Jump to your home folder |
| `l` / `p` | Fine-tune the layout / pick a preset |
| `r` | Reset the layout, or jump to the suggested preset |
| `s` | On the layout screen, save the page size as your default |
| `?` | Keyboard shortcuts |
| `ctrl+t` | Switch between the two colour themes |
| `ctrl+c` | Quit |

## Presets

```sh
ebook-to-pdf presets
```

| Preset | Page size | Layout |
| --- | --- | --- |
| `ereader` | default (Letter) | 11 pt serif, tight margins. The default preset. |
| `paperback` | 6 × 9 in | Book margins and a running header. |
| `print` | default (Letter) | Generous margins for printing. |
| `a4` | A4 | Standard margins. |
| `compact` | default (Letter) | Small type and thin margins; about a third fewer pages. |
| `large-print` | default (Letter) | 16 pt sans, open leading. |
| `phone` | Kindle | A narrow page that fills a phone screen. |
| `manuscript` | default (Letter) | Double-spaced pages for mark-up. |

Any preset can be overridden:

```sh
ebook-to-pdf book.azw3 --no-tui \
  --preset paperback \
  --page-size 120x160mm \
  --font-size 12 \
  --line-spacing 1.5 \
  --margin 12 \
  --no-justify
```

`ebook-to-pdf page-sizes` lists the built-in sizes. A measurement works too:
`120x160mm`, `6x9in` or `432x648pt`.

### Page size

Pages are US Letter unless you choose otherwise. The size presets (`paperback`,
`a4` and `phone`) always keep their own page; every other preset uses your
default page size.

- **Change the default** on the layout screen: pick a size and press `s` to
  save it as your default. To get the old A5 pages back, choose A5 and press `s`.
- **Or set it in the configuration file** with `layout.page_size` (for example
  `page_size: a5`).
- **For one run only**, pass `--page-size`; nothing is saved.

## Fonts

The body text uses a system serif face (Georgia, Times New Roman, DejaVu Serif
or Liberation Serif, whichever is found first). `--font sans` and `--font mono`
pick a system face of that kind, and `--font /path/to/Font.ttf` uses your own.
If nothing suitable is installed, the Go typefaces embedded in the binary are
used, so a conversion never fails for want of a font.

Only TrueType (`.ttf`) files can be embedded. OpenType/CFF (`.otf`) and font
collections (`.ttc`) are skipped in favour of the next candidate.

## Configuration

Settings are read from `config.yaml` in your configuration directory
(`ebook-to-pdf version` prints the path). Everything is optional:

```yaml
preset: paperback

output:
  dir: ~/PDFs
  suffix: ""
  conflict: rename   # fail, overwrite, rename or skip

layout:
  page_size: trade
  margin_mm: 18
  font: serif
  font_size: 11.5
  line_spacing: 1.42
  images: true
  cover: true
  title_page: true
  page_numbers: true
  bookmarks: true

ui:
  theme: midnight-ink   # or paper-sepia
```

Command-line flags override the file, which overrides the preset.

Three environment variables are read as well. `EBOOK_TO_PDF_CONFIG` moves the
whole configuration directory, which is handy for keeping a separate set of
settings. `EBOOK_TO_PDF_LOG_LEVEL` overrides the log level (for example
`debug`), even over `--log-level`; the terminal interface logs to a file in the
logs directory that `ebook-to-pdf version` prints, and headless runs log to
stderr. `EBOOK_TO_PDF_NO_TUI`, set to anything, never opens the terminal
interface, as if `--no-tui` were always given.

## What it understands

The format is worked out from the file's content first and its name second, so
a Kindle book misnamed `.epub` is still read as a Kindle book.

| Format | Extensions | What converts | Limits |
| --- | --- | --- | --- |
| EPUB 2 and 3 | `.epub` | Reflowable and fixed-layout books: spine order, metadata, cover, images, the book's stylesheet, and table of contents entries as headings and bookmarks | Fixed-layout text pages are reflowed, not positioned exactly |
| Kindle | `.azw3`, `.azw`, `.mobi`, `.prc` | KF8 (MOBI 8) and MOBI 6, including dual-format files; PalmDOC and HUFF/CDIC compression | `.kfx`, the newest Kindle format, is not supported |
| Comic archive | `.cbz` | One full page per image, in natural file-name order (`2.jpg` before `10.jpg`); `ComicInfo.xml` metadata | Zip only: `.cbr` (RAR) and `.cb7` are not read |
| FictionBook 2 | `.fb2`, `.fb2.zip` | Sections as headings, emphasis, poems, epigraphs, notes, embedded images and the cover; declared encodings such as Windows-1251 and KOI8-R | |
| Plain text | `.txt` | Paragraphs split on blank lines; conservative chapter heading detection (`CHAPTER IV`, `Part Two`, a lone `XIV`); Project Gutenberg headers and licences trimmed | UTF-8 or Windows-1252; skipped by folder scans without `--include-text` |
| HTML | `.html`, `.htm`, `.xhtml` | A single page with its own stylesheets and its images from the same folder or below | Remote images and anything outside the page's folder are never read; skipped by folder scans without `--include-text` |

Images may be JPEG, PNG, GIF, BMP or WebP. Styling comes from the book's own
stylesheet: alignment, relative font sizes, hanging indents and page breaks.

**Comics and fixed-layout books** get one PDF page per book page, each sized to
its page, with no added cover or title page: the book's first page already is
its cover. A fixed-layout book whose pages are text keeps the last fixed page
size for any reflowed pages that follow it.

**DRM is not removed.** A locked EPUB or Kindle book is refused with a clear
message rather than turned into a PDF of noise. Buy or download DRM-free copies
of the books you want to convert. (EPUB font obfuscation is not DRM, and such
books convert normally.)

## Development

```sh
make build     # build ./ebook-to-pdf
make test      # go test ./...
make lint      # golangci-lint run ./...
make ci        # lint, test and build (CI also runs the tests with -race)
make run       # build and open the interface
make install   # go install with the version compiled in
make snapshot  # cross-platform build with goreleaser
```

Releases are cut by pushing a version tag, which runs the tests and then
publishes binaries for Linux, macOS and Windows on both amd64 and arm64 through
GoReleaser. See [CHANGELOG.md](CHANGELOG.md) for what has changed in each
version.

The code is laid out in stages, each of which can be read on its own:

| Package | Does |
| --- | --- |
| `internal/ebook` | Format detection and one reader per format, each producing the same `Book` |
| `internal/htmldoc` | Book markup and CSS to a flat list of layout blocks |
| `internal/pdfout` | Blocks to pages: wrapping, images, bookmarks, fonts |
| `internal/engine` | Conversion jobs, output naming, progress |
| `internal/presets` | Named layouts |
| `internal/config` | The YAML settings file and where it lives |
| `internal/logging` | Log level and destination |
| `internal/tui` | The terminal interface |
| `internal/cli` | Flags, subcommands, headless conversion |

## Licence

MIT.
