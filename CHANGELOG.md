# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.0] - 2026-09-04

### Added
- Initial release
- Kindle book reader with no external dependencies — the MOBI/KF8 parser, layout engine and PDF writer are all built in, so no Calibre, Python or Ghostscript is needed
- Reads `.azw3`, `.azw`, `.mobi` and `.prc`: Palm database containers holding KF8 (MOBI 8) or MOBI 6, including dual-format files where both halves are present
- PalmDOC (LZ77) and HUFF/CDIC decompression, UTF-8 and Windows-1252 text, and JPEG, PNG, GIF and BMP images
- Interactive TUI wizard (splash → file picker → book details → presets → layout → preview → converting → complete)
- Built-in file browser with multi-select via Space, `b` to convert the selection, `a` to add every book in a folder, `tab` for typed paths, `d` for drive switching on Windows and `h` to jump home
- Batch processing with a queue screen, per-book progress and a summary of what was written
- Eight layout presets (`ereader`, `paperback`, `print`, `a4`, `compact`, `large-print`, `phone`, `manuscript`) and a recommendation based on the book's length and illustration count
- Layout screen for tuning page size, margins, typeface, text size, line spacing, justification, illustrations, cover, title page, page numbers, running header, bookmarks and chapter breaks
- Twelve built-in page sizes plus explicit measurements: `120x160mm`, `6x9in` or `432x648pt`
- Stylesheet-aware layout — the book's own CSS drives headings, alignment, relative font sizes, hanging indents and page breaks, which matters because Kindle books style headings with classes rather than heading tags
- PDF output with selectable text, outline bookmarks built from the book's headings, page numbers, an optional running header, the cover art, a generated title page and document metadata
- Font resolution at render time: a system serif, sans or monospaced TrueType face where one exists, a `.ttf` path of your own, or the Go typefaces embedded in the binary as a fallback that cannot fail
- Headless mode (`--no-tui`) with parallel conversion (`--jobs`), recursive folder search (`--recursive`), path input on stdin (`--stdin`), `--dry-run`, and `--overwrite`, `--auto-rename` and `--skip-existing` for existing files
- `probe` command for inspecting a book's metadata, format, compression, resources and headings without converting it
- `presets` and `page-sizes` commands, plus shell completions for bash, zsh, fish and PowerShell
- YAML configuration file for the default preset, output directory, layout overrides and theme, layered under command-line flags
- Two colour themes ("Midnight Ink" and "Paper Sepia") switchable at runtime with `ctrl+t` and remembered between runs
- Help overlay (`?`) with context-sensitive shortcuts on every screen
- Cross-platform support (Windows, macOS, Linux) on amd64 and arm64
- GoReleaser build pipeline with GitHub Actions CI/CD

### Notes
- DRM is not removed. An encrypted book reports that clearly instead of producing a PDF of noise.
- `.kfx`, the newest Kindle format, is a different container and is not supported.

[Unreleased]: https://github.com/jparkerweb/azw3-to-pdf/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/jparkerweb/azw3-to-pdf/releases/tag/v0.1.0
