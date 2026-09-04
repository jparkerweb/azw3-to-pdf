---
name: azw3-to-pdf-release
description: Cut a new semver release. Updates CHANGELOG.md, commits, tags, and pushes to trigger the GoReleaser CI pipeline. Use this skill whenever the user says "release", "cut a release", "bump version", "tag a release", "ship it", or wants to publish a new version. Also use when the user types "/azw3-to-pdf-release".
user-invocable: true
---

# Release Workflow

This skill walks through cutting a new semver release for azw3-to-pdf. Everything after the tag is automated: GitHub Actions runs the tests and then GoReleaser builds binaries for Linux, macOS and Windows on amd64 and arm64.

## Prerequisites

Before starting, verify:

- You are on the `main` branch
- The working tree is clean. If it is not, ask the user whether to commit or stash first
- `make ci` passes (golangci-lint, tests, build). Never tag a red tree
- `CHANGELOG.md` has something to release: either an `[Unreleased]` section with content, or a top version entry that has no matching git tag yet

If a prerequisite fails, stop and help the user resolve it before continuing.

## Step 1: Determine the Next Version

Run `git tag --sort=-v:refname | head -1` to find the latest tag.

**First release.** If there are no tags at all, the version is whatever the top entry of `CHANGELOG.md` already names (`0.1.0` unless it has moved on). There is no `[Unreleased]` section to rename, so skip Step 2 apart from checking the date, and go straight to Step 3.

**Later releases.** Read the `[Unreleased]` section and classify the changes:

| Change type | Version bump |
|-------------|--------------|
| `### Removed`, or `### Changed` with a breaking flag, config or output change | **Major** (X+1.0.0) |
| `### Added` (new features or capabilities) | **Minor** (X.Y+1.0) |
| `### Fixed`, `### Security`, or small `### Changed` entries only | **Patch** (X.Y.Z+1) |

Treat these as breaking for this project: removing or renaming a CLI flag, renaming a preset key or page size, changing a config YAML key, or changing default page geometry enough that existing output changes shape.

Present the proposed version with your reasoning, for example:

> The `[Unreleased]` section has 3 additions and 2 fixes. I would suggest **v0.2.0** (minor bump for the new features). Want to go with that, or pick a different version?

Wait for confirmation. The user may override.

## Step 2: Update CHANGELOG.md

Once the version is confirmed (say `0.2.0`):

1. **Rename `[Unreleased]`** to `[0.2.0] - YYYY-MM-DD` using today's date
2. **Add a fresh `[Unreleased]` header** above it, with no placeholder subsections
3. **Update the comparison links** at the foot of the file:
   - `[Unreleased]: https://github.com/jparkerweb/azw3-to-pdf/compare/v0.2.0...HEAD`
   - `[0.2.0]: https://github.com/jparkerweb/azw3-to-pdf/compare/v0.1.0...v0.2.0`

Entries describe what a user would notice, not how the code changed. "Hanging indents now survive conversion" belongs in the changelog; "extracted a helper in render.go" does not.

Show the user a summary of the changelog diff before proceeding.

## Step 3: Commit and Tag

```bash
git add CHANGELOG.md
git commit -m "Release v0.2.0"
git tag -a v0.2.0 -m "v0.2.0"
```

Tell the user the commit and tag exist locally and nothing has been published yet.

## Step 4: Push (with confirmation)

**Always ask before pushing.** A tag push publishes a public release, so it is the user's call:

> Ready to push the release commit and tag to origin. That triggers the release workflow, which runs the tests and then publishes binaries to the Releases page. Push now?

Only after the user confirms:

```bash
git push origin main --tags
```

## Step 5: Post-Push Summary

Watch the run rather than guessing at it:

```bash
gh run watch --exit-status
```

Then give the user:

- **Actions**: `https://github.com/jparkerweb/azw3-to-pdf/actions`
- **Release**: `https://github.com/jparkerweb/azw3-to-pdf/releases/tag/v0.2.0`

If the release job fails, read the log with `gh run view --log-failed`, fix the cause, delete the tag locally and remotely (`git tag -d v0.2.0 && git push origin :refs/tags/v0.2.0`), and start again from Step 3. Never leave a tag pointing at a commit whose release failed.

## Verifying Before You Tag

The whole pipeline can be rehearsed locally without publishing anything:

```bash
goreleaser release --snapshot --clean --skip=publish
```

That builds all six targets, makes the archives and checksums, and proves the version ldflags land. Check with `./dist/azw3-to-pdf_linux_amd64_v1/azw3-to-pdf version`.

## Reference

- GoReleaser config: `.goreleaser.yaml`
- Release workflow: `.github/workflows/release.yml`, triggered by `v*` tags
- CI workflow: `.github/workflows/ci.yml`, runs on pushes to main and pull requests
- Changelog format: [Keep a Changelog](https://keepachangelog.com/en/1.1.0/)
- The version in the binary comes from git through ldflags, so no constant in the source needs editing
