// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package arch

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// notSkills share the prefix a Skill reference carries without being Skills:
// the repository and the programs.
var notSkills = map[string]bool{
	"mellions-coxen": true,
}

// notSkillIDs share the `mellions:` prefix a Skill-tool id carries without
// being Skills: the claim label a lane publishes on the tracker, and the slash
// commands, read from commands/ so the list cannot fall behind them.
func notSkillIDs(t *testing.T, root string) map[string]bool {
	t.Helper()
	ids := map[string]bool{"claimed": true}
	cmds, _ := filepath.Glob(filepath.Join(root, "commands", "*.md"))
	for _, c := range cmds {
		ids[strings.TrimSuffix(filepath.Base(c), ".md")] = true
	}
	return ids
}

// retired matches the names of Skills this corpus no longer ships — the old
// estate-process names, and the five unprefixed names the corpus renamed. A
// method that hands work to one of them sends the session looking for a
// method that is not there. The prefixed forms are excluded by the character
// class before the name, so `mellions-bug-audit` does not match `bug-audit`.
var retired = regexp.MustCompile(`(?:^|[^a-z-])(github-issue-creation|github-issue-resolution-proposal|github-issue-resolution-close|bug-audit|issue-remediation|harness-rule|coding-hygiene-comment|leta-sandbox)\b`)

// skillRefs are the forms in which the corpus names a Skill: the name in
// backticks or parentheses, the id in backticks or bold as the skills hook
// prints it, the Skill-tool call in either quoting, and a bare name followed by
// the word Skill. A name in a path or a plain string is not a reference, and
// the forms are what keep those apart.
var skillRefs = []*regexp.Regexp{
	regexp.MustCompile("`(mellions-[a-z][a-z0-9-]*)`"),
	regexp.MustCompile(`\*\*(mellions-[a-z][a-z0-9-]*)\*\*`),
	regexp.MustCompile(`\((mellions-[a-z][a-z0-9-]*)\)`),
	regexp.MustCompile("`mellions:([a-z][a-z0-9-]*)`"),
	regexp.MustCompile(`\*\*mellions:([a-z][a-z0-9-]*)\*\*`),
	regexp.MustCompile(`Skill\(\s*skill:\s*\\?["']mellions:([a-z][a-z0-9-]*)\\?["']\s*\)`),
	regexp.MustCompile(`\b(mellions-[a-z][a-z0-9-]*) Skills?\b`),
}

// TestSkillsNameOnlySkillsThatExist: prose is the one surface no compiler
// reads. A Skill renamed, removed or never ported leaves every pointer to it
// standing, and a session following the pointer finds nothing — which it cannot
// tell from a Skill it was not shown. Every Skill the corpus names resolves to
// a directory under skills/, and no retired name survives anywhere in it.
func TestSkillsNameOnlySkillsThatExist(t *testing.T) {
	root := repoRoot(t)
	have := map[string]bool{}
	entries, err := os.ReadDir(filepath.Join(root, "skills"))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.IsDir() {
			have[e.Name()] = true
		}
	}

	ids := notSkillIDs(t, root)
	var problems []string
	for _, path := range corpusFiles(t, root) {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		rel, _ := filepath.Rel(root, path)
		for i, line := range strings.Split(string(raw), "\n") {
			at := rel + ":" + strconv.Itoa(i+1)
			for _, re := range skillRefs {
				for _, m := range re.FindAllStringSubmatch(line, -1) {
					name := m[1]
					if have[name] || notSkills[name] || ids[name] {
						continue
					}
					problems = append(problems, at+": names "+name+", which is not a Skill under skills/"+
						" (a repository, program or label that is not a Skill goes in notSkills or notSkillIDs)")
				}
			}
			for _, m := range retired.FindAllStringSubmatch(line, -1) {
				problems = append(problems, at+": names "+m[1]+", a Skill this corpus no longer ships under that name")
			}
		}
	}
	sort.Strings(problems)
	for _, p := range problems {
		t.Error(p)
	}
}

// corpusFiles is everything a session is given or that describes what it is
// given: the Skills, the identity, the commands, the hooks, the CLI's own
// prompts, the shift, and the current documents. The archive is history and
// may name anything.
func corpusFiles(t *testing.T, root string) []string {
	t.Helper()
	var out []string
	for _, d := range []string{"skills", "agents", "commands", "hooks", "cmd/mellions", "scripts", "deploy"} {
		filepath.WalkDir(filepath.Join(root, d), func(p string, e os.DirEntry, err error) error {
			if err != nil || e.IsDir() {
				return err
			}
			switch filepath.Ext(p) {
			case ".md", ".sh", ".go", ".json":
				out = append(out, p)
			}
			return nil
		})
	}
	docs, _ := filepath.Glob(filepath.Join(root, "docs", "*.md"))
	out = append(out, docs...)
	return append(out, filepath.Join(root, "README.md"), filepath.Join(root, "CLAUDE.md"))
}

// skillAsset matches a path a SKILL.md hands the session to open for itself:
// the assets, references and scripts that ship beside it and are never
// injected with it.
var skillAsset = regexp.MustCompile("`(assets/[^`]+|references/[^`]+|scripts/[^`]+)`")

// TestEverySkillAssetItNamesIsThere: an asset is reached by the session
// following a path this file gave it, and a path that resolves to nothing
// looks from there exactly like a file it was not shown. The corpus lost the
// issue taxonomy and template this way — deleted with the machinery around
// them, named by nothing that ran.
func TestEverySkillAssetItNamesIsThere(t *testing.T) {
	root := repoRoot(t)
	dirs, err := os.ReadDir(filepath.Join(root, "skills"))
	if err != nil {
		t.Fatal(err)
	}
	named := 0
	var problems []string
	for _, d := range dirs {
		if !d.IsDir() {
			continue
		}
		skill := filepath.Join(root, "skills", d.Name(), "SKILL.md")
		raw, err := os.ReadFile(skill)
		if err != nil {
			t.Errorf("skills/%s: %v", d.Name(), err)
			continue
		}
		for i, line := range strings.Split(string(raw), "\n") {
			for _, m := range skillAsset.FindAllStringSubmatch(line, -1) {
				named++
				// A Skill names a path either beside itself or from the
				// checkout root; a session opens whichever is there.
				beside := filepath.Join(root, "skills", d.Name(), m[1])
				fromRoot := filepath.Join(root, m[1])
				if !exists(beside) && !exists(fromRoot) {
					problems = append(problems, "skills/"+d.Name()+"/SKILL.md:"+strconv.Itoa(i+1)+
						" names "+m[1]+", which is at neither path a session would open")
				}
			}
		}
	}
	// A corpus that names no asset reads here exactly like one whose assets
	// all resolve.
	if named == 0 {
		t.Fatal("no asset path was examined; this guard would pass against a corpus that ships none")
	}
	sort.Strings(problems)
	for _, p := range problems {
		t.Error(p)
	}
}

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
