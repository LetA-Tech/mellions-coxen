# GitHub issue taxonomy

**Status:** v4.1

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

The repository's mandated schema governs; otherwise use the concise contract in
`github-issue-template.md`: outcome/acceptance, cause/evidence, scope/constraints,
verification, and delivery/decisions, with `Base: <branch> @ <sha>`. Add detail only
where material to implementation or acceptance; omit empty ceremony.

Hard requirements remain:

- **Evidence is tiered** — confirmed-from-source (`file:line` plus quoted source,
  verified at the pin) / evidence-supported / requires-live-evidence. A link to
  established analysis may carry the detail; an inference is never an observation.
- **Retention-windowed evidence states its expiry**, computed from the event date,
  not filing. Already expired: state that in the outcome and mark the dependent
  acceptance criterion **presumptively unsatisfiable** until unexpired evidence exists.
- **Cover the root-cause class.** The inventory includes affected files, functions,
  types and configuration. For code-path defects, establish the decisive call/data
  flow across boundaries, including dependency internals, failing branches, store
  write order and crash consequences. A graph, when useful, is plain ASCII.
- **Runtime conditions and material failure behavior** establish whether the shipped
  path is live and constrain the remedy. Never remove architectural, persistence,
  recovery or concurrency detail needed to choose or validate the fix for brevity.
- **Acceptance is checkable**, with its actual closure event. Required verification,
  independent review and repository approval conditions remain in force.
- **One root cause per issue.** Consolidate symptoms, never independently remediable
  problems. Reuse accepted evidence only while its relevant inputs remain unchanged;
  missing or stale evidence cannot satisfy the contract.

**Work without an implementation.** Site content omits inapplicable call/data-flow
and runtime detail. A discovery/decision issue instead states the question, options,
evaluation criteria, decision owner and completion gate. Record the decision on the
issue; a durable ADR, design or contract artifact closes on `promotion`, not `merge`.
No runtime acceptance event fits a decision without a runtime. Neither kind of work
licenses omitting evidence or tests material to its own outcome.

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
executor:codex    exceptionally complex reasoning, research or technical analysis
executor:kimi     as registered in the target repo's registry
executor:manual   must not run autonomously
```

**Haiku and Fable are never used** — not at any complexity. There is no `executor:haiku` and no
`executor:fable`.

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

- **Discovery/decision exemption** — the issue is discovery/decision work (§3) and no
  `skill:*` value governs decision work. State it in the body: `Skill: exempted — no skill governs
  discovery work`. Nine families plus a stated exemption is conformant; nine silently is not.
- **`skill:placeholder`** — **no** `skill:*` value in the target repo resolves at HEAD. It is not a
  skill and never resolves; it marks a stated absence, and the labeled-skill rule carves it out by
  name. If any value resolves, that value is used instead. It needs a tracking issue in the owning
  repository naming the gap, without which it is a stated absence nobody is closing.

## 6. Acceptance review

Accept when the affected inventory and decisive call/data flow are established at the current base,
required commands were run, raw output was captured, evidence is bound to its commit and relevant
inputs, and stated constraints were respected. Valid linked evidence need not be copied or rerun.

Reject when the class or decisive flow is incomplete, the inventory is stale, required tests or raw
evidence are missing, claims stand in for proof, or the PR head does not match the verified commit.
