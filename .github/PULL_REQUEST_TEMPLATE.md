## Related issue

<!-- Link the issue when one exists, e.g. Closes #123. Small documentation/local fixes do not require ceremonial issue creation. -->

## What engineering problem does this solve?

<!-- Describe the observed problem or capability gap. Establish the problem before describing the implementation. -->

## Root cause / evidence

<!-- What evidence establishes the cause? If this is a runtime integration, name the lifecycle surfaces you actually verified. -->

## What changed?

<!-- Keep the change centered on one engineering responsibility. -->

## Verification

<!-- List commands, tests, fresh-session checks, installed-product checks, or other evidence. -->

```text
make check
```

## Falsification / negative arm

<!-- How did you try to make the claimed fix fail? If a mutation should break the property, did the relevant test actually go red for the intended reason? -->

## Runtime / model impact

- [ ] Claude Code
- [ ] Codex
- [ ] Mellions CLI / shared core only
- [ ] New runtime adapter
- [ ] Documentation only

<!-- Model names are useful context, but runtime support is the integration boundary. -->

## Evidence boundary

- [ ] I have not included credentials, private infrastructure, customer/user data, or private project material.
- [ ] I distinguish measured results from inference or retrospective estimates.
- [ ] I have not claimed support for a runtime that this change does not actually integrate and verify.
- [ ] I have stated meaningful limitations that remain unproved.

## Contribution checks

- [ ] This PR targets `dev` unless a maintainer requested a release-specific PR to `main`.
- [ ] I ran `make check` locally, or I explain below why I could not.
- [ ] The change is focused on one engineering responsibility.
- [ ] I understand that contributions accepted into Mellions Engineer are licensed under Apache-2.0.

## Remaining limitations

<!-- State what this PR does not establish. A good PR is allowed to have a boundary. -->

## Why does this belong in Mellions?

<!-- Explain why this is durable engineering responsibility/reliability behavior rather than functionality better owned by the native coding-agent runtime or an individual repository. -->
