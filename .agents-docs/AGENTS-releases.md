# Releases
> Part of [AGENTS.md](../AGENTS.md): project guidance for AI coding agents.

`CHANGELOG.md` follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/)
and semantic versioning, with the newest version at the top. Work in progress
collects under `## [Unreleased]` with the usual `Added`, `Changed`, `Fixed` or
`Removed` headings. Anything a user would notice belongs in it before the pull
request merges; refactors and test-only changes do not.

Cut a release with the `ebook-to-pdf-release` skill rather than by hand. It
renames `[Unreleased]` to the new version with its date, adds the compare link
at the foot of the changelog, commits `Release vX.Y.Z`, makes an annotated tag
(`git tag -a vX.Y.Z`), and only after asking pushes with
`git push origin main --tags`. The tag drives everything else (see
[Commands](./AGENTS-commands.md) for the release workflow), so the version in
the binary comes from git rather than from a constant in the source.

The compare links in `CHANGELOG.md` still point at `jparkerweb/azw3-to-pdf`,
and so does the git remote, while `go.mod` and `.goreleaser.yaml` say
`ebook-to-pdf`. Check which name the GitHub repository has before adding a
link or publishing.
