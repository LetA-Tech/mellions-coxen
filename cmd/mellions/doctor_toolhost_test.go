package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A Codex installation missing codex-code-mode-host answers `codex --version`
// and `codex plugin list` normally while failing every tool call closed, so the
// only thing that separates it from a working one is the helper on disk.
func TestCodexToolHost(t *testing.T) {
	for _, tc := range []struct {
		name  string
		setup func(dir string) error
		want  bool
	}{
		{
			name:  "absent",
			setup: func(string) error { return nil },
			want:  false,
		},
		{
			name: "present and executable",
			setup: func(dir string) error {
				return os.WriteFile(filepath.Join(dir, "codex-code-mode-host"), []byte("#!/bin/sh\n"), 0o755)
			},
			want: true,
		},
		{
			name: "present, not executable",
			setup: func(dir string) error {
				return os.WriteFile(filepath.Join(dir, "codex-code-mode-host"), []byte("#!/bin/sh\n"), 0o644)
			},
			want: false,
		},
		{
			name: "a directory of that name",
			setup: func(dir string) error {
				return os.Mkdir(filepath.Join(dir, "codex-code-mode-host"), 0o755)
			},
			want: false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			bin := filepath.Join(dir, "codex")
			if err := os.WriteFile(bin, []byte("#!/bin/sh\n"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := tc.setup(dir); err != nil {
				t.Fatal(err)
			}
			got, ok := codexToolHost(bin)
			if ok != tc.want {
				t.Errorf("codexToolHost(%q) = %v, want %v", bin, ok, tc.want)
			}
			if want := filepath.Join(dir, "codex-code-mode-host"); got != want {
				t.Errorf("path = %q, want %q", got, want)
			}
		})
	}
}

func TestCodexToolHostInNPMPlatformPackage(t *testing.T) {
	packageName, target := codexPlatformPackage()
	if packageName == "" {
		t.Skip("no managed Codex package mapping for this platform")
	}
	root := t.TempDir()
	packageRoot := filepath.Join(root, "node_modules", "@openai", "codex")
	launcher := filepath.Join(packageRoot, "bin", "codex.js")
	if err := os.MkdirAll(filepath.Dir(launcher), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(launcher, []byte("#!/usr/bin/env node\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	visible := filepath.Join(root, "bin", "codex")
	if err := os.MkdirAll(filepath.Dir(visible), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(launcher, visible); err != nil {
		t.Fatal(err)
	}
	host := filepath.Join(packageRoot, "node_modules", "@openai", packageName,
		"vendor", target, "bin", "codex-code-mode-host")
	if err := os.MkdirAll(filepath.Dir(host), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(host, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	got, ok := codexToolHost(visible)
	if !ok || !sameDir(got, host) {
		t.Fatalf("codexToolHost(npm launcher) = %q, %v; want %q, true", got, ok, host)
	}
}

// The helper being right proves nothing on its own: doctor's answer is what a
// person reads, and it is `codex plugin list` that decides it. Driving
// pluginState through a stub codex is what shows the missing helper reaching
// the line instead of being computed and dropped.
func TestPluginStateCodexReportsMissingToolHost(t *testing.T) {
	stub := func(t *testing.T, withHost bool) string {
		t.Helper()
		dir := t.TempDir()
		bin := filepath.Join(dir, "codex")
		script := "#!/bin/sh\necho 'PLUGIN             STATUS              VERSION  PATH'\n" +
			"echo 'mellions@mellions  installed, enabled  0.1.0    /home/you/mellions-coxen'\n"
		if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
		if withHost {
			if err := os.WriteFile(filepath.Join(dir, "codex-code-mode-host"), []byte("#!/bin/sh\n"), 0o755); err != nil {
				t.Fatal(err)
			}
		}
		return bin
	}

	t.Run("helper missing is not present", func(t *testing.T) {
		state, detail := pluginState("codex", stub(t, false))
		if state == "present" {
			t.Fatalf("state = %q: an installation that fails every tool call closed reported healthy", state)
		}
		if !strings.Contains(detail, "codex-code-mode-host") {
			t.Errorf("detail = %q, want it to name the missing helper", detail)
		}
	})

	t.Run("helper present is present", func(t *testing.T) {
		state, _ := pluginState("codex", stub(t, true))
		if state != "present" {
			t.Fatalf("state = %q, want %q: a working installation must not be reported broken", state, "present")
		}
	})
}
