<!-- Mellions Coxen | LetA Tech Ltd. | leta@letatech.ca -->

# Claude Code model delegation — operator policy example

This is an **opt-in operator preference**, not a built-in Coxen model router or
an automatic change to a Claude Code installation. Coxen remains model- and
runtime-agnostic. Record the actual standing preference in the operator's
adopted partnership (`DECLARED`); select models and effort through Claude Code.
Do not encode provider-specific model configuration in Mellions' Go core,
`mellions.json`, global engineer identity, or shared runtime hooks.

## Example standing preference: Opus leads, Sonnet implements

| Engineer | Model | Default for substantial delegated work | Responsibility |
|---|---|---|---|
| Lead | Claude Opus 5.5 (`claude-opus-5-5`) | `xhigh`; `max` for exceptional uncertainty or high consequence | Deep audit, root cause, architecture, production solution design, novel complex implementation, high-stakes testing and independent end-to-end acceptance |
| Implementation worker | Claude Sonnet 5.5 (`claude-sonnet-5-5`) | `xhigh`; `max` only when needed | Well-specified production implementation, bounded remediation, regression tests, medium engineering work, routine PR review, sanity checks, refactoring and cleanup |
| Restricted worker | Claude Haiku 5.5 (`claude-haiku-5-5`) | **Disabled pending operator approval** | A possible future documentation-only editorial worker, after evaluation; never an implicit fallback |

A model is selected by **remaining uncertainty, consequences of error, and
required architectural judgment**, not ticket length or lines of code.
`max` is not the default for every task: weigh the additional reasoning
against cost, latency and observable value. The engineer uses judgment within
the operator's standing preference.

### Normal high-stakes handoff: Opus → Sonnet → Opus

1. **Opus investigates.** Re-establish the issue premise at current code,
   reproduce the defect when possible, inspect the full path and adjacent
   contracts, establish root cause, choose a production resolution and define
   evidence-backed acceptance criteria.
2. **Sonnet implements.** Own the isolated worktree; implement the grounded
   resolution with production-quality code and tests. Challenge the plan when
   code evidence contradicts it. Return actual commands, outputs, changed
   contracts and unresolved risks, not a confidence-based completion claim.
3. **Opus verifies.** Independently read the original requirement and the
   affected behavior; inspect the implementation, rerun the relevant checks,
   challenge the regression coverage, and validate across consequential
   boundaries. Delegate a bounded correction back to Sonnet if justified and
   re-verify it before accepting the outcome.

**Exceptions:** Opus writes novel or architecture-intensive code itself when
delegation would fragment critical reasoning. Sonnet may complete straightforward,
well-bounded tasks independently under normal verification gates. Do not force
three handoffs where one competent engineer can complete the obligation.
A new architectural or consequential discovery returns to Opus for judgment,
not to a worker instructed to make a speculative patch.

A Sonnet review can check routine PRs, but it is not a substitute for independent
Opus validation of materially high-risk changes.

### Haiku is not approved

Keep Haiku disabled for implementation, root-cause analysis, architecture,
financial or security logic, production decisions, migrations, and final
acceptance. Before considering it for non-normative editorial work, compare
actual output quality, review effort, latency, and cost against Sonnet on a
representative set of documents, and obtain explicit operator approval.
Technical documentation that specifies API contracts, architecture, operational
procedures, or security boundaries remains engineering work.

## How to apply this policy in Claude Code

1. Place the chosen preference in the partnership's owner-written `DECLARED`
   section and adopt it using the existing `mellions partner` workflow. That
   is the standing source of *who should do what*, not a model-setting file.
2. Start the lead session with the intended native model and effort (for
   example, `claude --model claude-opus-5-5 --effort xhigh`).
3. When delegating bounded implementation, select Sonnet explicitly through
   the native per-invocation model selection, or install a user-owned custom
   subagent in `~/.claude/agents/`. An optional example follows; it is **not**
   installed by Coxen:

```markdown
---
name: mellions-sonnet-implementer
description: Implement grounded production resolutions and bounded remediation after the lead defines the objective and evidence.
model: claude-sonnet-5-5
effort: xhigh
---

Own only your assigned isolated worktree. Recheck the relevant code before
editing. Implement the complete grounded resolution and tests; do not blindly
trust a proposed fix. Escalate newly discovered architecture or changed
contracts with concrete evidence. Return changes, test commands and raw
results, remaining risk, and a precise handoff. Do not claim final acceptance
on behalf of the lead engineer.
```

4. Verify the **effective** model and effort in the running Claude Code task
   (for example, `/tasks`), not just the requested frontmatter. Per-invocation
   selection, agent definitions, environment defaults, and organization policy
   can affect the result. In particular,
   `CLAUDE_CODE_SUBAGENT_MODEL_FORCE=1` can defeat mixed-model delegation;
   `CLAUDE_CODE_EFFORT_LEVEL` overrides agent frontmatter effort.
5. If the selected model/effort is unavailable, restricted, or substituted,
   report what actually ran and choose an authorized alternative with the
   operator rather than silently lowering the standard for consequential work.
   For Sonnet `max` assignments, use a supported per-session or suitable
   subagent configuration; the `xhigh` example above does **not** imply
   `max`.

Coxen's `mellions-delegation` method governs responsibility, dispatch content,
worktree isolation, review and handoff. Claude Code remains the authority for
the actual model, effort, tools and permissions. Updating this document alone
does not activate a model policy on any host.

References: [Claude models](https://platform.claude.com/docs/en/models/overview),
[Claude Code subagents](https://code.claude.com/docs/en/sub-agents),
[Claude Code model/effort configuration](https://code.claude.com/docs/en/model-config).
