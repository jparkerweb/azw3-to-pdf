# Testing
> Part of [AGENTS.md](../AGENTS.md): project guidance for AI coding agents.

## Running

```sh
go test -race -coverprofile=coverage.out ./...   # the whole suite, as CI runs it
go test ./internal/engine/                       # one package
go test ./internal/engine -run '^TestConvertCBZ$' -v
go test ./internal/engine -run 'TestConvert(CBZ|FB2|PlainText|HTMLFile)$' -v
go test ./internal/tui/...                        # the terminal interface
```

`-race` needs cgo and a C compiler, which a Windows machine may not have; plain
`go test ./...` (`make test`) does not. `TestHTMLFileSymlinkedFolderStaysInFolder`
skips itself where symlinks cannot be created (Windows without Developer Mode).

## How the tests are built

- Tests sit next to the code and are same-package tests. There are no
  `testdata/` folders, golden files or embedded fixtures: every book is built in
  memory or written under `t.TempDir()`.
- **Never use `t.Parallel()`.** Tests swap package globals and restore them with
  `t.Cleanup`: `engine.convertFn` (`fakeConvert` in `batch_test.go`),
  `screens.runBatch`, the zip size caps (`lowerZipLimits` in `zipsafe_test.go`)
  and the CLI flag variables (`resetRunFlags`, `withInputFlags`).
- Only `config_test.go` and `tui/app_test.go` touch the config directory, and
  both point `EBOOK_TO_PDF_CONFIG` at a temp dir with `t.Setenv`. Do the same in
  any test that saves settings.

## Fixture builders

| Builder | Where | Makes |
| --- | --- | --- |
| `buildZip(t, zipEntry{Name, Data, Store})` | `ebook/zipfixture_test.go` | Any archive; `Store` for the EPUB `mimetype` entry |
| `buildMinimalBook` | `ebook/ebook_test.go` | An uncompressed MOBI 6 in memory |
| `writeTestBook(t, dir, name, title, markup)` | `engine/convert_test.go` | The same MOBI, on disk |
| `pngOf`, `epubZip`, `xhtml`, `reflowableEPUB3`, `lockedEPUB`, `opfWith` | `ebook/epub_test.go` | EPUB pieces and whole books |
| `testPNG`, `testXHTML`, `writeTestEPUB` | `engine/epub_convert_test.go` | EPUBs on disk for end-to-end runs |
| `fb2Fixture` | `ebook/fb2_test.go` | A FictionBook document |
| `mustOptions(t)` | `engine/convert_test.go` | The default preset's options with the cover off |

Both `ebook` and `engine` define a `writeTestFile` helper with different
signatures; check which package you are in.

PDF output is checked structurally with `parsePages`
(`pdfout/pdfout_test.go`), never byte for byte.

## End-to-end and TUI tests

End-to-end conversions go through `engine.Convert`: `TestConvertEndToEnd`
(MOBI), `TestConvertReflowableEPUB`, `TestConvertFixedLayoutEPUB`, and in
`formats_convert_test.go` the CBZ, FB2, plain text and HTML cases (through
`convertTestBook`). Add one when adding a format.

`internal/tui/app_test.go` drives the app without a `tea.Program`: `newTestApp`
builds `NewApp` around `sampleBook()` at 100x30, `drive` and `send` call
`Update` directly, and `press` sends a key and runs the single command it
returns, as the event loop would. `TestEveryScreenIsRegistered` and
`TestRegisteredScreens` enforce registration through the `screenCount`
sentinel.
