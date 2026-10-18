# GitHub issue template

Repo-agnostic default; a repository-mandated schema governs. Every `<...>` is a parameter.

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
(`opus` | `sonnet` | `codex` | `kimi` | `manual`), never a skill name; there is no
`executor:haiku` and no `executor:fable`. Labels carry the routing metadata — never restate them in the body.

Read `gh label list --repo <repo>` before choosing. Taxonomy §5 governs a family the registry does
not carry: apply what exists, record what does not in the one permitted `Labels unavailable`
line, invent nothing.

## Body

Keep the contract concise; link existing analysis rather than narrating it again.
Include the detail needed to implement and judge the work, not empty headings.
Verify emitted citations at the stated base; retain quoted source where it supports
an established finding. This default does not reduce repository requirements.

```markdown
Base: <base-branch> @ <commit-sha>

## 1. Outcome and acceptance

Concrete inputs and state -> current wrong outcome and its consequence -> required
outcome. Checkable acceptance criteria, including the event that closes the issue:
merge, promotion, deployment, migration, recompute or runtime proof. No “reviewed”,
“documented” or “considered” as a substitute for correct behavior. An issue naming no
closure event needs a posted proof comment and explicit authorization to close.

## 2. Cause and evidence

The mechanism and affected class, not only the reported symptom. Link established
analysis with its pin; quote the decisive source at `file:line`.

Distinguish **Confirmed from source**, **Evidence-supported** (inferred, not executed),
and **Requires live evidence** (name the query, metric or response needed). Include
existing test/runtime observations; say what is unknown and what would settle it.
A filed hypothesis is not an established defect.

For retention-windowed evidence, state source, window and expiry from the event date.
Already expired at filing: say so in §1 and mark any dependent acceptance criterion
**presumptively unsatisfiable** until an unexpired source establishes it.

## 3. Scope and constraints

Affected files, functions, types and config keys, with current path/line references;
cover the root-cause class. Link an existing complete inventory rather than copy it.
Include architectural, runtime and failure detail wherever it changes the remedy:

- Call/data flow through the decisive implementation, including dependencies and
  other repositories. Use plain ASCII if a graph helps; Mermaid does not render
  reliably in issue views. Show the failing branch, stores and write order, and what
  a crash between writes leaves.
- Flags, environment, deploy settings and binaries that make the path live, with
  source evidence; say whether the shipped configuration reaches the defect.
- Material persistence/migration consequences, retry/recovery, concurrency,
  round trips and deadlines, including races a proposed fix could introduce.
- Required architecture/contracts, protected paths, affected components, dependencies
  and what a careless fix breaks. Resolution states the invariant to establish;
  a partial remedy says what it leaves unresolved.

Omit inapplicable detail; do not write sections merely to deny that they apply.

## 4. Verification

Tests to add/change and commands to run, with paths, required services or devices,
meaningful regression/falsification, baseline evidence for failure attribution and
required final gates. State the raw evidence to retain and link accepted proof that
remains valid. Missing proof is work, never an assumed pass.

## 5. Delivery and decisions

Material risks, dependencies/ordering and reserved decisions or approval conditions.
A current contract need not be restated as a separate plan unless the repository
requires one; record material decisions before implementation. Keep implementation
proof in the PR and durable linked artifacts, then add the actual closure evidence.
```

## Banned padding

Routing metadata already carried by labels, required-skills boilerplate, empty N/A
sections, generic preambles, closing narratives and instructions to “verify against
dev” or “inventory later” in place of actual evidence. Compress ritual only; retain
implementation-critical evidence, architecture, failure behavior and constraints.
