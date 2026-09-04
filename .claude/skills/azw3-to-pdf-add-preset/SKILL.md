---
name: azw3-to-pdf-add-preset
description: Add or change a layout preset or page size, touching every place that has to agree (registry, tests, README table, changelog). Use when the user says "add a preset", "new preset", "add a page size", "change the ereader preset", or describes a layout they want available by name. Also use when the user types "/azw3-to-pdf-add-preset".
user-invocable: true
---

# Add a Preset or Page Size

A preset is one entry in a registry plus the places that describe it. Missing one of those places is how the README ends up lying about the tool.

## Decide Which One

A **preset** is a whole layout: page size, margins, typeface, size, spacing and which extras are on. Add one when the user wants a named starting point.

A **page size** is only paper dimensions. Add one when the user wants a size that presets can then use. Note that a size is often unnecessary, because `--page-size 120x160mm`, `6x9in` and `432x648pt` already work through `parseCustomSize`.

## Adding a Page Size

1. Add an entry to the `pageSizes` map in `internal/pdfout/options.go`. The key, the `Name` field and the map key must be the same string. Dimensions are in points, 72 to the inch.
2. Add a row to the page sizes table in `README.md`.
3. Nothing else: `PageSizeNames`, `PageSizes`, the shell completions and the layout screen all read from the map.

## Adding a Preset

### Step 1: The registry

Append to the `registry` slice in `internal/presets/presets.go`. Order matters, because it is the order shown in the interface and by `azw3-to-pdf presets`. Fill in every field:

| Field | Notes |
|-------|-------|
| `Key` | Lower case, hyphenated, used on the command line |
| `Name` | Title case, shown in the interface |
| `Description` | One sentence saying who it is for, not what it sets |
| `PageSize` | A key from the `pageSizes` map, or a measurement string |
| `MarginMM` | Millimetres, converted for you |
| `Font` | `serif`, `sans` or `mono` |
| `FontSize` | Points |
| `LineSpacing` | A multiple of the font size, roughly 1.2 to 1.6 for reading |
| `Justify` | True for a printed-book feel, false for screens and large print |
| `TitlePage`, `PageNumbers`, `RunningHeader` | The extras |

### Step 2: Sanity check the geometry

`TestAllPresetsProduceValidOptions` in `internal/presets/presets_test.go` already enforces the basics: a real page size, a readable font size, and at least 100 points of text column. It runs for every preset, so a new one is covered without new test code.

Check the line length by hand as well. A text column should hold roughly 45 to 75 characters:

```
characters ≈ (page width − left margin − right margin) / (font size × 0.5)
```

The layout screen prints this figure, so `go run ./cmd/azw3-to-pdf` and press `l` to see it.

### Step 3: The recommendation engine

If the preset is meant to be suggested automatically, add a case to `Recommend` in the same file and a case to `TestRecommend`. Leave it alone otherwise: recommendations should stay few and obvious.

### Step 4: Documentation

- Add a row to the presets table in `README.md`, describing the layout the way the table's other rows do
- Add a line to the `[Unreleased]` section of `CHANGELOG.md` under `### Added`

Nothing else references presets by name. The command line help, the shell completions, the `presets` subcommand and the interface all read the registry.

### Step 5: Prove it

```bash
make ci
go run ./cmd/azw3-to-pdf presets
go run ./cmd/azw3-to-pdf /path/to/book.azw3 --no-tui --preset <key> -o out.pdf --overwrite
```

Then look at the pages: use the `azw3-to-pdf-verify-output` skill. A preset that has never been rendered is a guess, not a preset.

## Changing an Existing Preset

Same steps, with one extra consideration: changing a preset changes the output for everyone who uses it, including the default. Say so to the user, note it in the changelog under `### Changed` rather than `### Added`, and treat a large change to the default preset as a minor version bump rather than a patch.

## Reference

- Registry and recommendation: `internal/presets/presets.go`
- Page sizes and option clamping: `internal/pdfout/options.go`
- Where a preset ends up: `Preset.Options()` produces a `pdfout.Options`, which the command line and the interface then override
