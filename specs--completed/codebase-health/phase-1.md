# Phase 1: Options composition
**Status:** Complete
**Estimated Tasks:** 10

## Overview

Today a layout knob is encoded in seven places (`pdfout`, `presets`, `config`, `cli/flags`, `tui/app`, `tui/screens/layout`, `tui/messages`). This phase adds one `Options.ApplyOverrides` method and migrates each consumer to it, so a new knob needs one change. Covers FR-1 and FR-7.

## Prerequisites

- Work is on the `develop` branch (created by the Git skill before this phase; verify with `git branch --show-current`)
- `go test ./...` passes on the starting commit

## Tasks

### Setup

- [x] **Task 1.1:** Confirm the current branch is `develop`; if not, stop and ask the person to create it before any commit.
  - Run `git branch --show-current`.

### Core method

- [x] **Task 1.2:** Decide and record the "explicitly set" mechanism for `Options` in `internal/pdfout/options.go`.
  - Read the `Options` struct and list each field with its type and zero-value meaning (bool, int, float, string).
  - Pick one mechanism: pointer fields on a separate partial type, or a `Set` bitmask/struct alongside `Options`. Prefer a separate `OptionsOverrides` partial type with pointer fields if it keeps `Options` itself unchanged for existing callers.
  - Write the decision as a short doc comment above the new type, explaining why zero is ambiguous.
- [x] **Task 1.3:** Add `Options.ApplyOverrides` in `internal/pdfout/options.go`.
  - Signature follows the plan (`func (o Options) ApplyOverrides(partial ...) Options`), adjusted to the mechanism chosen in Task 1.2.
  - Returns a new value, never mutates the receiver. An unset field in `partial` leaves the base value; a set field wins even when it is the zero value.
  - Cover every field in `Options`; add a test (Task 1.4) that fails when a field is added to `Options` without being handled here (for example, reflect over the struct fields and assert each is covered).
- [x] **Task 1.4:** Add table-driven tests in `internal/pdfout/pdfout_test.go` (or a new `options_test.go`).
  - One case per field type: bool set to false over true, int set to 0 over non-zero, string set to empty, unset field keeps base, set field wins.
  - A test that the receiver is not mutated.
  - The completeness test from Task 1.3.

### Migrate consumers

- [x] **Task 1.5:** Migrate `Preset.Options()` in `internal/presets/presets.go` to build its result through `ApplyOverrides` on the default options.
  - Existing `presets_test.go` must still pass unchanged.
- [x] **Task 1.6:** Migrate the `LayoutConfig` application in `internal/config/config.go` to produce a partial and apply it with `ApplyOverrides`.
  - Existing `config_test.go` must still pass unchanged. Keep `internal/config` free of a new import cycle: it is a leaf in the layering, so if `config` cannot import `pdfout`, expose the partial from `config` and apply it in the caller instead.
- [x] **Task 1.7:** Migrate `buildPDFOptions` in `internal/cli/flags.go` to the same path, replacing its copy of the preset-to-Options mapping (FR-7).
  - Precedence must stay: defaults, then preset, then config file, then explicit flags. Only flags the user actually passed (`cmd.Flags().Changed`) count as set.
- [x] **Task 1.8:** Migrate `applyLayout` in `internal/tui/app.go`, the `layoutFields` table in `internal/tui/screens/layout.go`, and `LayoutChangedMsg` in `internal/tui/messages/messages.go` to the same path.
  - `LayoutChangedMsg` should carry the partial (or the final `Options`) so a new knob does not require a new message field.
  - Do not change any screen's visible behavior or key bindings.

## Phase Testing

- [x] **Task 1.9:** Run `go build ./...`, `go vet ./...` and `go test ./...`; all must pass.
- [x] **Task 1.10:** Prove equivalence: write a throwaway test or script that, for every preset in the `registry` slice, compares the `Options` produced before this phase (checkout the previous commit into a temp worktree) with the `Options` produced now, and confirm they are identical. Delete the throwaway afterwards, or keep it as a permanent test if it is cheap.

## Acceptance Criteria

- Exactly one function merges layout overrides onto `Options`
- `presets`, `config`, `cli/flags`, `tui/app` and the layout screen all use it
- Every preset yields the same `Options` values as before
- Precedence of defaults, preset, config, flags is unchanged
- `go build ./...`, `go vet ./...` and `go test ./...` pass

## Notes

- Do not add any new layout option here (NFR-2).
- The ambiguity of zero vs unset is the main risk; the per-field-type tests are the mitigation.

## Phase Completion Summary

**Date:** 2026-10-24
**Implementer:** Claude (Implement + Review)

**What was done**

- `internal/pdfout/options.go`: added `OptionsOverrides` (one pointer field per `Options` field), `Options.ApplyOverrides`, `OverridesFrom` (a full `Options` as overrides) and the `Ptr` helper. Decision for Task 1.2: a separate partial type with pointer fields, so `Options` itself is unchanged for existing callers. The doc comment explains why zero is ambiguous.
- `internal/pdfout/options_test.go`: per-field-type cases, unset-keeps-base, receiver-not-mutated, and a reflection test that fails when an `Options` field is missing from `OptionsOverrides`, `ApplyOverrides` or `OverridesFrom`.
- `presets.Preset.Options()`, `cli.buildPDFOptions` (now `configOverrides` + `flagOverrides` + `ApplyOverrides`), `tui.App.applyLayout` and `LayoutModel.apply` all go through the one path. `LayoutChangedMsg` now carries `Overrides pdfout.OptionsOverrides`.
- Precedence is unchanged: defaults, preset, config file, then flags.

**Verification**

- `go build ./...`, `go vet ./...`, `go test ./...` and `golangci-lint run ./...` all pass.
- Equivalence (Task 1.10): a throwaway test dumped `Options` as JSON for every preset, plus `buildPDFOptions` for each preset under four scenarios (no config or flags, config only, flags only, config plus flags), in a temp worktree of the previous commit and in the working tree. The two outputs are byte-identical (21812 bytes each). The test and the worktree were deleted afterwards.

**Notes and deviations**

> SPEC NOTE: Task 1.1 asked for a `develop` branch. The person chose to stay on the current branch, `refactor/duplication`, created from `origin/main`.
> SPEC NOTE: Task 1.6 names `internal/config/config.go`, but the config-to-`Options` application lived in `cli/flags.go`. `config` stays a leaf; the conversion is `configOverrides` in `cli/flags.go`.
> SPEC NOTE: the `layoutFields` table in `tui/screens/layout.go` is one row per knob for display and editing, so it was left alone. Only the screen's `apply()` moved to the shared path.
> SPEC NOTE: `LayoutChangedMsg` used to carry the page size by name and re-look it up. It now carries the `PageSize` itself, which also keeps a custom size's unit (the old round trip read a `6x9in` size back as millimetres).
> SPEC NOTE: `internal/tui/app_test.go` changed (not listed as must-stay-unchanged): `TestLayoutChangeReachesOptions` builds the new message shape; its assertions are the same.

**Review (focused, by a fresh helper)**

5 findings, all confirmed and fixed: AGENTS.md "Adding things" updated to the new recipe; the completeness test now uses distinct values and checks `OverridesFrom` one field at a time; `fillNonZero` panics on unknown kinds; new `internal/cli/flags_test.go` covers config/flag precedence and zero-vs-unset; `LayoutModel.apply` documents why a full snapshot is safe. Build, vet, tests and golangci-lint pass after the fixes.
