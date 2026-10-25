# Adding things
> Part of [AGENTS.md](../AGENTS.md): project guidance for AI coding agents.

- **A format**:
  - a reader file in `internal/ebook`;
  - in `open.go`: a `Format` constant, its `Format.String()` case, its
    `decodeAs` case, and a detection rule in `Detect` (through `classifyZip`
    for a zip, `detectText` for a text or markup format);
  - its extensions in `engine.BookExtensions`, and in `engine.TextExtensions`
    too if folder scans should skip it unless `--include-text` is given;
  - the extension list in the root command's help text (`internal/cli/root.go`)
    and the splash line in `internal/tui/screens/splash.go`;
  - tests in `ebook/open_test.go` and an end-to-end case in
    `engine/formats_convert_test.go`;
  - a row in the README's format table.
- **A page size**: `internal/pdfout/options.go`, the `pageSizes` map.
  Completion, the `page-sizes` command and the layout screen pick it up.
- **A preset**: `internal/presets/presets.go`, the `registry` slice. Set
  `SizePreset` only when the page size is the point of the preset (Paperback,
  A4, Phone); every other preset takes the user's default page size, and its
  `PageSize` field is never read. Adding or removing a size preset also means
  editing the hard-coded "(not used by Phone, Paperback, A4)" status in
  `internal/tui/app.go`.
- **A layout setting**:
  - `pdfout.Options`, `OptionsOverrides`, `ApplyOverrides` and `OverridesFrom`
    in `internal/pdfout/options.go` (`TestOverridesCoverEveryOption` fails if
    one is missed; it is the only part a test enforces);
  - the flag in `registerRunFlags`, any range check in `validateFlags`, and
    `flagOverrides`, all in `internal/cli/flags.go`;
  - `config.LayoutConfig` and `configOverrides` if it should be saved in the
    config file;
  - the `presets.Preset` fields and `OptionsWithPageSize` if presets should set
    it;
  - the `layoutFields` table in `internal/tui/screens/layout.go`.
- **A screen**:
  - a model in `internal/tui/screens` implementing `style.Screen`
    (`FooterHint`, `Bindings`, `PushState`, alongside `style.ScreenModel`'s
    `Init`/`Update`/`View`);
  - a `messages.Screen` constant and its `Screen.String()` case (the header
    shows it, and `TestScreensRender` checks the header);
  - registration in `NewApp`, and an entry in `TestScreensRender`'s
    hand-written list in `internal/tui/app_test.go`.
