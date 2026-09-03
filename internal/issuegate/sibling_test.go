// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package issuegate

import (
	"os"
	"path/filepath"
	"testing"
)

func mkfile(t *testing.T, root, repo, rel string) {
	t.Helper()
	p := filepath.Join(root, repo, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("package x\n\n\n\n\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
}

// TestACitationIntoASiblingRepositoryIsNotAMovedPremise.
//
// A cross-repository issue may cite a sibling path without naming the sibling.
// Resolve across known checkouts before classifying the citation as moved.
func TestACitationIntoASiblingRepositoryIsNotAMovedPremise(t *testing.T) {
	root := t.TempDir()
	mkfile(t, root, "frontend", "app/main.ts")
	mkfile(t, root, "edge-service", "internal/edge/service/models/models.go")
	known := map[string]string{
		"frontend":     filepath.Join(root, "frontend"),
		"edge-service": filepath.Join(root, "edge-service"),
	}

	// Cited from frontend, living in edge-service, named without a repository.
	got, ok := Locate(Citation{Path: "internal/edge/service/models/models.go", Line: 2}, "frontend", known)
	if !ok {
		t.Fatal("a path present in a sibling checkout read as absent")
	}
	if got != filepath.Join(root, "edge-service", "internal/edge/service/models/models.go") {
		t.Errorf("resolved to %q, want the sibling's copy", got)
	}

	// The issue's own repository still wins when it holds the path.
	if got, _ := Locate(Citation{Path: "app/main.ts", Line: 1}, "frontend", known); got !=
		filepath.Join(root, "frontend", "app/main.ts") {
		t.Errorf("resolved to %q, want the issue's own repository", got)
	}

	// Genuinely absent everywhere is still absent — the signal is not all noise.
	if _, ok := Locate(Citation{Path: "store/gone.ts", Line: 1}, "frontend", known); ok {
		t.Error("a path in no checkout resolved to something")
	}
}

// TestTwoSiblingsHoldingOnePathIsAmbiguousRatherThanGuessed.
func TestTwoSiblingsHoldingOnePathIsAmbiguousRatherThanGuessed(t *testing.T) {
	root := t.TempDir()
	mkfile(t, root, "a", "internal/shared/thing.go")
	mkfile(t, root, "b", "internal/shared/thing.go")
	mkfile(t, root, "here", "own.go")
	known := map[string]string{
		"a": filepath.Join(root, "a"), "b": filepath.Join(root, "b"),
		"here": filepath.Join(root, "here"),
	}
	if _, ok := Locate(Citation{Path: "internal/shared/thing.go", Line: 1}, "here", known); ok {
		t.Error("two checkouts hold that path and one was picked, which invents a finding")
	}
}

// TestAnExplicitRepositoryIsNotOverridden. Where the citation names its
// repository, that is the claim and searching elsewhere would contradict it.
func TestAnExplicitRepositoryIsNotOverridden(t *testing.T) {
	root := t.TempDir()
	mkfile(t, root, "other", "internal/x.go")
	mkfile(t, root, "here", "own.go")
	known := map[string]string{"other": filepath.Join(root, "other"), "here": filepath.Join(root, "here")}
	if _, ok := Locate(Citation{Repo: "here", Path: "internal/x.go", Line: 1}, "here", known); ok {
		t.Error("a citation that named its own repository resolved against a different one")
	}
}

// TestAnElidedPathIsProseNotAClaim.
//
// People write `internal/.../period_close.go` to mean "somewhere under
// internal". Treating it as a path this repository should hold reported a moved
// premise about a file nobody ever named.
func TestAnElidedPathIsProseNotAClaim(t *testing.T) {
	root := t.TempDir()
	mkfile(t, root, "here", "internal/edge/period_close.go")
	known := map[string]string{"here": filepath.Join(root, "here")}
	for _, path := range []string{
		"internal/.../period_close.go",
		"internal/…/period_close.go",
	} {
		if _, ok := Locate(Citation{Path: path, Line: 1}, "here", known); ok {
			t.Errorf("%q resolved to something, which would make it checkable", path)
		}
	}
}

// TestTheTailOfAPathResolvesTheWayABasenameDoes.
//
// Once a file has been named in full, people cite its tail. Same rule as a
// basename: one match answers, more than one is uncheckable.
func TestTheTailOfAPathResolvesTheWayABasenameDoes(t *testing.T) {
	root := t.TempDir()
	mkfile(t, root, "here", "own.go")
	mkfile(t, root, "edge", "internal/edge/service/periodclose/models.go")
	known := map[string]string{"here": filepath.Join(root, "here"), "edge": filepath.Join(root, "edge")}

	got, ok := Locate(Citation{Path: "periodclose/models.go", Line: 1}, "here", known)
	if !ok {
		t.Fatal("the tail of a real path read as absent")
	}
	if got != filepath.Join(root, "edge", "internal/edge/service/periodclose/models.go") {
		t.Errorf("resolved to %q", got)
	}

	// Two files sharing a tail stays uncheckable rather than guessed.
	mkfile(t, root, "other", "internal/x/periodclose/models.go")
	known["other"] = filepath.Join(root, "other")
	if _, ok := Locate(Citation{Path: "periodclose/models.go", Line: 1}, "here", known); ok {
		t.Error("an ambiguous tail was resolved, which invents a finding")
	}
}

// TestANestedCheckoutIsNotASecondCopy.
//
// A nested worktree is another copy of the same repository, not a sibling
// candidate that makes each citation ambiguous with itself.
func TestANestedCheckoutIsNotASecondCopy(t *testing.T) {
	root := t.TempDir()
	mkfile(t, root, "app", "features/bank/store/useBankConnectionsStore.ts")
	// The same repository, checked out again inside itself.
	mkfile(t, root, "app/.worktrees/fix-1403", "features/bank/store/useBankConnectionsStore.ts")
	known := map[string]string{"app": filepath.Join(root, "app")}

	got, ok := Locate(Citation{Path: "store/useBankConnectionsStore.ts", Line: 1}, "app", known)
	if !ok {
		t.Fatal("a file present exactly where the issue said read as a moved premise, " +
			"because a nested worktree held a second copy of it")
	}
	if filepath.Base(filepath.Dir(filepath.Dir(got))) == ".worktrees" {
		t.Errorf("resolved into the nested checkout: %s", got)
	}
}
