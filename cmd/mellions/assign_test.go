// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package main

import "testing"

// TestAssignOpenTakesTheIdEitherWay.
//
// Every other assign verb takes the id as an argument, so a session that has
// written `assign record <id>` and `assign handoff <id>` writes `assign open
// <id>` too. Open read the id only from -id and parsed with flag.Parse, which
// stops at the first argument: the id went nowhere and so did every flag after
// it, so the objective and the reason the work was chosen were dropped in
// silence and the refusal named a repository that had in fact been given.
func TestAssignOpenTakesTheIdEitherWay(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		id   string
	}{
		{"positional", []string{"frontend-42"}, "frontend-42"},
		{"flag", []string{"-id", "frontend-42"}, "frontend-42"},
		{"both agreeing", []string{"frontend-42", "-id", "frontend-42"}, "frontend-42"},
		{"positional between flags", []string{"-repo", "r", "frontend-42", "-branch", "b"}, "frontend-42"},
		{"neither", nil, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, o, err := parseOpen(tc.args)
			if err != nil {
				t.Fatalf("parseOpen(%q): %v", tc.args, err)
			}
			if o.ID != tc.id {
				t.Errorf("id = %q, want %q", o.ID, tc.id)
			}
		})
	}
}

// TestAssignOpenReadsTheFlagsAfterThePositionalId is the rest of the same
// defect: the id was not the only argument flag.Parse abandoned at the first
// non-flag. A fix that only found the id would leave open refusing work whose
// objective and reason were on the command line all along.
func TestAssignOpenReadsTheFlagsAfterThePositionalId(t *testing.T) {
	cfg, o, err := parseOpen([]string{
		"frontend-42", "-repo", "frontend-app", "-issue", "#42",
		"-objective", "retire the legacy flow", "-because", "the owner asked for it",
		"-config", "/tmp/c.json", "-budget", "4h",
	})
	if err != nil {
		t.Fatalf("parseOpen: %v", err)
	}
	for _, got := range []struct{ field, have, want string }{
		{"id", o.ID, "frontend-42"},
		{"repo", o.Repo, "frontend-app"},
		{"issue", o.Issue, "#42"},
		{"objective", o.Objective, "retire the legacy flow"},
		{"because", o.Because, "the owner asked for it"},
		{"config", cfg, "/tmp/c.json"},
	} {
		if got.have != got.want {
			t.Errorf("%s = %q, want %q", got.field, got.have, got.want)
		}
	}
	if o.Budget.Wall.Hours() != 4 {
		t.Errorf("budget = %v, want 4h", o.Budget.Wall)
	}
}

// TestAssignOpenRefusesTwoIds. Silently preferring one would cut a branch and a
// worktree under a name the session did not type, and the record it wrote would
// be found under neither id the next session looked for.
func TestAssignOpenRefusesTwoIds(t *testing.T) {
	for _, args := range [][]string{
		{"frontend-42", "-id", "frontend-43"},
		{"frontend-42", "frontend-43"},
	} {
		if _, o, err := parseOpen(args); err == nil {
			t.Errorf("parseOpen(%q) accepted two ids and chose %q", args, o.ID)
		}
	}
}
