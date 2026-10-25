# Phase 1: Rename and default page size

**Status:** Complete
**Estimated Tasks:** 14

## Overview

The tool becomes **ebook-to-pdf** everywhere: module path, binary, `cmd/`
folder, config and cache folders, environment variables, release and CI
configuration, local skills and the promo page. Separately, the default page
size becomes a saved setting that is US Letter when nothing is saved, with a
"Save as default" action on the TUI layout screen. No new formats yet.

## Prerequisites

- The codebase-health refactor on `refactor/duplication` is committed (the rename touches every file it changed) VERIFIED (committed as 70ddd50 at the person's request)
- `make ci` is green on that commit VERIFIED (golangci-lint 0 issues, go test ./... ok, build ok; make is not installed on this machine, so its three steps were run directly)
- Covers FR-9 and FR-11 of the plan VERIFIED

## Tasks

### Rename: code

- [x] **Task 1.1:** Change the module path in `go.mod` from `github.com/jparkerweb/azw3-to-pdf` to `github.com/jparkerweb/ebook-to-pdf`, then rewrite every import in every `.go` file (tests included) to the new path. A mechanical replace of the string `github.com/jparkerweb/azw3-to-pdf/` is enough. Run `go build ./...` and `go vet ./...`; both must pass. Do not touch `go.sum`.

- [x] **Task 1.2:** Move `cmd/azw3-to-pdf/` to `cmd/ebook-to-pdf/` with `git mv` so history follows. In `Makefile`, set `BINARY_NAME := ebook-to-pdf` and point the `build` and `install` targets at `./cmd/ebook-to-pdf/`. In `.gitignore`, replace `/azw3-to-pdf` and `/azw3-to-pdf.exe` with `/ebook-to-pdf` and `/ebook-to-pdf.exe`.

- [x] **Task 1.3:** Rename the settings locations. In `internal/config/paths.go` set `appName = "ebook-to-pdf"` and `envConfigDir = "EBOOK_TO_PDF_CONFIG"` (update the comment). In `internal/logging/logging.go` set `envLogLevel = "EBOOK_TO_PDF_LOG_LEVEL"` and the log file name to `ebook-to-pdf.log`. Do not add any code that reads or migrates the old `azw3-to-pdf` folder or variables (out of scope by decision). `internal/config/config_test.go` already uses the `envConfigDir` constant, so it needs no change; confirm it passes.

- [x] **Task 1.4:** Rename what the conversion writes. In `engine.Convert` (`internal/engine/convert.go`) change the temporary file pattern from `.azw3topdf-*.tmp` to `.ebooktopdf-*.tmp`. In `pdfout.Render` (`internal/pdfout/render.go`) set `PdfInfo.Creator` and `PdfInfo.Producer` to `"ebook-to-pdf"`. Update any assertion on these strings in `internal/pdfout/pdfout_test.go` and `internal/engine/convert_test.go`.

- [x] **Task 1.5:** Rename the CLI's user-facing text. In `internal/cli/root.go` set the command `Use` to `ebook-to-pdf` and change the `Long` text's opening to "ebook-to-pdf converts Kindle books (.azw3, .azw, .mobi, .prc) into PDFs." (the format list grows in Phase 2, Task 2.18). In `internal/cli/version.go` print `ebook-to-pdf %s (commit: ...)`. Sweep `internal/cli/*.go` (`completion.go`, `flags.go`, `headless.go`, `presets.go`, `probe.go`, `run.go`) for the old name in help strings, examples and messages and replace it. Update `internal/cli/flags_test.go` and `internal/cli/probe_test.go` expectations.
  > SPEC NOTE: `internal/cli/run.go` also read an undocumented `AZW3_TO_PDF_NO_TUI` variable; renamed to `EBOOK_TO_PDF_NO_TUI` so the Task 1.11 sweep is clean. Neither CLI test file asserted the old name, so they needed no change.

- [x] **Task 1.6:** Rename the TUI's text. In `App`'s header rendering (`internal/tui/app.go`, the `style.HeaderStyle()` line that prints the version) print ` ebook-to-pdf %s `. Update the splash screen (`internal/tui/screens/splash.go`) title art or wordmark if it spells the old name; keep the subtitle as is for now (Phase 2 broadens it). Sweep `internal/tui/` for any other occurrence of `azw3-to-pdf` in strings and update `internal/tui/app_test.go` and `internal/tui/screens/batch_test.go` expectations.

### Rename: build, release and docs

- [x] **Task 1.7:** Update `.goreleaser.yaml`: `project_name: ebook-to-pdf`, build `id: ebook-to-pdf`, `main: ./cmd/ebook-to-pdf/`, `binary: ebook-to-pdf`, plus any other occurrence (archive names derive from `ProjectName`). In `.github/workflows/ci.yml` change the build step to `go build -o ebook-to-pdf ./cmd/ebook-to-pdf/` and the smoke step to `./ebook-to-pdf version`. Check `.github/workflows/release.yml` for the old name and update it. Run `goreleaser check` if goreleaser is installed; otherwise `make snapshot` is not required.
  > SPEC NOTE: `release.github.name` in `.goreleaser.yaml` also moved to `ebook-to-pdf`, so a release publishes to the renamed repo (Task 4.19). `release.yml` named neither the binary nor the folder. `goreleaser check` passes.

- [x] **Task 1.8:** Rename the local skills. `git mv` each of `.claude/skills/azw3-to-pdf-add-preset`, `-release`, `-update-docs` and `-verify-output` to `.claude/skills/ebook-to-pdf-*`. In each `SKILL.md` update the `name:` frontmatter, the `/azw3-to-pdf-*` trigger text, binary invocations (`./azw3-to-pdf ...` becomes `./ebook-to-pdf ...`) and paths. Update the Local skills table and every command or path in `AGENTS.md` that names the old binary, `cmd/` folder or skills (the full AGENTS.md rewrite for formats is Task 4.16; this task only renames). Leave `CLAUDE.md` alone unless it names the old tool.

- [x] **Task 1.9:** Move the promo page to the new domain. Set `docs/CNAME` to `ebook-to-pdf.equilllabs.com`. `git mv docs/azw3-to-pdf.jpg docs/ebook-to-pdf.jpg`. In `docs/index.html` replace every `azw3-to-pdf` (title, canonical URL, `og:*` tags now on `https://ebook-to-pdf.equilllabs.com/`, GitHub links to `github.com/jparkerweb/ebook-to-pdf`, the `go install github.com/jparkerweb/ebook-to-pdf/cmd/ebook-to-pdf@latest` lines, terminal examples, image `src`). Keep the page's Kindle-only copy for now; Task 4.17 adds the new formats.

- [x] **Task 1.10:** Rename `README.md`: title, install commands (`go install github.com/jparkerweb/ebook-to-pdf/cmd/ebook-to-pdf@latest`), release download links, badges, every usage example and the config folder and `EBOOK_TO_PDF_CONFIG` / `EBOOK_TO_PDF_LOG_LEVEL` references. Add one sentence near the top saying the project was renamed from azw3-to-pdf and that old settings are not carried over.
  > SPEC NOTE: the README banner `azw3-to-pdf.jpg` at the repo root was renamed to `ebook-to-pdf.jpg` (and its `src` updated) so the sweep finds no hit. The README never named the config or log-level variables, so there was nothing to rename there.

- [x] **Task 1.11:** Run the rename sweep: `git grep -nI -e "azw3-to-pdf" -e "azw3topdf" -e "AZW3_TO_PDF"`. Every remaining hit must be either a past `CHANGELOG.md` entry or compare link, or a file under `specs/` or `specs--completed/`. File extensions such as `.azw3` are not hits. Then run `make ci` and confirm the built binary is `./ebook-to-pdf` and `./ebook-to-pdf version` prints the new name.
  > SPEC NOTE: the sweep leaves the three CHANGELOG compare links and one README line, the "renamed from azw3-to-pdf" sentence that Task 1.10 asks for. `make` is not installed here, so `make ci` was run as its three steps: golangci-lint (0 issues), `go test ./...` (all ok) and `CGO_ENABLED=0 go build -o ebook-to-pdf.exe ./cmd/ebook-to-pdf/`; `ebook-to-pdf version` prints `ebook-to-pdf dev` and the `ebook-to-pdf` config and log folders.

### Default page size

- [x] **Task 1.12:** Teach presets about the saved default in `internal/presets/presets.go`. Add a `SizePreset bool` field to `Preset` (doc comment: the preset's page size is part of its identity and wins over the saved default). Set `SizePreset: true` on `phone`, `paperback` and `a4` only. Add `const DefaultPageSize = "letter"`. Add `func (p Preset) PageSizeFor(saved string) string` returning `p.PageSize` for size presets, otherwise `saved` when non-empty, otherwise `DefaultPageSize`. Add `func (p Preset) OptionsWithPageSize(saved string) (pdfout.Options, error)` that builds options exactly like today's `Options()` but looks the size up with `PageSizeFor(saved)`; make `Options()` call `OptionsWithPageSize("")`. Make `Summary()` describe the size from `PageSizeFor("")` so E-reader no longer advertises A5. In `internal/pdfout/options.go`, make `LookupPageSize("")` and `DefaultOptions()` use `pageSizes["letter"]` instead of `a5`.
  - Test in `internal/presets` (create `presets_test.go` if missing): E-reader with no saved size gives 612 x 792; E-reader with `"a5"` gives A5; Phone with `"a5"` keeps the `kindle` size; Paperback keeps `trade`; A4 keeps `a4`; Large print with nothing saved gives Letter.
  > SPEC NOTE: `Options.Normalize` also fell back to A5 for a zero page size; it now falls back to Letter too, so no path still defaults to A5. The existing `LookupPageSize("")` assertion in `pdfout_test.go` now expects Letter.

- [x] **Task 1.13:** Apply the saved default in the CLI. In `buildPDFOptions` (`internal/cli/flags.go`) build the base options with `preset.OptionsWithPageSize(cfg.Layout.PageSize)` (empty string when `cfg` is nil). Remove the `PageSize` handling from `configOverrides`, since the size now enters through the preset (leaving it there would let `layout.page_size` override the size presets). Keep validating `layout.page_size`: if `pdfout.LookupPageSize` fails, return the error naming the config file (`cfg.Path()`) and the bad value. `--page-size` still overrides everything through `flagOverrides`.
  - Test in `internal/cli/flags_test.go`: no config gives Letter for E-reader; `layout.page_size: a5` gives A5 for E-reader and keeps `kindle` for Phone; `--page-size a6` with Phone gives A6; an unknown `layout.page_size` returns an error that names the value.

- [x] **Task 1.14:** Apply the saved default in the TUI and add "Save as default". In `App.Update`'s `messages.PresetSelectedMsg` case (`internal/tui/app.go`) build options with `msg.Preset.OptionsWithPageSize(saved)`, where `saved` is `a.cfg.Layout.PageSize` when `a.cfg` is non-nil. Check the initial options passed in from `runTUI` (`internal/cli/run.go`) already come from `buildPDFOptions`, so they follow Task 1.13. Add `messages.SaveDefaultPageSizeMsg{Size pdfout.PageSize}` in `internal/tui/messages/messages.go`. In `LayoutModel.Update` (`internal/tui/screens/layout.go`) bind `s` to send it with `m.opts.PageSize`; add `{"s", "Save page size as default"}` to `Bindings()` and `s: save size as default` to `FooterHint()`. In `App.Update` handle the message: write `layout.page_size` (the built-in `Name` for named sizes; for a custom size write `fmt.Sprintf("%gx%gpt", w, h)` so an inch or millimetre measurement round-trips exactly), call `a.cfg.Save()`, and set `a.status` to `"Default page size saved: <Label>"`, or to the save error, which names the config path. When the current preset is a size preset, append " (Phone, Paperback and A4 keep their own size)" to the status.
  - Test in `internal/tui/app_test.go`: with a config pointed at `t.TempDir()` through `EBOOK_TO_PDF_CONFIG`, pressing `s` on the layout screen writes `page_size` to `config.yaml`, and a custom 6x9in size round-trips to 432 x 648 points.
  > VERIFIED: `runTUI` builds its initial options with `buildPDFOptions` (internal/cli/run.go), so they follow Task 1.13. With no config loaded (`a.cfg` nil) the `s` key reports that nothing was saved rather than failing.

## Acceptance Criteria

- `go build ./...`, `make ci` pass; the binary is `ebook-to-pdf` and `ebook-to-pdf version` prints the new name
- `git grep` for the old name finds only changelog history and spec folders
- Config is read from the `ebook-to-pdf` folder and `EBOOK_TO_PDF_CONFIG`; nothing reads the old folder
- PDF Creator and Producer say `ebook-to-pdf`
- A fresh config renders E-reader on US Letter; Phone, Paperback and A4 keep their sizes; `--page-size` still wins for one run
- Pressing `s` on the TUI layout screen persists `layout.page_size` and later runs use it
- `go.mod` gains no modules

## Notes

- Only Phone, Paperback and A4 are size presets, by decision in the planning conversation. Print and Manuscript already use Letter; Large print (A4 today), E-reader and Compact (A5 today) all follow the saved default, so Large print moves from A4 to Letter on a fresh install. The changelog entry in Task 4.18 must say so.
- The GitHub repo rename, DNS record and git remote change are the owner's manual steps (Task 4.19). Until the repo is renamed, `go install` with the new module path fails; do not tag a release between this phase and that rename.
- `LookupPageSize` stores a custom size's `Name` without its unit (e.g. `6x9`), which would read back as millimetres; that is why Task 1.14 writes custom sizes in points.

## Phase Completion Summary

**Date:** 2026-10-25
**Implementer:** Claude (Implement + Review)

**What was done:** The tool is now ebook-to-pdf throughout: module path and every import, `cmd/ebook-to-pdf/`, binary name in the Makefile, `.gitignore`, GoReleaser and CI, config folder and `EBOOK_TO_PDF_CONFIG` / `EBOOK_TO_PDF_LOG_LEVEL` / `EBOOK_TO_PDF_NO_TUI`, PDF Creator and Producer, temp file pattern, CLI help, TUI header and splash wordmark, local skills, AGENTS.md, README and the promo page (now on ebook-to-pdf.equilllabs.com). The default page size is US Letter when nothing is saved: presets gained `SizePreset` (Phone, Paperback, A4), `DefaultPageSize`, `PageSizeFor` and `OptionsWithPageSize`; the CLI feeds `layout.page_size` in through the preset and still validates it, naming the config file; the TUI layout screen saves the current size as the default with `s`, writing custom sizes in points.

**Checks:** golangci-lint 0 issues; `go test ./...` all packages ok; `CGO_ENABLED=0 go build ./cmd/ebook-to-pdf/` ok and `ebook-to-pdf version` prints the new name and folders; `ebook-to-pdf presets` lists E-reader, Compact and Large print on US Letter; `goreleaser check` passes; the rename sweep leaves only CHANGELOG compare links and the README's "renamed from" sentence.

**Files changed:** go.mod, Makefile, .gitignore, .goreleaser.yaml, .github/workflows/ci.yml, cmd/ebook-to-pdf/main.go (moved), every Go file's imports, internal/config/paths.go, internal/logging/logging.go, internal/engine/convert.go, internal/pdfout/render.go, internal/pdfout/options.go, internal/pdfout/pdfout_test.go, internal/cli/root.go, version.go, run.go, flags.go, flags_test.go, internal/presets/presets.go, presets_test.go, internal/tui/app.go, app_test.go, messages/messages.go, screens/layout.go, screens/splash.go, .claude/skills/ebook-to-pdf-* (moved), AGENTS.md, README.md, ebook-to-pdf.jpg (moved), docs/CNAME, docs/index.html, docs/ebook-to-pdf.jpg (moved).

**Issues:** None blocking. `make` is not installed on this machine, so `make ci` ran as its three steps. A stale local `azw3-to-pdf.exe` (v0.1.0 build, never tracked) stopped being gitignored by the rename and was deleted so a commit-all would not pick it up. The E-reader description still says "Tablet-sized pages" although it now renders on Letter; wording is left for the Phase 4 docs pass.

**Review fixes (2026-10-25):** all four review findings fixed. (1) The promo page brand, hero wordmark, `aria-label` and theme storage key now say ebook-to-pdf. (2) The layout footer hint is shortened to `s: save size`, and the size-preset status suffix to "(not used by Phone, Paperback, A4)", so both fit in 80 columns. (3) Saving a measured size reports the value written (e.g. `432x648pt`) rather than "custom". (4) `Preset.SummaryFor(saved)` lets the TUI preset list, the batch queue and `--dry-run` describe the size a preset actually renders at; `Summary()` (the `presets` command) still describes the defaults. New shared state field `style.SharedState.SavedPageSize`. Lint, `go test ./...` and the build pass again after the fixes.
