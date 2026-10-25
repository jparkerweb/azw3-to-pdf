# AGENTS.md

Project documentation for AI coding agents working on `ebook-to-pdf`.

## What this is

A Go CLI that converts ebooks into PDFs: EPUB 2 and 3 (reflowable and
fixed-layout), Kindle (`.azw3`, `.azw`, `.mobi`, `.prc`), comic archives
(`.cbz`), FictionBook 2 (`.fb2`, `.fb2.zip`), plain text (`.txt`) and single
HTML files. Everything is in-process: every format reader, the layout engine
and the PDF writer. There is no dependency on Calibre or any other external tool, and
adding one would defeat the point of the project.

## How to use this file

This file is an index. Each section below gives the essentials and links to a
file in `.agents-docs/` with the detail; open that file when your task touches
its area. Default page size, Conventions and the Failure log stay here because
they apply to almost every change.

## Commands

Run `make ci` (lint, test, build) before a pull request; CI also runs the tests
with `-race`. Also covers every `make` target, the CI and release workflows,
the Go 1.25 pin, and the three environment variables (`EBOOK_TO_PDF_CONFIG`,
`EBOOK_TO_PDF_LOG_LEVEL`, `EBOOK_TO_PDF_NO_TUI`).

Details: [Commands](./.agents-docs/AGENTS-commands.md)

## Testing

Same-package tests built from in-memory fixtures; never `t.Parallel()`, since
tests swap package globals. The detail file lists the fixture builders, the
end-to-end and headless TUI tests, and how to run one package or one test.

Details: [Testing](./.agents-docs/AGENTS-testing.md)

## Architecture

A pipeline, one package per stage: `internal/ebook` (format readers to one
`Book`), `internal/htmldoc` (markup and CSS to layout blocks), `internal/pdfout`
(blocks to pages), `internal/engine` (jobs, batches, output naming), with
`internal/cli` choosing between `internal/tui` and headless mode. The detail
file holds the per-format gotchas (zip guard, HUFF/CDIC arithmetic, EPUB
self-closing tags, DRM refusal), the reader hooks and Kindle markup, the
renderer and TUI rules, a map of the supporting files, and cross-package
gotchas (`Normalize` after overrides, `--dry-run` side effects). Read it before
touching any reader, the parser or the renderer.

Details: [Architecture](./.agents-docs/AGENTS-architecture.md)

## Adding things

Checklists for every place that has to change together when adding a format,
page size, preset, layout setting or TUI screen.

Details: [Adding things](./.agents-docs/AGENTS-adding-things.md)

## Default page size

The default page size is US Letter (`presets.DefaultPageSize`) unless the user
saved another as `layout.page_size`, from the layout screen's `s` key or the
config file. A preset with `SizePreset` keeps its own size whatever is saved;
`--page-size` overrides both for one run without saving. Resolve it through
`Preset.PageSizeFor` (which `OptionsWithPageSize` and `SummaryFor` use), never
by reading `Preset.PageSize` directly.

## Local skills

`.claude/skills/` holds the repeatable workflows for this repo (release, update
docs, verify output, add preset). Prefer them over improvising the same steps.
A new skill file needs `git add -f`, because `.gitignore` matches `skills/`.

Details: [Local skills](./.agents-docs/AGENTS-local-skills.md)

## Releases

`CHANGELOG.md` follows Keep a Changelog, collecting under `[Unreleased]`;
anything a user would notice goes in it before the pull request merges. Cut a
release with the `ebook-to-pdf-release` skill: its `vX.Y.Z` tag drives
GoReleaser.

Details: [Releases](./.agents-docs/AGENTS-releases.md)

## Conventions

- Errors say what the user should do about them, and name the file involved.
- Comments explain why, not what. Format quirks deserve a comment; obvious Go
  does not.
- No em dashes in prose, comments or commit messages.
- Never add Claude Code attribution to commits unless the harness asks for it.

## Keeping this file current

The `Failure log` section below is a recording of mistakes made by previous AI Agents while working with this code base.

When you make a mistake, get corrected, or discover something about this codebase that wasn't written down:

1. Add one line to the `Failure log` below, in the imperative, describing the correct behaviour.
2. Keep it specific to this repo. General advice belongs nowhere.
3. If this fix is a workflow rather than a rule, put it in `.claude/skills/` and link it from here.
4. Include the change in the same commit and mention it in your summary.
5. A rule broken twice becomes an automated check (a test, a lint rule or a script), not more prose.
6. Before adding a line, look for one on the same topic: sharpen or extend that one rather than adding a near-duplicate.
7. Keep the log to 17 entries at most. Over the cap, first merge entries that share a cause, then drop the lowest-value ones: one-offs unlikely to recur, rules a test or script now enforces, and entries about code that no longer exists.

## Failure log

- When adding a format, update the splash line in `internal/tui/screens/splash.go` and the help text in `internal/cli/root.go` too: both went stale when EPUB, CBZ, FB2, text and HTML landed.
- When adding an environment variable, list it in README.md and `.agents-docs/AGENTS-commands.md` in the same change: `EBOOK_TO_PDF_NO_TUI` was missed in both.
