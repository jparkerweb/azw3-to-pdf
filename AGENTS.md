# AGENTS.md

Project documentation for AI coding agents working on `azw3-to-pdf`.

## What this is

A Go CLI that converts Kindle books (`.azw3`, `.azw`, `.mobi`, `.prc`) into
PDFs. Everything is in-process: the MOBI/KF8 parser, the layout engine and the
PDF writer. There is no dependency on Calibre or any other external tool, and
adding one would defeat the point of the project.

## Commands

```sh
make build     # build ./azw3-to-pdf
make test      # go test ./...
make lint      # golangci-lint run ./...
make ci        # lint, test and build, exactly what CI does
make run       # build and open the terminal interface
make fmt       # gofmt -w .
make snapshot  # cross-platform build with goreleaser
```

`.github/workflows/ci.yml` runs golangci-lint, `go test -race` with a coverage
artifact, and a build that executes `azw3-to-pdf version` to prove the binary
works. All three must be green before a pull request merges, so run `make ci`
first. golangci-lint runs on its defaults, with no configuration file.

`.github/workflows/release.yml` fires on a `v*` tag: it runs the tests and then
GoReleaser, which publishes archives for Linux, macOS and Windows on amd64 and
arm64 with the version, commit and date compiled in. Release notes are grouped
from conventional commit prefixes, so `feat:`, `fix:` and `perf:` messages land
in the right section of the changelog.

Go is pinned to 1.25 in both workflows and in `go.mod`; keep them in step.

## Architecture

The conversion is a pipeline, one package per stage. Each stage can be read and
tested without the ones after it.

```
file bytes
  → internal/ebook     container, headers, decompression, resources
  → internal/htmldoc   markup + CSS  →  flat list of layout blocks
  → internal/pdfout    blocks  →  pages
  → internal/engine    job orchestration, output naming, progress
```

`internal/cli` chooses between the terminal interface (`internal/tui`) and
headless conversion. `internal/presets` holds the named layouts, and
`internal/config` the YAML settings file.

### internal/ebook

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

### internal/htmldoc

A streaming tokenizer (`golang.org/x/net/html`) turns markup into `Block`s
holding styled `Span`s. It is not a browser and should not grow into one.

The book's own stylesheet matters a great deal: Kindle books style headings
with classes rather than heading tags, so `css.go` parses the small set of
declarations that change layout (`text-align`, `font-size`, `margin-left`,
`text-indent`, `display:block`, `page-break-*`, weight and style). Without it,
chapter titles render as body text and run-in headings collide with the text
after them.

### internal/pdfout

Lays blocks out with `github.com/signintech/gopdf`. Text is positioned by
baseline from the top of the page. Fonts are resolved at render time: a system
TrueType face if one is found, otherwise the Go faces embedded through
`golang.org/x/image/font/gofont`. `.ttc` and `.otf` files are rejected because
gopdf cannot embed them.

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

`app_test.go` renders every screen headlessly; add to it when adding a screen.

## Adding things

- **A page size**: `internal/pdfout/options.go`, the `pageSizes` map.
- **A preset**: `internal/presets/presets.go`, the `registry` slice.
- **A layout setting**: add it to `pdfout.Options`, then to the flag set in
  `internal/cli/flags.go`, the `layoutFields` table in
  `internal/tui/screens/layout.go` and `messages.LayoutChangedMsg`.
- **A screen**: a model in `internal/tui/screens`, a `messages.Screen`
  constant, registration in `NewApp`, and footer and help entries in `app.go`.

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
