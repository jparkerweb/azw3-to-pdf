# Plan Conversation: codebase-health
**Date:** 2026-10-24

Source: an X-ray rough-spots survey (`specs/codebase-health/ARCHITECTURE-BRIEF-20261024.md`) covering 7 structural rough spots across `internal/pdfout`, `internal/presets`, `internal/config`, `internal/cli`, `internal/tui`, `internal/engine`, `internal/htmldoc`. All 7 were picked to fix. The person also asked that this work land on a dedicated `develop` branch, created before implementation starts (a Git-skill step, out of scope for this plan itself).

## Decisions

| Phase | Decision | Why |
|---|---|---|
| 1 — Requirements | No additional files, reference materials, or external systems beyond what the brief names | The brief already covers every file; this is a pure internal refactor |
| 1 — Testing | Unit tests, run after each phase, critical-paths coverage, code first | One of the 7 fixes (#6) is literally "add missing tests"; every other package already has tests |
| 1 — Sign-off | Approved as drafted | FR-1 through FR-7 map 1:1 onto the brief's 7 spots |
| 2 — System context | Confirmed: `ebook`/`htmldoc`/`config`/`logging` are leaves, `pdfout→presets→engine` is the intended chain, `cli`/`tui` are the two entry points, no import cycles | Matches the earlier X-ray structural survey |
| 3 — Scope | Large scope recognized (7 components, 7 requirements, 4 internal integration points), but the person asked to keep it in one pass rather than pause for a new conversation | Avoids losing momentum on a session that's already fully specified |
| 4 — Tech stack | No new technology; stays on Go, Cobra, Bubble Tea, gopdf, YAML config, `testing` package | This is a refactor, not a rewrite |
| 5 — Architecture | Options composed via one `ApplyOverrides` method; batch logic unified into `engine.RunBatch`; font-scale unified into one `htmldoc.EffectiveScale`; `render.go` split by concern within the same package; TUI screens driven by one `Screen` interface instead of three switches | Matches the brief's root causes directly: each fix removes one place where the same idea was encoded more than once |
| 6 — Technical spec | 6 implementation phases (Options, batch runner, font scale, render split, screen registry, remaining tests), 7 risks, 8 success criteria | Grouped FR-1/FR-7 and part of FR-6 into the phases they naturally belong to, rather than one phase per brief item |
| 7 — Transition | Proceed to PLAN-DRAFT at 92% confidence | No open requirements gaps remained |
