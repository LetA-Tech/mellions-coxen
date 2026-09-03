// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func skillsFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write := func(name, desc string) {
		dir := filepath.Join(root, name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		doc := "---\nname: " + name + "\ndescription: " + desc + "\n---\n\n# " + name + "\n"
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(doc), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("mellions-falsification", `"Load this before a green run is cited as proof that a fix holds. Triggers — \"falsify\", \"revert arm\"."`)
	write("mellions-territory", `"Load this before removing or reverting anything you did not write, or when another lane may hold the same worktree."`)
	return root
}

func skillsOut(t *testing.T, root string, args ...string) string {
	t.Helper()
	return captureStdout(t, func() {
		if err := cmdSkills(context.Background(), append([]string{"-dir", root}, args...)); err != nil {
			t.Fatalf("cmdSkills(%q): %v", args, err)
		}
	})
}

// The list is the toolbox: every method, what each is for, and how to load one.
// The utterance triggers are cut — unattended work has no utterances, and what
// a session matches against is the situation it is in.
func TestSkillsListsEveryMethodWithoutItsUtterances(t *testing.T) {
	out := skillsOut(t, skillsFixture(t))
	for _, want := range []string{"mellions:mellions-falsification", "mellions:mellions-territory",
		"cited as proof", `Skill(skill: "mellions:<name>")`} {
		if !strings.Contains(out, want) {
			t.Errorf("the list does not carry %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "revert arm") {
		t.Errorf("the list carries the utterance triggers:\n%s", out)
	}
}

// Asked in the words of the work rather than in the description's own sentence,
// it still answers — and a clear winner comes back whole, with the call that
// loads it.
func TestSkillsAnswersTheQuestionInTheWordsItIsAsked(t *testing.T) {
	// Both methods match some of this; territory matches most of it, and sorts
	// after falsification by name — so an unranked answer returns the wrong one.
	out := skillsOut(t, skillsFixture(t), "another", "lane", "may", "hold", "this", "worktree", "before", "reverting")
	if !strings.Contains(out, "# mellions:mellions-territory") {
		t.Errorf("the query did not reach the territory method:\n%s", out)
	}
	if !strings.Contains(out, `Skill(skill: "mellions:mellions-territory")`) {
		t.Errorf("the answer does not say how to load it:\n%s", out)
	}
	if strings.Contains(out, "mellions-falsification") {
		t.Errorf("a method that answers the question less well was printed anyway:\n%s", out)
	}
}

// The other way a toolbox comes back empty: the directory has methods in it and
// none declares what it is for. Skipping them silently prints a shorter list
// than the installation carries, which is the answer nobody can tell from a
// correct one.
func TestSkillsRefusesADirectoryWhoseMethodsDeclareNothing(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "mellions-nameless")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("# no front matter here\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := cmdSkills(context.Background(), []string{"-dir", root}); err == nil {
		t.Fatal("a directory whose only method declares nothing printed a toolbox")
	}
}

// A toolbox with nothing in it must say so. Printing an empty list would read
// exactly like an installation that carries no methods, which is the one answer
// this must never give silently.
func TestSkillsRefusesToReportAnEmptyToolbox(t *testing.T) {
	err := cmdSkills(context.Background(), []string{"-dir", t.TempDir()})
	if err == nil {
		t.Fatal("an empty skills directory printed a toolbox instead of saying it found none")
	}
	if !strings.Contains(err.Error(), "doctor") {
		t.Errorf("the error does not say what establishes the load path: %v", err)
	}
}
