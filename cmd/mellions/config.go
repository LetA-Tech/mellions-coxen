// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/LetA-Tech/mellions-coxen/internal/checkout"
	"github.com/LetA-Tech/mellions-coxen/internal/durable"
)

func cmdConfig(args []string) error {
	if len(args) == 0 {
		return errors.New("config needs a verb: init, show, path, home or reports")
	}
	fs := newFlagSet("config", flag.ContinueOnError)
	cfgPath := fs.String("config", "", "config file")
	at := fs.String("at", "", "where init writes; default ~/.mellions/config.json")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if args[0] == "init" {
		return configInit(*at)
	}
	cfg, err := loadConfig(*cfgPath)
	if err != nil {
		return err
	}
	switch args[0] {
	case "path":
		fmt.Println(cfg.path)
	// One line, no heading, nothing else on stdout: these two are what
	// scripts/shift.sh and scripts/shifts.sh ask so that a shift's home and the
	// report writer's directory are the binary's, never a second default of
	// their own. A script that computes either itself is how one host ends up
	// with two homes.
	case "home":
		fmt.Println(cfg.home())
	case "reports":
		fmt.Println(cfg.reportsDir())
	case "show":
		return configShow(cfg)
	default:
		return fmt.Errorf("config: unknown verb %q", args[0])
	}
	return nil
}

func configShow(cfg *Config) error {
	fmt.Printf("# Mellions configuration — %s\n\n", cfg.path)
	fmt.Print("What the estate is and where its records live. Permissions, tools, hooks,\n" +
		"sandboxing, credentials and model settings belong to the runtime and are\n" +
		"inherited unchanged; what is delegated to the engineer is stated in the\n" +
		"partnership, in the owner's words.\n\n")
	fmt.Printf("owner:        %s\n", orUnset(cfg.Owner))
	fmt.Printf("repos:        %s\n", orUnset(strings.Join(cfg.Repos, ", ")))
	fmt.Printf("checkouts:    %s\n", orUnset(strings.Join(cfg.roots(), ", ")))
	set := cfg.checkouts()
	if missing := checkout.Missing(set, cfg.Repos); len(missing) > 0 {
		fmt.Printf("  no checkout: %s\n", strings.Join(missing, ", "))
	}
	for _, r := range cfg.Repos {
		if dir, ok := set.Dir(r); ok && filepath.Dir(dir) != cfg.WorkRoot {
			fmt.Printf("  %-14s %s\n", r, dir)
		}
	}
	fmt.Printf("sources:      %s\n", strings.Join(cfg.Sources, ", "))
	fmt.Printf("programs:     %s\npartnerships: %s\n", cfg.programsDir(), cfg.partnersDir())
	fmt.Printf("assignments:  %s\nreports:      %s\n", cfg.assignmentsRoot(), cfg.reportsDir())
	fmt.Printf("shifts:       %s\n", filepath.Join(cfg.home(), "shifts"))
	return nil
}

// configInit writes the configuration a fresh install does not have,
// establishing what it can and asking for the rest.
func configInit(at string) error {
	path := at
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		path = filepath.Join(home, ".mellions", "config.json")
	}
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("%s already exists; read it with `mellions config show`", path)
	}
	c := Config{
		Sources:          []string{"programs", "assignments", "github", "git", "stale"},
		OwnerLabels:      []string{"needs-owner", "pending-owner-decision"},
		StaleMinAgeHours: 168,
		PerRepoLimit:     50,
		GitSinceHours:    168,
	}
	var found, ask []string
	if login := discoverLogin(); login != "" {
		c.Owner = login
		found = append(found, "GitHub login "+login)
	} else {
		ask = append(ask, "`owner` — the GitHub organisation or user the repositories live under")
	}
	if root, repos := discoverWorkRoot(); root != "" {
		c.WorkRoot, c.Repos = root, repos
		found = append(found, fmt.Sprintf("%d checkout(s) under %s", len(repos), root))
	} else {
		ask = append(ask, "`work_root` and `repos` — where the checkouts are, and which are in scope")
	}
	raw, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := durable.Write(path, append(raw, '\n'), 0o644); err != nil {
		return err
	}
	fmt.Printf("Wrote %s\n\n", path)
	if len(found) > 0 {
		fmt.Printf("Discovered: %s\n\n", strings.Join(found, " · "))
	}
	if len(ask) > 0 {
		fmt.Println("Still yours to fill in:")
		for _, a := range ask {
			fmt.Printf("  %s\n", a)
		}
		fmt.Println()
	}
	fmt.Print("Then: mellions doctor\n")
	return nil
}

func discoverLogin() string {
	out, err := exec.Command("gh", "api", "user", "--jq", ".login").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// discoverWorkRoot looks for the directory that holds several checkouts.
func discoverWorkRoot() (string, []string) {
	wd, err := os.Getwd()
	if err != nil {
		return "", nil
	}
	for dir := wd; ; {
		var repos []string
		if entries, err := os.ReadDir(dir); err == nil {
			for _, e := range entries {
				if !e.IsDir() {
					continue
				}
				if _, err := os.Stat(filepath.Join(dir, e.Name(), ".git")); err == nil {
					repos = append(repos, e.Name())
				}
			}
		}
		if len(repos) > 1 {
			sort.Strings(repos)
			return dir, repos
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", nil
		}
		dir = parent
	}
}

func orUnset(s string) string {
	if s == "" {
		return "(unset)"
	}
	return s
}
