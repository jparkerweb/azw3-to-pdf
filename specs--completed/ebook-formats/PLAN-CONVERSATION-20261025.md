# Planning conversation: ebook-formats

**Date:** 2026-10-25
**Plan:** [PLAN-DRAFT-20261025.md](./PLAN-DRAFT-20261025.md)
**Channel:** web console, launched from the Plan2Code dashboard

## Decisions

| # | Question | Answer |
|---|----------|--------|
| 1 | The idea | Add EPUB and MOBI so the project becomes a general ebook to PDF tool |
| 2 | What MOBI means | Already supported; the person wants other popular formats instead |
| 3 | Formats to add | EPUB, CBZ, FB2 (and `.fb2.zip`), TXT, HTML (not KFX) |
| 4 | Project name | Rename in this plan, to `ebook-to-pdf` |
| 5 | Which EPUBs | EPUB 3 and 2 reflowable, and fixed-layout |
| 6 | DRM-locked EPUBs | Person asked for DRM removal for books they own; declined (circumvention, and the project refuses DRM). Agreed: refuse with a clear error and pointers to DRM-free copies |
| 7 | Fixed-layout fidelity | One book page per PDF page; full-page images exact, positioned text reflowed |
| 8 | Existing settings | Start fresh, no migration |
| 9 | Promo site | Move to `ebook-to-pdf.equilllabs.com` (owner adds DNS) |
| 10 | Testing | Unit tests only, critical paths, code first |
| 11 | Requirements recap | Confirmed |
| 12 | Folder scans and .txt/.html | Only when named directly; `--include-text` for scans |
| 13 | Comic page size | Sized to each image. Note: make the default page size configurable, Letter for a fresh install |
| 14 | How the default works | Saved default wins over presets |
| 15 | Where to set it | TUI layout screen only |
| 16 | Size of the work | Large, but continue in this session |
| 17 | Spec folder | `ebook-formats` |
| 18 | Tech stack | Approved: stdlib plus existing modules, in-house EPUB reader |
| 19 | Presets and the default | Size presets (Phone, Paperback, A4) keep their size |
| 20 | Design | Approved |
| 21 | Build phases | Posted as an editable list; session stopped with a handoff before an answer |
| 22 | Build phases (resumed from the handoff) | Combine phases where possible while keeping separation of concerns |
| 23 | Risks and success criteria | Approved as drafted |
| 24 | Regrouped build order | Four phases: rename and default page size; reader seam and renderer hooks; EPUB reflowable and fixed-layout; comics, FB2, text, HTML and docs. No changes |
| 25 | Write the final plan | Yes, at 91% confidence |

## Notes

- System context finding: every reader can produce the existing `ebook.Book`; the markup parser already treats each `<html>` as a page break and finds images by `recindex`, so EPUB spine documents can be concatenated with rewritten image references.
- `config.yaml` already accepted `layout.page_size` (applied on top of presets in `cli/flags.go`), but nothing set it and E-reader defaulted to A5.
- Resumed 2026-10-25 from handoff `2026-10-25-0857-handoff` on the web console, at Phase 6.
- Verification: one gap found and fixed (Out of Scope pointed at the old Phase 7 for the owner's manual steps; now Task 4.6).
