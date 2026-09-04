---
name: azw3-to-pdf-update-docs
description: Audit and update all project documentation (README.md, AGENTS.md, CHANGELOG.md and the local skills) so it matches the current codebase. Use when the user says "update docs", "sync docs", "audit docs", "docs are stale", "refresh documentation", or after significant code changes. Also use when the user types "/azw3-to-pdf-update-docs".
user-invocable: true
---

# Documentation Audit and Update

This skill checks every documentation claim against the code that backs it and fixes what has drifted. It uses parallel subagents so the whole audit costs one round trip.

## Files to Audit

| File | Checked against |
|------|-----------------|
| `README.md` | CLI flags, presets table, page sizes, config keys, font behaviour, format support |
| `AGENTS.md` | Package layout, format notes, build and CI commands, the "adding things" recipes |
| `CHANGELOG.md` | Whether user-visible changes since the last tag are recorded |
| `.claude/skills/*/SKILL.md` | Commands and file paths the skills tell an agent to use |

## Execution

Launch **five parallel Explore subagents**, one per audit area. Each reads both the documentation and the source that backs it, then reports every discrepancy it finds. Do not have them edit anything: they report, you fix.

### Agent 1: README, command line

Compare the README's usage examples and flag mentions against `registerRunFlags` in `internal/cli/flags.go` and the persistent flags in `internal/cli/root.go`. Report flags that exist but are undocumented, flags documented but not implemented, wrong defaults, and subcommands in `internal/cli/` that the README never mentions (`probe`, `presets`, `page-sizes`, `completion`, `version`).

### Agent 2: README, presets and page sizes

Compare the README presets table against the `registry` slice in `internal/presets/presets.go`: keys, names, page size, font, size, margins and justification must match `Preset.Summary()`. Compare the page size claims against the `pageSizes` map in `internal/pdfout/options.go`, including the count and the custom measurement suffixes accepted by `parseCustomSize` (`mm`, `in`, `pt`).

### Agent 3: README, configuration and fonts

Compare the README's YAML example against the struct tags in `internal/config/config.go`. Every key shown must exist with that exact `yaml:` tag, and the nesting must match. Then compare the font section against `internal/pdfout/fonts.go`: the candidate family names actually searched, the platform directories, which container formats `usableTTF` rejects, and what the embedded fallback is.

### Agent 4: AGENTS.md

Check every factual claim: the package table against the directories under `internal/`, the pipeline description against what each package really does, the format notes in the `internal/ebook` section against `huffcdic.go`, `text.go` and `book.go`, the make targets against the `Makefile`, and the workflow description against `.github/workflows/`. Verify the Go version claim against `go.mod` and both workflow files, which must agree.

### Agent 5: Changelog and skills

Run `git log $(git describe --tags --abbrev=0 2>/dev/null || echo --root)..HEAD --oneline` and report user-visible changes that are missing from the `[Unreleased]` section of `CHANGELOG.md`. Separately, read every `.claude/skills/*/SKILL.md` and verify the commands and file paths they name still exist.

## After the Agents Report

1. **Compile the findings** into one list grouped by file
2. **Apply the fixes**, preferring `Edit` for targeted changes and `Write` only for a full rewrite
3. **Match the existing conventions**: table styles, heading levels, sentence case, and no em dashes anywhere
4. **Add missing changelog entries** under `[Unreleased]`, written from the reader's point of view
5. **Show the user a summary** of the changes, grouped by file

## Where This Project Drifts

Pay extra attention to these, which are the usual culprits:

- **New flags** added to `internal/cli/flags.go` but never added to the README
- **New presets or page sizes** added to the registry but missing from the README tables
- **Config keys** renamed in `internal/config/config.go` while the README example keeps the old ones
- **The Go version** in `go.mod` diverging from the pin in both workflow files
- **Package descriptions** in AGENTS.md going stale when a file moves between packages
- **Format notes** in AGENTS.md that no longer match the parser, which is worse than no note at all because someone will trust them
- **Screens** added to `internal/tui/screens/` without footer hints, help bindings or a mention in the interface key table

## Validation

After the edits, run `make ci`. Documentation changes should never break it, so a failure means something else was touched by accident. If a doc fix revealed that the *code* is wrong rather than the docs, say so plainly and ask the user which side should change.
