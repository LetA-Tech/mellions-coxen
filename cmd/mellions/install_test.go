// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package main

import (
	"strings"
	"testing"
)

func TestPublishedSourceIsThePublicRepository(t *testing.T) {
	if publishedSource != "LetA-Tech/mellions-coxen" {
		t.Fatalf("publishedSource = %q, want the permanent public repository", publishedSource)
	}
}

// TestInstallingClearsWhatIsThereBeforeAddingTheSource.
//
// Both runtimes cache a plugin under the version in its manifest and will not
// re-read the source while that version is unchanged — `claude plugin update`
// answers "already at the latest version (0.1.0)" and refreshes nothing. A
// deployment from a local checkout found this the expensive way: the binary
// updated, the plugin did not, and both runtimes reported success.
//
// The plugin and the marketplace are two objects and both have to go. Clearing
// only the plugin left the marketplace bound to its old source, so installing
// from anywhere else failed outright.
//
// Clearing may fail — removing something that was never there is not an error.
// Adding may not: a silent failure there is an install that reports success over
// the previous content, which is the one outcome an installer must not have.
func TestInstallingClearsWhatIsThereBeforeAddingTheSource(t *testing.T) {
	clearing := func(s string) bool {
		return strings.Contains(s, "remove") || strings.Contains(s, "uninstall")
	}
	for _, a := range adapters {
		steps := a.steps("/some/checkout")
		var cleared, added []string
		seenAdd := false
		for _, st := range steps {
			line := strings.Join(st.args, " ")
			if clearing(line) {
				if seenAdd {
					t.Errorf("%s: %q clears after something was added", a.name, line)
				}
				if !st.spent {
					t.Errorf("%s: %q would fail the install when there is nothing to clear",
						a.name, line)
				}
				cleared = append(cleared, line)
				continue
			}
			seenAdd = true
			if st.spent {
				t.Errorf("%s: %q may not fail silently", a.name, line)
			}
			added = append(added, line)
		}

		if len(cleared) < 2 {
			t.Errorf("%s clears %v — the plugin and the marketplace are two objects and both "+
				"have to go, or a new source cannot replace an old one", a.name, cleared)
		}
		if !strings.Contains(strings.Join(cleared, " "), "marketplace") {
			t.Errorf("%s never drops the marketplace, so installing from a different source "+
				"fails with the marketplace still bound to the old one", a.name)
		}
		if len(added) < 2 {
			t.Errorf("%s adds %v; expected the marketplace and the plugin", a.name, added)
		}
	}
}
