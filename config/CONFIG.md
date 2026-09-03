<!-- Mellions Engineer | LetA Tech Ltd. | leta@letatech.ca -->

# Configuring Mellions

`mellions config init` writes the file; `mellions config show` prints it back.
It is JSON at `~/.mellions/config.json` (or `-config`, `$MELLIONS_CONFIG`,
`./mellions.json`).

It holds what the runtime cannot know about the estate. Permissions, tools,
hooks, MCP, sandboxing, credentials and model settings belong to Claude Code or
Codex and are inherited unchanged; keys that would shadow them are refused.
What the engineer may decide on its own is not configuration either — it is
the partnership, in the owner's words.

```json
{
  "owner": "your-github-org",
  "repos": ["service-a", "service-b"],
  "work_root": "/home/you/workspace",
  "work_roots": ["/home/you/workspace", "/home/you/projects"],
  "checkouts": {"service-b": "/home/you/elsewhere/service-b"},
  "work_registers": {"service-b": "docs/work-register.md"},
  "sources": ["programs", "assignments", "github", "git", "stale"],
  "owner_labels": ["needs-owner", "pending-owner-decision"],
  "stale_min_age_hours": 168,
  "per_repo_limit": 50,
  "git_since_hours": 168,
  "assignments_root": "/home/you/mellions/assignments",
  "report_root": "/home/you/mellions",
  "programs_dir": "/home/you/mellions/programs",
  "partners_dir": "/home/you/mellions/partners"
}
```

| key | meaning |
|---|---|
| `owner` | the GitHub organisation or user the repositories live under |
| `repos` | the repositories the engineer is responsible for |
| `work_root`, `work_roots`, `checkouts` | where their checkouts are |
| `work_registers` | repository-relative work register for a repository that does not use GitHub issues as its work-item authority |
| `sources` | which survey sources run: `programs`, `assignments`, `github`, `git`, `stale` |
| `owner_labels` | issue labels meaning "waiting on the owner" |
| `stale_min_age_hours` | issues younger than this are not scanned for stale premises |
| `per_repo_limit` | cap on issues and pull requests collected per repository |
| `git_since_hours` | how far back recent-change reporting looks |
| `assignments_root`, `report_root`, `programs_dir`, `partners_dir` | where the engineer's own records live; defaults under `~/mellions` |

Everything else — what is delegated, how the owner wants to be worked with —
is `mellions partner establish`, and the unattended deny list is
`deploy/unattended-settings.json`.

This file contains repository names and local paths. Keep it outside project
repositories unless you intentionally want to share those details. Mellions
stores no credentials here and refuses credential-shaped configuration keys.
