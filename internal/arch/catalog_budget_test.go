// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package arch

import (
	"os"
	"path/filepath"
	"testing"
	"unicode/utf8"
)

// The per-file bound `runtime_bounds_test.go` guards decides what a Skill's
// BODY may hold. A second, independent bound decides whether the model is ever
// told the Skill exists, and it is aggregate: every installed skill's catalog
// line is rendered into one budget, shared across every plugin on the host.
//
// codex-rs `ext/skills/src/render.rs` at `rust-v0.149.0`, the tag matching the
// installed codex-cli 0.149.0:
//
//   - `skill_metadata_budget` (:127) picks the budget. A configured
//     `max_context_tokens` gives Tokens(min(configured, 10_000)); otherwise a
//     known context window gives Tokens(window * 2 / 100)
//     (`SKILL_METADATA_CONTEXT_WINDOW_PERCENT` = 2); with neither, it falls back
//     to Characters(`DEFAULT_SKILL_METADATA_CHAR_BUDGET`) = 8,000.
//   - each entry renders as `- {name}: {description} ({kind}: {locator})` plus a
//     newline (:243-262), costed in characters or in ceil(bytes/4) (:170-183).
//   - `allocate_skill_lines` (:325) has three regimes. Everything fits: full
//     descriptions. It does not, but the name-and-locator lines alone do:
//     descriptions are shortened round-robin ACROSS THE WHOLE CORPUS (:408) —
//     every Skill is cut to one near-uniform cap, not the one that grew. Even
//     those do not fit: entries are dropped (:353-366) by a greedy fill, which
//     is not a prefix cut — a cheap line after an expensive one still gets in.
//   - the description is capped at 1,024 characters first (:1170), which is what
//     `runtime_bounds_test.go` already guards; that cap is per entry and does
//     nothing about the aggregate.
//   - none of it is announced. A budget-shortened description gets no ellipsis
//     (:243-250) and reads as one that ends mid-sentence; only the 1,024 cap
//     adds one. A dropped Skill leaves an omission line only where the policy
//     includes one (:69-74), which excludes the host catalog (:762). And the
//     warning that would name the shortening averages over EVERY Skill rather
//     than the shortened ones (:116-124), so cutting a few badly among many
//     stays under the 100-character threshold and says nothing at all (:112).
//
// So a description is not free text. It is the only thing that tells a model
// when to reach for a method, and it is spent out of one budget shared with
// every other skill installed beside it — and, where more than one of the
// executor, orchestrator and host catalogs is populated, across all three in a
// single allocation (:722-727).
//
// This guard is a ratchet, not a fit test, and the distinction is the finding:
// AT THE RECORDED MEASUREMENT THE MELLIONS CORPUS ALONE ALREADY EXCEEDS THE
// 8,000-CHARACTER DEFAULT BUDGET, and that default is what a surface rendering
// with no known context window gets (`extension.rs:232` passes None).
// Bringing it under is a design decision about where method lives, which is
// open on issue #230 and is the owner's; keeping the number from growing while
// that is decided is not, so the ceiling below is the measured value and moving
// it up is a deliberate act with a reason.
const (
	// DEFAULT_SKILL_METADATA_CHAR_BUDGET, render.rs:17 — the whole catalog.
	codexCatalogCharBudget = 8000

	// Measured against the corpus, not chosen. Raising it is allowed and is
	// meant to be argued for in the commit that does it.
	corpusCatalogCharCeiling = 9323
)

// catalogLineChars is what one entry costs in the Characters regime, less the
// locator. The locator is an install path or a package id — it varies by host
// and is not the corpus's to control — so leaving it out makes every number
// here a LOWER BOUND on the real cost. A guard that under-reports fails late;
// one that over-reports fails on somebody else's install layout.
func catalogLineChars(name, description string) int {
	// "- " + name + ": " + description + "\n"; render.rs:252-261.
	return 2 + utf8.RuneCountInString(name) + 2 + utf8.RuneCountInString(description) + 1
}

// TestSkillCatalogMetadataStaysUnderItsRatchet walks the corpus for the same
// reason the per-file guard does: the Skill added tomorrow is the one nobody
// adds to a list.
func TestSkillCatalogMetadataStaysUnderItsRatchet(t *testing.T) {
	root := repoRoot(t)
	entries, err := os.ReadDir(filepath.Join(root, "skills"))
	if err != nil {
		t.Fatal(err)
	}

	total := 0
	held := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		raw, err := os.ReadFile(filepath.Join(root, "skills", name, "SKILL.md"))
		if err != nil {
			t.Errorf("skills/%s: %v", name, err)
			continue
		}
		front, ok := frontmatter(string(raw))
		if !ok {
			continue // runtime_bounds_test.go reports this; do not report it twice.
		}
		m := descriptionLine.FindStringSubmatch(front)
		if m == nil {
			continue // likewise.
		}
		held++
		total += catalogLineChars(name, m[1])
	}

	// A walk that matched nothing reports every corpus clean, and reads exactly
	// like a corpus that fits.
	if held == 0 {
		t.Fatal("no SKILL.md description was measured; this guard would pass against an empty corpus")
	}

	if total > corpusCatalogCharCeiling {
		t.Errorf("the %d Skill descriptions cost at least %d catalog characters, above the recorded %d. "+
			"Codex shortens descriptions across the WHOLE corpus when the shared budget is exceeded and drops "+
			"entries when even the bare lines do not fit, so this is spent against every other skill installed "+
			"beside Mellions — shorten a description rather than raising this, or raise it in a commit that says why",
			held, total, corpusCatalogCharCeiling)
	}

	if total <= codexCatalogCharBudget {
		t.Errorf("the corpus now costs %d catalog characters, at or under the %d-character default budget "+
			"(render.rs:17) that this guard's comment records it as exceeding. That is good news and the comment "+
			"is now wrong: re-read the mechanism, then say so here rather than leaving a stale claim standing",
			total, codexCatalogCharBudget)
	}
}
