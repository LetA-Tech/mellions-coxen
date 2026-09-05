---
name: mellions-harness-rule
description: Turn a lesson into a check the repository enforces itself. Use when a defect has just been fixed and the same mistake could be made again — especially when it exists or existed in sibling repositories, or when the reason it was wrong is invisible from the call site.
---
<!-- Mellions Engineer | LetA Tech Ltd. | leta@letatech.ca -->

# Turning a lesson into a harness rule

A fix repairs one occurrence. A harness rule stops the next one.

The difference matters most for defects that are invisible locally. `AcquireDuration()`
returns a `time.Duration`; comparing it to a threshold reads correctly at the call
site and is wrong only because the value is cumulative. No amount of care at that
line prevents it. A check that fails the build does.

## When this is worth doing

Ask, after the fix is proven:

- Could a competent engineer make this same mistake again next month?
- Did the same defect exist in a sibling repository? (It usually did.)
- Is the reason it is wrong invisible from the code that does it?

Two yeses is enough. One is usually not.

**When it is not worth doing:** a one-off in code nobody else touches, a defect
whose wrongness is obvious on the line, or anything you would have to over-fit a
matcher to catch. A rule that fires on correct code is worse than no rule — it
gets suppressed, and then it protects nothing.

## Where the rule goes

In the repository that has the defect, not in the engineer. A rule in a skill
binds sessions that load it. A rule in the repository binds every future
engineer, including the humans, including the ones who never heard of this.

Prefer, in order:

1. **A test in the package itself** — no new tooling, runs in the existing suite,
   and the failure lands next to the code.
2. **A `go vet`-style analysis or a linter rule** — when the pattern spans
   packages.
3. **A CI step** — only when it genuinely cannot run locally.

## Writing it

Go makes the first option cheap: parse the package and assert the shape.

```go
// TestAcquireDurationIsNeverComparedDirectly guards a defect that is invisible
// at the call site: pgxpool's AcquireDuration() is the CUMULATIVE total across
// the pool's lifetime, so comparing it to a per-acquire threshold produces a
// warning that latches on and never clears. It shipped in four services.
//
// Divide by AcquireCount() first. See evaluateAcquireLatency.
func TestAcquireDurationIsNeverComparedDirectly(t *testing.T) {
    fset := token.NewFileSet()
    pkgs, err := parser.ParseDir(fset, ".", nil, 0)
    // ... walk for a BinaryExpr whose operand is a call to AcquireDuration
    //     and whose operator is a comparison, ignoring the one in the helper
}
```

Properties the rule must have:

- **It names the defect and why, not just the pattern.** Whoever trips this in
  six months needs to understand it without reading the original issue.
- **It fails on the defect and passes on the fix.** Prove that: reintroduce the
  old shape, watch it fail, restore it. This is the same falsification the fix
  itself needed, applied to the rule.
- **It cannot pass while the property is false.** The matcher is a proxy, and
  reintroducing the defect tests one instance of it. Where the rule claims more
  than it literally matches, write the change that satisfies the matcher and
  leaves the property false — the phrase kept in a sentence that no longer
  instructs, the field checked and then ignored — and watch it pass. If one
  exists, widen the check or narrow what the failure message claims.
- **It says what to do instead.** A failure that only says "do not do this" sends
  the next engineer looking for the answer.

## Then close the class

A rule stops the next crossing; it does not repair the ones already made. Where
it fires on the crossing — a formatter's rewrite, a coercion, a truncation, a
normalisation — everything already across is green to it: past the transition
the artefact is valid, and the tool that would have objected has nothing left to
see. Count that population out of the corpus rather than out of the rule. A rule
that instead tests every artefact on every run has already counted it.

A rule in one repository leaves the siblings carrying the defect. Check them —
`gh search code` across the org finds the shape in minutes — and either fix them
or file, with the rule cited so the fix and its guard travel together.

That is the whole point: the lesson stops depending on anyone remembering it.
