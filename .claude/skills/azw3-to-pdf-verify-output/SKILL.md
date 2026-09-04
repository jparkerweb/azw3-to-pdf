---
name: azw3-to-pdf-verify-output
description: Convert a real book and actually look at the resulting PDF pages, to check layout changes or diagnose a book that converts badly. Use when the user says "check the output", "does it still look right", "verify the PDF", "this book renders wrong", "the layout is off", or after any change to the parser, markup handling or renderer. Also use when the user types "/azw3-to-pdf-verify-output".
user-invocable: true
---

# Verify Converted Output

Unit tests prove the PDF is well formed. They cannot tell you it looks right. This skill converts a real book and renders its pages to images so they can be inspected, which is the only way to catch layout regressions.

Use it after touching `internal/htmldoc` or `internal/pdfout`, and whenever a user reports that a particular book converts badly.

## Step 0: Find a Book and the Tools

A book is needed. Ask the user for a path if none is obvious, or look in the usual places (`~/Downloads`, `~/Documents`, a Calibre library). Sample books are gitignored, so they will not be in the repo.

Rendering needs poppler:

```bash
command -v pdftoppm pdftotext pdfinfo
```

If poppler is missing, say so and fall back to the text-only checks in Step 4. Do not silently skip the visual check and report success.

Work in the scratchpad directory, never in the repo. Copy the book to a short ASCII path first: long names with punctuation trip up tooling on Windows.

## Step 1: Inspect Before Converting

```bash
go run ./cmd/azw3-to-pdf probe /path/to/book.azw3 --verbose
```

Read the format, compression and block counts. This separates a parsing problem from a layout problem before any pages are drawn. Signals worth noticing:

| Symptom | Likely cause |
|---------|--------------|
| Garbled or wrong words in the headings list | Decompression bug, see the HUFF/CDIC notes in AGENTS.md |
| 0 headings on a book that clearly has chapters | The heading heuristic or the stylesheet parse |
| 0 images on an illustrated book | Resource indexing in `internal/ebook/book.go` |
| Very few blocks for a long book | Flow splitting or the markup parser |
| An error about DRM | Nothing to fix, the book is encrypted |

## Step 2: Convert

```bash
go run ./cmd/azw3-to-pdf /path/to/book.azw3 --no-tui -o "$SCRATCH/out.pdf" --overwrite
```

Add `--preset paperback` or other flags when checking a specific layout. Note the reported page count, image count and bookmark count.

## Step 3: Look at the Pages

Render a spread of pages, not just the first, and read them as images:

```bash
pdftoppm -png -r 100 -f 2 -l 2 "$SCRATCH/out.pdf" "$SCRATCH/pg"    # title page
pdftoppm -png -r 100 -f 60 -l 62 "$SCRATCH/out.pdf" "$SCRATCH/pg"  # body text
```

Pick pages deliberately: the cover, the title page, a text page, a page with an illustration, a page with a heading on it, and a list or table if the book has one. Then open each PNG with the `Read` tool and check:

- Text sits inside the margins and does not overflow the page
- Lines do not overlap, and paragraph spacing is even
- Justified lines have plausible word gaps rather than rivers of white
- Headings are bigger than the body text and are not stranded at the foot of a page
- Images are scaled to fit and are not stretched
- The running header and page number are in place when the preset asks for them
- Nothing is missing between one page and the next

## Step 4: Check the Text Layer

Rendering can look right while the text layer is broken, which ruins search and copy-paste:

```bash
pdftotext -f 60 -l 61 "$SCRATCH/out.pdf" -
pdfinfo "$SCRATCH/out.pdf"
```

Look for words glued together (`The Solution:Purchase`), missing spaces after list numbers (`7.Remove`), and lost punctuation. Those are markup or word-measurement problems, not drawing problems. `pdfinfo` should report the right title, author, page count and page size.

Bookmarks are stored as UTF-16 hex strings, so grep will not find them. Count and decode them like this:

```bash
PYTHONIOENCODING=utf-8 python -c "
import re, binascii, sys
data = open(sys.argv[1], 'rb').read()
titles = re.findall(rb'/Title\s*<([0-9A-Fa-f]+)>', data)
print(len(titles), 'outline entries')
for t in titles[:8]:
    print(' ', binascii.unhexlify(t).decode('utf-16-be', 'replace').lstrip('﻿'))
" "$SCRATCH/out.pdf"
```

## Step 5: Compare Before and After

For a change that is meant to alter layout, convert with the binary from `main` as well and put the same page side by side. Page counts that move by more than a percent or two without an intended reason mean something else changed.

## Step 6: Report

Tell the user what you actually saw, and send the rendered page images so they can judge for themselves. Say plainly if a check was skipped, for instance because poppler was missing. Never describe a page you did not open.

## Reference

- Markup and stylesheet handling: `internal/htmldoc/parse.go`, `internal/htmldoc/css.go`
- Layout and drawing: `internal/pdfout/render.go`
- Book parsing and format quirks: `internal/ebook/`, and the notes in AGENTS.md
- Sample books, PDFs and rendered pages are all gitignored, so keep them out of the repo
