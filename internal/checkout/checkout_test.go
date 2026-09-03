// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package checkout

import (
	"os"
	"path/filepath"
	"testing"
)

func repo(t *testing.T, parent, name string) string {
	t.Helper()
	dir := filepath.Join(parent, name)
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

// TestAnInstallationCanSeeMoreThanOneRoot.
//
// One root meant an estate whose work is split across two parents had to choose
// which half the engineer was allowed to know about — and Mellions, living
// beside the repositories it works on rather than among them, could not manage
// its own.
func TestAnInstallationCanSeeMoreThanOneRoot(t *testing.T) {
	a, b := t.TempDir(), t.TempDir()
	repo(t, a, "policy-service")
	repo(t, b, "mellions-coxen")

	got := Resolve([]string{a, b}, nil, []string{"policy-service", "mellions-coxen"})
	if len(got) != 2 {
		t.Fatalf("resolved %d of 2 repositories across two roots: %v", len(got), got)
	}
	if d, _ := got.Dir("mellions-coxen"); d != filepath.Join(b, "mellions-coxen") {
		t.Errorf("mellions-coxen resolved to %q", d)
	}
}

// TestTheFirstRootHoldingANameWins. Ordering roots is how an installation says
// which copy is authoritative when the same name exists in two.
func TestTheFirstRootHoldingANameWins(t *testing.T) {
	a, b := t.TempDir(), t.TempDir()
	repo(t, a, "shared")
	repo(t, b, "shared")

	if d, _ := Resolve([]string{a, b}, nil, []string{"shared"}).Dir("shared"); d != filepath.Join(a, "shared") {
		t.Errorf("resolved to %q, want the first root", d)
	}
	set, err := Discover(a, b)
	if err != nil {
		t.Fatal(err)
	}
	if set["shared"] != filepath.Join(a, "shared") {
		t.Errorf("discovery took %q, want the first root", set["shared"])
	}
}

// TestATrackerNameThatIsNotADirectoryNameStillResolves.
//
// A checkout directory can differ from its tracker repository name, so that
// mapping must be explicit.
func TestATrackerNameThatIsNotADirectoryNameStillResolves(t *testing.T) {
	root := t.TempDir()
	dir := repo(t, root, "memory-service-source")

	if _, ok := Resolve([]string{root}, nil, []string{"memory-service"}).Dir("memory-service"); ok {
		t.Fatal("memory-service resolved without being told where it is")
	}
	got, ok := Resolve([]string{root}, map[string]string{"memory-service": dir}, []string{"memory-service"}).Dir("memory-service")
	if !ok || got != dir {
		t.Errorf("memory-service resolved to %q, %v — want %q", got, ok, dir)
	}
}

// TestASymlinkedCheckoutIsACheckout.
//
// A directory entry reports a symlink as not-a-directory, so the walker this
// replaces skipped linked checkouts silently — indistinguishable from a
// repository nobody configured, which is the failure mode that costs the most
// to notice.
func TestASymlinkedCheckoutIsACheckout(t *testing.T) {
	real, root := t.TempDir(), t.TempDir()
	target := repo(t, real, "actual")
	link := filepath.Join(root, "linked")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	set, err := Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := set.Dir("linked"); !ok {
		t.Error("a symlinked checkout was skipped, and nothing said so")
	}
}

// TestARepositoryThatResolvesNowhereIsNamedRatherThanGuessed.
func TestARepositoryThatResolvesNowhereIsNamedRatherThanGuessed(t *testing.T) {
	root := t.TempDir()
	repo(t, root, "present")

	set := Resolve([]string{root}, nil, []string{"present", "absent"})
	if _, ok := set.Dir("absent"); ok {
		t.Error("a repository with no checkout resolved to something")
	}
	miss := Missing(set, []string{"present", "absent"})
	if len(miss) != 1 || miss[0] != "absent" {
		t.Errorf("Missing = %v, want [absent]", miss)
	}
}

// TestUnreadableRootsAreAnErrorNotAnEmptyEstate.
func TestUnreadableRootsAreAnErrorNotAnEmptyEstate(t *testing.T) {
	if _, err := Discover(filepath.Join(t.TempDir(), "nope")); err == nil {
		t.Error("discovery over a root that does not exist returned no error")
	}
}
