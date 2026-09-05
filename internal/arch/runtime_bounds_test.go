// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package arch

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Both runtimes carry a Skill, and the smaller of the two decides what the
// corpus may hold. The bound bites when a Codex session INVOKES a Skill, not
// at startup: the body is then injected and cut to 8,000 bytes at a UTF-8
// boundary. `MAX_SKILL_PROMPT_BYTES` in codex-rs `ext/skills/src/render.rs`,
// applied by `truncate_main_prompt_contents` — unconditionally per selected
// entry in `ext/skills/src/extension.rs`, and in `ext/skills/src/host_prompt.rs`
// only where `is_agent_plugin_skill` holds, which is how a Mellions Skill
// arrives. Verified against codex-rs `rust-v0.149.0`, the tag matching
// codex-cli 0.149.0; re-derive it there rather than trusting this line.
//
// The startup catalog carries name, description and a locator, never a body,
// so a prompt render taken before any Skill is invoked shows no body — that
// is the catalog behaving, not evidence the bound does not bind. A cut raises
// a session warning naming the Skill; the model is told nothing, so what
// reaches it is method ending mid-sentence.
//
// Both runtimes also cap a catalog description at 1,024 characters. A Skill
// past either reaches one runtime whole and the other cut off mid-method.
// That cap is per entry and is not what decides whether a description survives:
// Codex renders every installed skill's catalog line into one shared budget and
// shortens or drops entries to fit. `catalog_budget_test.go` holds that bound.
//
// The guard measures the whole file, and so does Codex: `read_skill_text` in
// `ext/skills/src/host_outcome.rs` reads `path_to_skills_md` whole with
// `ReadFileOptions::default()`, and no step between there and the cut strips
// YAML frontmatter — the crate contains no frontmatter handling at all. The
// frontmatter is inside the 8,000, so no free bytes hide in it. Traced through
// the local-filesystem provider, which is what a directory marketplace uses
// here; a remote plugin install reads by a different field and was not traced.
//
// A description is charged to both bounds and cannot be trimmed for free.
// `skills` in `cmd/mellions/skills.go` matches a query against the WHOLE
// description (:83), utterance triggers included; `situation` (:204) cuts at
// `Triggers —` for display only. Deleting a trigger list therefore costs the
// method its own defining query: with the lists gone from the three Skills the
// session hooks deliver, `mellions skills "prove a fix holds"` stops returning
// mellions-falsification alone and returns six, and `"establish what the code
// actually does"` puts mellions-deep-research third.
//
// The hooks `session-reasoning.sh`, `session-research.sh` and
// `session-falsification.sh` strip the frontmatter before injecting the body,
// so for those three the description is charged to the per-file cap and to the
// catalog budget while contributing nothing to what a session carries at
// start. That asymmetry is real and the bytes are still not recoverable this
// way; a saving has to come out of the body.
const (
	codexSkillBytes  = 8000
	descriptionChars = 1024
)

var descriptionLine = regexp.MustCompile(`(?m)^description:[ \t]*(.*)$`)

// TestEverySkillFitsBothRuntimes walks the corpus rather than naming it. A
// list holds the Skills somebody remembered to add; the corpus grows, and the
// one added tomorrow is exactly the one nobody adds to a list.
func TestEverySkillFitsBothRuntimes(t *testing.T) {
	root := repoRoot(t)
	entries, err := os.ReadDir(filepath.Join(root, "skills"))
	if err != nil {
		t.Fatal(err)
	}
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
		held++
		if len(raw) > codexSkillBytes {
			t.Errorf("skills/%s/SKILL.md is %d bytes; a Codex session invoking it receives the first %d "+
				"and the method ends mid-sentence — re-cut it rather than raising the bound", name, len(raw), codexSkillBytes)
		}
		front, ok := frontmatter(string(raw))
		if !ok {
			t.Errorf("skills/%s/SKILL.md has no frontmatter to read a description from", name)
			continue
		}
		m := descriptionLine.FindStringSubmatch(front)
		if m == nil {
			t.Errorf("skills/%s/SKILL.md has no description line", name)
			continue
		}
		if n := len([]rune(m[1])); n > descriptionChars {
			t.Errorf("skills/%s description is %d characters; both catalogs cut it at %d", name, n, descriptionChars)
		}
	}
	// A walk that matched nothing reports every corpus clean, and reads in CI
	// exactly like a corpus that fits.
	if held == 0 {
		t.Fatal("no SKILL.md was examined; this guard would pass against an empty corpus")
	}
}

// frontmatter returns the YAML block a SKILL.md opens with. The description is
// read from there rather than from the whole file: a body line beginning
// description: is prose, and matching it would let a Skill whose frontmatter
// carries no description pass.
func frontmatter(text string) (string, bool) {
	if !strings.HasPrefix(text, "---\n") {
		return "", false
	}
	end := strings.Index(text[4:], "\n---")
	if end < 0 {
		return "", false
	}
	return text[4 : 4+end], true
}

// A Skill written up to the hard cap has nowhere to put the next lesson, and
// the session that has one discovers that at the moment it is writing it —
// so the trade gets made against a deadline, by deleting a paragraph, and is
// invisible in the diff. The reserve is the room an ordinary lesson needs.
const skillReserveBytes = 500

// Measured, not chosen: the Skills already inside the reserve, at the size
// they are. A listed Skill may not grow; an unlisted one may not enter the
// band. An entry leaves when its Skill drops below the band, and one is added
// only by a commit that says what was spent and why.
var skillsInReserve = map[string]int{
	"mellions-continuity":        7999,
	"mellions-environment":       7995,
	"mellions-self-learning":     7987,
	"mellions-territory":         7983,
	"mellions-delegation":        7983,
	"mellions-issue-remediation": 7977,
	"mellions-reasoning":         7994,
	"mellions-bug-audit":         7821,
	"mellions-falsification":     7995,
	"mellions-deep-research":     7888,
	"mellions-issue-closure":     7651,
}

// TestNoSkillSpendsItsLastBytesUnnoticed fires with room left to act on,
// which the hard cap by construction cannot: it reports at the boundary, when
// the only move left is to delete something.
func TestNoSkillSpendsItsLastBytesUnnoticed(t *testing.T) {
	root := repoRoot(t)
	entries, err := os.ReadDir(filepath.Join(root, "skills"))
	if err != nil {
		t.Fatal(err)
	}
	band := codexSkillBytes - skillReserveBytes
	held := 0
	seen := make(map[string]bool, len(skillsInReserve))
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		raw, err := os.ReadFile(filepath.Join(root, "skills", name, "SKILL.md"))
		if err != nil {
			continue // TestEverySkillFitsBothRuntimes reports this; do not report it twice.
		}
		held++
		baseline, listed := skillsInReserve[name]
		if listed {
			seen[name] = true
			if len(raw) > baseline {
				t.Errorf("skills/%s/SKILL.md is %d bytes, above its recorded %d and inside the %d-byte "+
					"reserve below the %d cap — a Skill carrying this much debt may not take on more; "+
					"restore headroom in it rather than raising the baseline", name, len(raw), baseline, skillReserveBytes, codexSkillBytes)
			}
			continue
		}
		if len(raw) > band {
			t.Errorf("skills/%s/SKILL.md is %d bytes, inside the %d-byte reserve below the %d cap: a lesson "+
				"of ordinary size can no longer land here without deleting one already there. Restore headroom, "+
				"or record the debt in skillsInReserve in the commit that spends it", name, len(raw), skillReserveBytes, codexSkillBytes)
		}
	}
	for name, baseline := range skillsInReserve {
		if !seen[name] {
			t.Errorf("skillsInReserve records %s at %d bytes and the corpus has no such Skill, so that entry "+
				"exempts nothing and hides the next Skill that takes its name", name, baseline)
		}
	}
	// A walk that matched nothing reports every corpus clean, and reads in CI
	// exactly like a corpus with room to spare.
	if held == 0 {
		t.Fatal("no SKILL.md was examined; this guard would pass against an empty corpus")
	}
}
