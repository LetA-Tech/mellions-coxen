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

// A check that leaves a directory behind is a check that makes every session
// obeying "give back what you borrowed" do it by hand, which means most will
// not. `hooks/test-session-renewal.sh` built a 4.9 MB binary into a
// `mktemp -d` whose path it never bound to a name, and removed it with a bare
// `rm -rf` on the last line — so the build-failure exit and any signal
// orphaned it. A bare `rm -rf` reads as cleanup and is not one: it covers the
// happy path only.
var (
	mktempCall = regexp.MustCompile(`(\w+)=\$\(mktemp -d\)`)
	trapLine   = regexp.MustCompile(`(?m)^\s*trap\s+'[^']*rm -rf "\$(\w+)"`)
)

// TestEveryTempDirAHookTestMakesIsTrapped walks hooks/ rather than naming the
// three scripts that have one today, so a fourth is held to it on the commit
// that adds it.
func TestEveryTempDirAHookTestMakesIsTrapped(t *testing.T) {
	dir := filepath.Join(repoRoot(t), "hooks")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading hooks/: %v", err)
	}

	checked := 0
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".sh") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("reading %s: %v", e.Name(), err)
		}
		body := string(b)

		// The unbound form is the defect itself: the directory cannot be
		// removed later because nothing kept its path.
		if strings.Contains(body, `$(mktemp -d)/`) {
			t.Errorf("hooks/%s: mktemp -d used inline, so the directory's own "+
				"path is thrown away and nothing can remove it; assign it to a "+
				"variable and trap that", e.Name())
		}

		trapped := map[string]bool{}
		for _, m := range trapLine.FindAllStringSubmatch(body, -1) {
			trapped[m[1]] = true
		}
		for _, m := range mktempCall.FindAllStringSubmatch(body, -1) {
			checked++
			if !trapped[m[1]] {
				t.Errorf("hooks/%s: $%s holds a mktemp -d with no "+
					`trap 'rm -rf "$%s"' EXIT INT TERM HUP, so a failing exit `+
					"or a signal leaves it on disk", e.Name(), m[1], m[1])
			}
		}
	}

	// A regexp that matches nothing reports every repository clean.
	if checked == 0 {
		t.Fatal("no mktemp -d found under hooks/ — this guard is matching " +
			"nothing and would pass against the defect it exists to catch")
	}
}
