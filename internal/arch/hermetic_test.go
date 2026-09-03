// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package arch

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func buildBin(t *testing.T, root, out string) {
	t.Helper()
	cmd := exec.Command("go", "build", "-o", out, "./cmd/mellions")
	cmd.Dir = root
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build the binary under test: %v\n%s", err, b)
	}
}

// runBin runs the built binary with an environment stripped of everything that
// could point it back at the operator's own state, plus the pins under test.
func runBin(t *testing.T, bin, dir string, pins []string, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	cmd.Env = append([]string{"PATH=" + os.Getenv("PATH")}, pins...)
	b, err := cmd.CombinedOutput()
	return string(b), err
}

// TestHookTestsThatRunTheBinaryPinTheirOwnState.
//
// A hook test that execs the binary makes it resolve a config, and the
// resolution ends at $HOME/.mellions/config.json: left inherited, the test
// reads whichever config the operator has, creates that config's
// assignments_root, and lists the operator's live assignments. Two consequences
// — the result depends on the machine it runs on, and on a host with no
// ~/.mellions the test fails for a reason that has nothing to do with the hook.
//
// So a test script that reaches the binary has to pin both: MELLIONS_CONFIG,
// because it is the first candidate the binary reads, and HOME, because every
// config key that is absent falls back to it again.
func TestHookTestsThatRunTheBinaryPinTheirOwnState(t *testing.T) {
	root := repoRoot(t)
	scripts, err := filepath.Glob(filepath.Join(root, "hooks", "test-*.sh"))
	if err != nil || len(scripts) == 0 {
		t.Fatalf("no hooks/test-*.sh found under %s (err %v) — this guard would pass by matching nothing", root, err)
	}
	checked := 0
	for _, p := range scripts {
		raw, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		s := string(raw)
		// The binary is reached one of two ways: built and handed to the hook
		// as MELLIONS_BIN, or invoked directly.
		if !strings.Contains(s, "MELLIONS_BIN") && !strings.Contains(s, "./cmd/mellions") {
			continue
		}
		checked++
		name := filepath.Base(p)
		for _, pin := range []string{"MELLIONS_CONFIG=", "HOME="} {
			if !strings.Contains(s, "export "+pin) && !strings.Contains(s, pin) {
				t.Errorf("hooks/%s runs the binary and never pins %s: it will read the operator's own state, and fail on a host that has none", name, strings.TrimSuffix(pin, "="))
			}
		}
	}
	if checked == 0 {
		t.Fatal("no hook test was found to run the binary — the check matched nothing and would report any tree clean")
	}
}

// TestTheHermeticPinSurvivesAnEmptyHome.
//
// The pin above is a string in a script; that it is present says nothing about
// whether the binary honours it. This runs the binary the way the hook does,
// with HOME pointing at an empty directory and MELLIONS_CONFIG at a config of
// its own, and requires it to work — which it can only do by reading the pinned
// config rather than the one under a home that has nothing in it.
func TestTheHermeticPinSurvivesAnEmptyHome(t *testing.T) {
	root := repoRoot(t)
	dir := t.TempDir()
	bin := filepath.Join(dir, "mellions")
	buildBin(t, root, bin)

	home := filepath.Join(dir, "home")
	state := filepath.Join(dir, "state")
	cfg := filepath.Join(dir, "config.json")
	for _, d := range []string{home, state} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	body := `{"owner":"test","assignments_root":"` + state + `/assignments","report_root":"` + state + `"}`
	if err := os.WriteFile(cfg, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := runBin(t, bin, dir, []string{"HOME=" + home, "MELLIONS_CONFIG=" + cfg}, "assign", "list")
	if err != nil {
		t.Fatalf("`mellions assign list` under an empty HOME failed: %v\n%s", err, out)
	}
	// The proof that the pinned config was the one read: the state root it
	// names now exists, and the empty home is still empty.
	if _, err := os.Stat(filepath.Join(state, "assignments")); err != nil {
		t.Fatalf("the pinned assignments_root was not the one used: %v", err)
	}
	entries, err := os.ReadDir(home)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Fatalf("the run wrote into the pinned HOME: %v — with the real HOME it would have written there", names)
	}
}
