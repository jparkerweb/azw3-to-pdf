# azw3-to-pdf

Turn Kindle books into PDFs, from a terminal interface or a single command.

```
                     _____    __                     ______
  ____ ______      _|__  /   / /_____     ____  ____/ / __/
 / __ `/_  / | /| / //_ <   / __/ __ \   / __ \/ __  / /_
/ /_/ / / /| |/ |/ /__/ /  / /_/ /_/ /  / /_/ / /_/ / __/
\__,_/ /___/__/|__/____/   \__/\____/  / .___/\__,_/_/
                                      /_/
```

`azw3-to-pdf` reads `.azw3`, `.azw`, `.mobi` and `.prc` files and lays them out
as PDFs. Everything is built in: the MOBI/KF8 parser, the layout engine and the
PDF writer are all part of the binary. There is no Calibre, no Python, no
Ghostscript and nothing else to install.

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
go install github.com/jparkerweb/azw3-to-pdf/cmd/azw3-to-pdf@latest
```

Or clone and build:

```sh
git clone https://github.com/jparkerweb/azw3-to-pdf.git
cd azw3-to-pdf
make build
```

## Use

Open the interface and browse for a book:

```sh
azw3-to-pdf
```

Convert one book from the shell:

```sh
azw3-to-pdf book.azw3 --no-tui
```

Convert a whole folder into a separate output directory, four at a time:

```sh
azw3-to-pdf ~/Books --recursive --no-tui --output-dir ~/PDFs --jobs 4
```

Look inside a book without converting it:

```sh
azw3-to-pdf probe book.azw3 --verbose
```

### In the interface

| Key | Does |
| --- | --- |
| `enter` | Open a folder, choose a book, confirm |
| `space` | Add the highlighted book to a batch |
| `b` | Convert the batch |
| `tab` | Type a path instead of browsing |
| `d` | Switch drive (Windows) |
| `h` | Jump to your home folder |
| `l` / `p` | Fine-tune the layout / pick a preset |
| `?` | Keyboard shortcuts |
| `ctrl+t` | Switch between the two colour themes |
| `ctrl+c` | Quit |

## Presets

```sh
azw3-to-pdf presets
```

| Preset | Layout |
| --- | --- |
| `ereader` | A5, 11 pt serif, tight margins. The default. |
| `paperback` | 6 × 9 in with book margins and a running header. |
| `print` | US Letter with generous margins for printing. |
| `a4` | A4 with standard margins. |
| `compact` | Small type and thin margins; about a third fewer pages. |
| `large-print` | A4, 16 pt sans, open leading. |
| `phone` | A narrow page that fills a phone screen. |
| `manuscript` | Double-spaced Letter pages for mark-up. |

Any preset can be overridden:

```sh
azw3-to-pdf book.azw3 --no-tui \
  --preset paperback \
  --page-size 120x160mm \
  --font-size 12 \
  --line-spacing 1.5 \
  --margin 12 \
  --no-justify
```

`azw3-to-pdf page-sizes` lists the built-in sizes. A measurement works too:
`120x160mm`, `6x9in` or `432x648pt`.

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
(`azw3-to-pdf version` prints the path). Everything is optional:

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

## What it understands

| | |
| --- | --- |
| Containers | Palm database (`.azw3`, `.azw`, `.mobi`, `.prc`) |
| Formats | KF8 (MOBI 8) and MOBI 6, including dual-format files |
| Compression | None, PalmDOC (LZ77) and HUFF/CDIC |
| Text | UTF-8 and Windows-1252 |
| Images | JPEG, PNG, GIF and BMP |
| Styling | The book's own stylesheet: alignment, relative font sizes, hanging indents and page breaks |

**DRM is not removed.** A book bought from the Kindle store and never
de-authorised is encrypted, and `azw3-to-pdf` will say so rather than produce a
PDF of noise. Convert books you own in an unencrypted form.

`.kfx`, the newest Kindle format, is a different container and is not supported.

## Development

```sh
make build     # build ./azw3-to-pdf
make test      # go test ./...
make lint      # golangci-lint run ./...
make ci        # lint, test and build, as CI does
make run       # build and open the interface
make snapshot  # cross-platform build with goreleaser
```

Releases are cut by pushing a tag: `git tag v0.1.0 && git push origin v0.1.0`
runs the tests and then publishes binaries for Linux, macOS and Windows on both
amd64 and arm64 through GoReleaser.

The code is laid out in stages, each of which can be read on its own:

| Package | Does |
| --- | --- |
| `internal/ebook` | Palm container, MOBI/KF8 headers, decompression, resources |
| `internal/htmldoc` | Book markup and CSS to a flat list of layout blocks |
| `internal/pdfout` | Blocks to pages: wrapping, images, bookmarks, fonts |
| `internal/engine` | Conversion jobs, output naming, progress |
| `internal/presets` | Named layouts |
| `internal/tui` | The terminal interface |
| `internal/cli` | Flags, subcommands, headless conversion |

## Licence

MIT.
