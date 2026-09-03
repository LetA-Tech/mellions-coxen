# GitHub issue taxonomy

**Status:** v4.0

The required structure for every issue Mellions creates — bug, remediation, feature, audit, coding,
design or incident follow-up.

Repo-agnostic by construction: no repo name, branch name or person's name in this file. The target
repository and the base branch come from the repository being worked in; every label value is
verified against that repository's own registry before it is used.

**This asset is a specification, not a log.** No changelog, no version history, no dated entries, no
session narration. Changes replace the text they supersede.

## 1. Core rule

An issue is the **work contract**, not proof of implementation. It states what to solve, where, why,
which branch is truth, which call paths are involved, which tests and evidence are required, and
which constraints cannot be violated.

Proof comes later: branch diff, commit SHA, raw command output, test results, CI, evidence, PR URL.

## 2. Title format

```text
[Mellions] - <short action-oriented title>
```

Exactly that: the word, one space, one hyphen, one space. The prefix marks an issue Mellions filed
or that represents Mellions engineering work — it is how the owner reads the tracker. Type and
repository are carried by the `type:` and `repo:` labels and are never restated in the title.

## 3. Body

Eleven sections in fixed order, plus a `Base: <branch> @ <sha>` header. `github-issue-template.md`
defines them and is the authority on their order and wording.

Hard requirements:

- **Evidence is tiered** — confirmed-from-source (`file:line` plus quoted code, verified at the
  stated commit) / evidence-supported / requires-live-evidence. Never present an inference as an
  observation.
- **Evidence from a retention-windowed source states its expiry** — source, window, and expiry
  computed from the **event date the evidence attests to**, never the filing date. Already expired at
  filing time: say so in §1 and treat any dependent criterion as **presumptively unsatisfiable**.
  Non-exhaustive seeds: Plaid Logs 14d, VictoriaLogs 30d, MongoDB audit log ~7d.
- **The call graph is plain ASCII**, never Mermaid. It crosses boundaries — into vendored SDKs and
  dependencies when the decisive behaviour is deeper than the adapter — and shows the failing branch
  and what a crash between two writes leaves.
- **Code locations are an inventory**, not a hint list — files, functions, types, config keys, each
  with path and line range.
- **Runtime conditions** name the flags, env values and binaries that make the path live, and whether
  the defect is live under the shipped configuration.
- **Acceptance criteria are checkable** by reading code or running a command. No "reviewed",
  "documented", "considered".
- **One root cause per issue.** Consolidate symptoms of one defect; never merge independently
  remediable problems.

**N/A profiles.** Sections 4–7 presuppose an implementation. Two shapes have none; both are decided
by the work item's actual nature, and an implementation issue may not invoke either to skip the call
graph or test plan.

**Site content profile** — `type:design` and content/copy items in public web/marketing repos mark
sections 4–7 `N/A (site content profile)`. Sections 1–3 and 8–11 stay mandatory.

**Discovery/decision profile** — `type:design` issues in any repo whose deliverable is a decision,
not an implementation. Sections 4–7 mark `N/A (discovery profile)`, each with a named replacement:

- §4 → **Decision question** — what must be decided, and what depends on the answer.
- §5 → **Options and evaluation criteria** — the candidates and what distinguishes them.
- §6 → **Decision owner** — who the judgment belongs to.
- §7 → **Completion gate** — a written decision record posted to the issue. No decision closes
  silently.

Sections 1–3 and 8–11 are reframed, not waived: §1 the decision needed and the cost of not making it;
§2 why none exists yet; §8 the decision or the invariant it must establish; §10 verifies it was
recorded; §11 restates the gate as checkable items. It closes on `promotion` when it produces a
durable artifact (ADR, design or contract doc), never on `merge`. `deployment`, `migration`,
`recompute` and `runtime-proof` do not fit; a decision has no runtime.

## 4. Label taxonomy

The canonical vocabulary. **Verify each value against the target repo's registry
(`gh label list --repo <repo>`) before use** — registries diverge per repo, and §5 says what to do
where one does not carry a family.

Two families carry no list here, because enumerations go stale and silently block conformant issues.
`repo:<repo>` is the target repository's own short name. `skill:<name>` must resolve to a `SKILL.md` at the target repo's HEAD before it is used — under
`.claude/skills/<name>/` where the repository installs skills itself, or `skills/<name>/` where they
ship in a plugin. An issue requiring a skill nobody can load is a work contract with an
unsatisfiable clause.

### Type

```text
type:bug  type:feature  type:design  type:coding  type:audit  type:remediation  type:incident-followup
```

`type:design` is a decision or discovery work item — architecture, schema, protocol or process,
reached before an implementation exists (§3). **Not** confined to visual or UI work.

### Source

Where the work item came from.

```text
source:operator  source:mellions  source:claude-mcp  source:findeck
source:victoria-logs  source:victoralert  source:ci  source:production
```

### Scope

```text
scope:single-repo  scope:cross-repo  scope:service-local  scope:platform
scope:infra  scope:ci-cd  scope:observability  scope:content
```

### Probability

How likely the failure is to occur, judged independently of how bad it would be. Exactly one,
mandatory. This is an INPUT to `risk:`, never a restatement of it.

```text
probability:high         near-certain, or already occurring in production
probability:medium-high  likely — the conditions that trigger it are commonly met
probability:medium-low   possible — needs an uncommon but reachable combination
probability:low          unlikely — needs a rare or adversarial combination
```

Something that has ALREADY happened is `probability:high`. A latent defect nothing has yet
exercised is not, however severe it would be — that severity is `impact:`, and keeping the two
apart is the whole reason both are recorded.

### Impact

How bad it is if it occurs, judged independently of how likely that is. Exactly one, mandatory.
The second INPUT to `risk:`.

```text
impact:high         severe — member money, data loss, outage, or a compliance breach
impact:medium-high  significant — degraded correctness or availability, member-visible
impact:medium-low   moderate — contained, recoverable, or internal-only
impact:low          minor — cosmetic, hygiene, or cleanup
```

`impact-area:` is a separate, optional, zero-or-more family answering "bad at WHAT": `cost`,
`milestones`, `team`, `quality`, `solution-fit`, `member-experience`, `protection`, `legal`,
`reputation`, `future-growth`. It is never counted toward the mandatory families.

### Risk

`risk:` is the RESULT, not a free judgement: it is the cell this matrix gives for the
`probability:` / `impact:` pair chosen above. Exactly one, mandatory. A rung that does not match
the matrix is non-conformant — recording both inputs is what makes the rung recomputable by a
reader, which a single asserted rung never was.

| probability ↓ / impact → | `low` | `medium-low` | `medium-high` | `high` |
|---|---|---|---|---|
| **`high`** | `medium-low` | `medium-high` | `high` | `high` |
| **`medium-high`** | `medium-low` | `medium-low` | `medium-high` | `high` |
| **`medium-low`** | `low` | `medium-low` | `medium-low` | `medium-high` |
| **`low`** | `low` | `low` | `medium-low` | `medium-low` |

The table is authoritative. It bands `probability × impact` scored `low`=1 … `high`=4 (1–2 low,
3–6 medium-low, 8–9 medium-high, 12–16 high) — but read the cell. Do not redo the arithmetic, and
do not adjust an input to reach a preferred rung: the inputs are the judgement, the rung is not.

`risk:medium` is NOT a rung.

```text
risk:high  risk:medium-high  risk:medium-low  risk:low
```

`risk-area:` is the failure-mode category — zero or more, optional, never counted toward the
mandatory families.

```text
risk-area:financial-correctness  risk-area:schema  risk-area:migration  risk-area:security
risk-area:provider  risk-area:provider-compliance  risk-area:idempotency  risk-area:concurrency
risk-area:cross-repo-contract  risk-area:test-tampering  risk-area:observability
risk-area:deployment  risk-area:telemetry
```

A repo's registry may add its own `risk-area:*` values. `risk-area:financial-correctness` is
reserved for ledger/money math. A repo with no `risk-area:privacy-*` uses `risk-area:security`.

### Priority

```text
priority:p0  production/system-critical
priority:p1  blocks remediation, redeploy, or correctness-critical progress
priority:p2  important but not blocking today
priority:p3  cleanup, future hardening, improvement
```

`severity:*` is deprecated — one priority signal, not two.

### Model routing

Which model the work is suited to. A hint for whoever picks the work up, never a skill name and
never an instruction to a specific session.

```text
executor:opus     complex work needing strong reasoning and long-context understanding
executor:sonnet   medium-complexity and minor work, where the logic, architecture and mechanism
                  are already established
executor:fable    exceptionally complex reasoning, research or technical analysis
executor:codex    exceptionally complex reasoning, research or technical analysis
executor:kimi     as registered in the target repo's registry
executor:manual   must not run autonomously
```

**Haiku is never used** — not at any complexity. There is no `executor:haiku`.

### Site governance

The site governance layer (`meta-flow:*`, `adversarial-pass:*`, `sensitive-action`,
`counsel-ratification-deferred`, `owner:*`, `wcag:*`) is orthogonal and unaffected by this taxonomy.

## 5. Minimum label set

Ten families on every issue:

```text
type:<type>  source:<source>  repo:<repo>  scope:<scope>
probability:<rung>  impact:<rung>  risk:<rung>  priority:<priority>
executor:<model>  skill:<skill-if-known>
```

`probability:` and `impact:` are the two INPUT axes; `risk:` is the matrix result of that pair
(§4). All three are exactly one, and an issue whose `risk:` contradicts its own inputs is
non-conformant even though all ten families are present. `risk-area:` and `impact-area:` are
separate, optional and additional — "bad at what", not "how bad".

### Where the registry does not carry a family

Estate registries diverge, and several repositories carry no taxonomy labels at all. Neither
inventing a value nor abandoning the label set is acceptable: an invented label is a claim about a
vocabulary that does not exist, and no labels at all is how an issue becomes unroutable.

`gh label list --repo <repo>` is read **before** the labels are chosen, not after a failed
`gh issue create`. Then, per family:

- **The family exists and one of its values fits** — use that value.
- **The family exists but this taxonomy's value is not in it** — use the registry's own nearest
  value. Registries carry local synonyms (`source:operator` for `source:operator` is one), and the
  registry wins over this file on values, never on structure.
- **The family is absent from the registry** — do not create it as a side effect of filing, and do
  not silently drop it. Apply every family that does exist, and record the rest in one line at the
  foot of the body:

  ```text
  Labels unavailable in this repository's registry: probability:high, impact:medium-low
  ```

  That line is the exception to "labels are never restated in the body" — it is not routing
  metadata, it is a stated gap, and it is what lets a later reader see the judgement that had
  nowhere to go. Seeding a repository's registry is a separate, deliberate piece of work.

Two Skill-family carve-outs, not to be conflated — the first concerns the *kind of work*, the second
the *repo*. Both require attempting resolution and finding none, never skipping the check.

- **Discovery/decision exemption** — the issue uses the discovery/decision profile (§3) and no
  `skill:*` value governs decision work. State it in the body: `Skill: exempted — no skill governs
  discovery work`. Nine families plus a stated exemption is conformant; nine silently is not.
- **`skill:placeholder`** — **no** `skill:*` value in the target repo resolves at HEAD. It is not a
  skill and never resolves; it marks a stated absence, and the labeled-skill rule carves it out by
  name. If any value resolves, that value is used instead. It needs a tracking issue in the owning
  repository naming the gap, without which it is a stated absence nobody is closing.

## 6. Acceptance review

Accept when the code inventory was verified against the current base branch, all call sites were
identified, the call graph was confirmed or updated, the required commands were run, raw output was
captured, evidence is bound to a commit SHA, and the stated constraints were respected.

Reject when call sites are incomplete, the call graph is missing for a code-path bug, the inventory
is stale, required tests or raw evidence are missing, claims stand in for proof, or the PR head does
not match the verified commit.
