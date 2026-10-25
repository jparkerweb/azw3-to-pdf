# Local skills
> Part of [AGENTS.md](../AGENTS.md): project guidance for AI coding agents.

`.claude/skills/` holds the repeatable workflows for this repo. Prefer them
over improvising the same steps:

| Skill | Use it for |
|-------|-----------|
| `ebook-to-pdf-release` | Cutting a version: changelog, commit, tag, push, watch the pipeline |
| `ebook-to-pdf-update-docs` | Auditing README, AGENTS.md, `.agents-docs/`, the changelog and the skills against the code |
| `ebook-to-pdf-verify-output` | Converting a real book and looking at the rendered pages |
| `ebook-to-pdf-add-preset` | Adding or changing a layout preset or page size |

`.claude/settings.local.json` turns off gitignore filtering so that sample
books and generated PDFs can still be found when testing. `.gitignore` ignores
only `*.pdf`, `*.azw3`, `*.mobi` and `*.azw`; sample `.epub`, `.prc`, `.cbz`,
`.fb2`, `.txt` and `.html` files are not ignored, so keep them out of the repo
by hand.

`.gitignore` also matches `skills/` and `settings.local.json`, so the files
already in `.claude/` are tracked only because they were force-added. A new
skill file needs `git add -f`.
