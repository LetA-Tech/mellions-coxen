// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/LetA-Tech/mellions-coxen/internal/pluginreg"
)

// method is one Skill this installation carries: the name a session loads it
// by, and the situation that calls for it.
type method struct {
	Name  string
	When  string
	Path  string
	Whole string
}

// cmdSkills answers, mid-task and without loading anything, what methods exist
// and which one this situation wants.
//
// The catalog a session is handed at startup is awareness, and awareness decays:
// it is read once, at minute zero, against work that has not happened yet, and
// what a session needs at minute twenty is an answer to a question it now has.
// A catalog cannot be asked a question. This can, and it costs nothing until it
// is — which is the whole reason it is a command and not more startup text.
//
// It prints what to load and never loads anything. Deciding that a method
// applies is the engineer's; a tool that decided it would be back to spending
// context on methods nobody asked for.
func cmdSkills(ctx context.Context, args []string) error {
	fs := newFlagSet("skills", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	dir := fs.String("dir", "", "the skills directory to read, when it is not this installation's")
	if err := fs.Parse(args); err != nil {
		return err
	}
	root := *dir
	if root == "" {
		root = skillsDir()
	}
	all, err := readMethods(root)
	if err != nil {
		return err
	}
	query := strings.ToLower(strings.TrimSpace(strings.Join(fs.Args(), " ")))
	if query == "" {
		fmt.Printf("# %d methods, in %s\n\n", len(all), root)
		fmt.Println("Load one with the Skill tool when the situation it names arrives, and not before:")
		fmt.Println("`Skill(skill: \"mellions:<name>\")`. `mellions skills <what you are doing>` narrows this.")
		fmt.Println()
		for _, m := range all {
			fmt.Printf("- **mellions:%s** — %s\n", m.Name, m.When)
		}
		return nil
	}

	// Ranked by how many of the words land, not by the phrase and not by all of
	// them: a session asks in the words of what it is doing, which is never the
	// description's own sentence. Demanding every word returns nothing while the
	// method it wanted sits in the list; the short words are dropped because
	// the words below are in every description and rank nothing.
	var words []string
	for _, w := range strings.Fields(query) {
		if len(w) >= 3 && !common[w] {
			words = append(words, w)
		}
	}
	if len(words) == 0 {
		words = strings.Fields(query)
	}
	score := map[string]int{}
	var hit []method
	for _, m := range all {
		hay := strings.ToLower(m.Name + " " + m.Whole)
		n := 0
		for _, w := range words {
			if strings.Contains(hay, w) {
				n++
			}
		}
		if n > 0 {
			score[m.Name] = n
			hit = append(hit, m)
		}
	}
	sort.SliceStable(hit, func(i, j int) bool { return score[hit[i].Name] > score[hit[j].Name] })
	if len(hit) == 0 {
		fmt.Printf("No method here names %q. `mellions skills` lists all %d.\n", query, len(all))
		return nil
	}
	// One clear winner is printed whole; a tie is a list, because choosing
	// between two methods is the reader's and a full description of the wrong
	// one is the context this command exists to not spend.
	if len(hit) == 1 || (len(hit) > 1 && score[hit[0].Name] > score[hit[1].Name]) {
		m := hit[0]
		fmt.Printf("# mellions:%s\n\n%s\n\n", m.Name, m.Whole)
		fmt.Printf("Load it: `Skill(skill: \"mellions:%s\")`\nIt is %s\n", m.Name, m.Path)
		return nil
	}
	fmt.Printf("%d of %d methods name %q, closest first:\n\n", len(hit), len(all), query)
	if len(hit) > 6 {
		hit = hit[:6]
	}
	for _, m := range hit {
		fmt.Printf("- **mellions:%s** — %s\n", m.Name, m.When)
	}
	fmt.Printf("\nLoad one with `Skill(skill: \"mellions:<name>\")`.\n")
	return nil
}

// skillsDir is the skills directory of the checkout the runtime loads from,
// which is the one whose methods a session would actually receive. A hook has
// already been told where that is.
func skillsDir() string {
	if root := os.Getenv("MELLIONS_SKILLS_DIR"); root != "" {
		return root
	}
	return filepath.Join(pluginRoot(pluginreg.Read(home(), pluginreg.ID)), "skills")
}

func readMethods(root string) ([]method, error) {
	entries, err := filepath.Glob(filepath.Join(root, "*", "SKILL.md"))
	if err != nil || len(entries) == 0 {
		return nil, errors.New("skills: no SKILL.md under " + root +
			"; `mellions doctor` says which checkout the runtime loads from")
	}
	var out []method
	for _, p := range entries {
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		whole := frontMatter(string(b), "description")
		if whole == "" {
			continue
		}
		out = append(out, method{
			Name:  filepath.Base(filepath.Dir(p)),
			When:  situation(whole),
			Path:  p,
			Whole: whole,
		})
	}
	if len(out) == 0 {
		return nil, errors.New("skills: no SKILL.md under " + root + " declares a description")
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// frontMatter reads one key out of a SKILL.md's YAML front matter, joining the
// continuation lines a long description is wrapped across.
func frontMatter(doc, key string) string {
	lines := strings.Split(doc, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return ""
	}
	var b strings.Builder
	reading := false
	for _, ln := range lines[1:] {
		if strings.TrimSpace(ln) == "---" {
			break
		}
		if strings.HasPrefix(ln, key+":") {
			reading = true
			b.WriteString(strings.TrimSpace(strings.TrimPrefix(ln, key+":")))
			continue
		}
		if !reading {
			continue
		}
		// A wrapped value is indented; the next key is not.
		if ln == "" || !strings.HasPrefix(ln, " ") && !strings.HasPrefix(ln, "\t") {
			break
		}
		b.WriteString(" " + strings.TrimSpace(ln))
	}
	return strings.Trim(strings.TrimSpace(b.String()), `"'`)
}

// common words appear in every method's description, so they rank nothing and
// searching on them returns the whole catalog, which is the catalog.
var common = map[string]bool{
	"the": true, "this": true, "that": true, "and": true, "for": true, "with": true,
	"when": true, "what": true, "where": true, "from": true, "have": true, "has": true,
	"can": true, "you": true, "your": true, "are": true, "was": true, "not": true,
	"any": true, "one": true, "its": true, "his": true, "her": true, "them": true,
	"work": true, "load": true, "use": true, "using": true, "about": true,
}

// situation is the description without its utterance triggers. Unattended work
// has no utterances; what a session matches against is the situation it is in.
func situation(desc string) string {
	for _, cut := range []string{" Triggers — ", " Do NOT use "} {
		if i := strings.Index(desc, cut); i >= 0 {
			desc = desc[:i]
		}
	}
	return strings.TrimSpace(desc)
}
