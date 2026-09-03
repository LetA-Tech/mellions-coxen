# GitHub issue template

Repo-agnostic. Every `<...>` is a parameter. Section order is fixed; do not add sections.

## Title

```
[Mellions] - <short action-oriented title>
```

The word, one space, one hyphen, one space. Type and repository live in the `type:` and `repo:`
labels, not the title.

## Labels (all ten families required)

```
type:<type>  source:<source>  repo:<repo>  scope:<scope>
probability:<rung>  impact:<rung>  risk:<rung>  priority:<priority>
executor:<model>  skill:<skill-if-known>
```

`probability:<rung>` and `impact:<rung>` are the two INPUT axes — each exactly one, each
`high` | `medium-high` | `medium-low` | `low`. `risk:<rung>` is NOT a third judgement: it is the
cell the taxonomy §4 matrix gives for that pair, and a rung contradicting its own inputs is
non-conformant. Look the cell up; do not estimate it. `risk-area:<category>` (e.g.
`risk-area:financial-correctness`, `risk-area:security`) and `impact-area:<category>` (e.g.
`impact-area:member-experience`, `impact-area:legal`) are separate, optional, additional families:
zero or more, as many as the work item actually spans, never counted toward the ten.
`priority:<rung>` is `priority:p0` … `priority:p3`. `executor:*` is a model
(`opus` | `sonnet` | `fable` | `codex` | `kimi` | `manual`), never a skill name; there is no
`executor:haiku`. Labels carry the routing metadata — never restate them in the body.

Read `gh label list --repo <repo>` before choosing. Taxonomy §5 governs a family the registry does
not carry: apply what exists, record what does not in the one permitted `Labels unavailable`
line, invent nothing.

## Body

Verify every `file:line` against the base branch before emitting it. A wrong line number costs the
implementer an hour.

```markdown
Base: <base-branch> @ <commit-sha>

## 1. Defect

What the code does, and the member-facing / operational consequence. Concrete: inputs and state ->
wrong outcome. Do not restate the title.

## 2. Root cause

The mechanism, not the symptom. If two individually-reasonable decisions are jointly wrong, name both.

## 3. Evidence

**Confirmed from source** — reconstructed from code at `<commit-sha>`, with `file:line` and quoted code.
**Evidence-supported** — follows from the code but was not executed.
**Requires live evidence** — cannot be settled from the repo; name the query, metric series or captured
response that would settle it. Omit this tier only if genuinely empty.

Include any test, probe, log or runtime observation that exists. Say so when a cited comment is wrong.

State what is **not** established, and what would settle it. Where that is the reason this is an
issue rather than a merged change, say so — the limit of the evidence is what decides the
disposition, and burying it costs the reader the judgement.

**Evidence from a retention-windowed source** (e.g. Plaid Logs — 14 days, VictoriaLogs — 30 days,
MongoDB audit log — ~7 days; non-exhaustive) states the source, the window, and the expiry date
computed from the **event date**, not the filing date. If the expiry has already passed as of this
issue's filing date, say so in §1 (not buried here) and mark the criterion it would settle as
presumptively unsatisfiable in §11 — do not present it as pending or checkable.

## 4. Call and data flow

Plain ASCII in a fenced `text` block — never Mermaid (it fails to render in the issue view). Entry
point -> ... -> store or provider write, `file:line` on each hop. Do not stop at an interface,
adapter, repository, client, worker or service boundary when the decisive behavior is deeper —
follow into vendored SDKs and dependencies and cite those paths. Show the failing branch, not only
the happy path. Follow the graph with: which store is written in which order, and what a crash
between two writes leaves behind.

## 5. Code locations

Files, functions/methods, types/interfaces, config keys — each with path and line range. Group by
repo/module when the path crosses into a dependency. This is the implementer's inventory; make it
complete.

## 6. Runtime conditions

Feature flags, env values, deploy settings and binaries that make this path live. Quote the config
lines. State plainly whether the defect is live under the shipped configuration.

## 7. Failure, recovery, concurrency and latency behavior

Only what applies. The interleaving that breaks it; what a crash or restart leaves; what is
restart-dependent; retry/backoff/breaker behavior; round-trip counts and deadlines.

Most defects have no concurrency dimension, and this heading has historically been answered with a
sentence saying so. One line, or absent — a section that exists to deny that it applies is the
padding the banned list forbids. Where it does apply, the race the *fix* would introduce belongs
here as much as the one the defect has.

## 8. Resolution

Outcome-level and production-grade: what must become true. Name the invariant, not the diff. Where a
cheaper partial fix exists, say what it does and does not close.

## 9. Affected components, risks and dependencies

Components touched. What a careless fix breaks. Ordering against other issues, by number.

## 10. Test and verification plan

Exact tests to add or change, with package paths. The command sequence that proves the fix. What raw
output must be captured. If an integration test needs a service, name it.

## 11. Acceptance criteria

- [ ] Objectively verifiable by reading code or running a command. No "reviewed", "documented",
      "considered".
- [ ] A criterion that depends on evidence from a retention-windowed source whose expiry had
      already passed at filing time (§3, §1) is marked **presumptively unsatisfiable**, not phrased
      as a normal checkable item, until proven otherwise by a source that has not expired.
- [ ] States the event the issue closes on — merge, promotion, deployment, migration, recompute or
      runtime proof. An issue that names none closes on a posted proof comment plus the owner's word.
```

## Banned in the body

Work-item-type and source checkbox blocks. Target-repo and base-branch sections (the `Base:` header
covers both). Intent. Required-skills boilerplate. Non-goals. Rollout/alerting boilerplate. An
executor section. External-system notes. Disclaimers. Any sentence instructing the reader to "verify
against dev" or "the implementer must inventory" — procedure is not content. Generic summaries,
preambles, closing paragraphs.

Compress ritual only. Never drop evidence, architectural context, failure behavior, or
implementation-critical detail to be shorter.
