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
	"time"

	"github.com/LetA-Tech/mellions-coxen/internal/awareness"
	"github.com/LetA-Tech/mellions-coxen/internal/program"
	"github.com/LetA-Tech/mellions-coxen/internal/provenance"
)

func cmdProgram(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("program needs a verb: discover, show, list, check, adopt")
	}
	switch args[0] {
	case "discover":
		return programDiscover(ctx, args[1:])
	case "show":
		return programShow(args[1:])
	case "list":
		return programList(args[1:])
	case "check":
		return programCheck(args[1:])
	case "adopt":
		return docAdopt(provenance.KindProgram, args[1:])
	default:
		return fmt.Errorf("program: unknown verb %q", args[0])
	}
}

func (c *Config) programPath(slug string) string {
	return filepath.Join(c.programsDir(), slug+".md")
}

// programDiscover collects evidence and writes no program: what is true is
// established here, and what any of it means is the session's to decide.
func programDiscover(ctx context.Context, args []string) error {
	fs := newFlagSet("program discover", flag.ExitOnError)
	cfgPath := fs.String("config", "", "config file")
	window := fs.Int("window-days", 90, "how far back to judge activity")
	save := fs.String("out", "", "also write the evidence here")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg, err := loadConfig(*cfgPath)
	if err != nil {
		return err
	}
	ev, err := program.Discover(ctx, program.DiscoverOptions{
		WorkRoot: cfg.WorkRoot, Checkouts: cfg.checkouts(), WindowDays: *window,
	})
	if err != nil {
		return err
	}
	text := ev.Text()
	if *save != "" {
		if err := os.WriteFile(*save, []byte(text), 0o644); err != nil {
			return err
		}
	}
	fmt.Print(text)
	fmt.Print(`
---

Now write the program. One file per program at ` + cfg.programsDir() + `/<slug>.md,
starting with a "# Program: <name>" title and a "discovered: <RFC3339>" line.

Mark every section heading with its provenance, and hold the line between them:

  {DISCOVERED}  established from the evidence above, with the citation
  {INFERRED}    your reading — supported by evidence, not established by it
  {UNKNOWN}     what this could not settle, and what would settle it
  {DECLARED}    the owner's intent. Leave these as questions for them.

You may not write {DECLARED} content. Purpose, correctness, standing priorities
and what is deliberately deferred are the owner's; a repository quiet for six
months is a fact, and whether that means abandoned, finished or frozen is not
something evidence can reach. Write the headings and say what you need to know.

A good first draft has full DISCOVERED sections, a substantial UNKNOWN section,
and DECLARED sections that are questions. Then: mellions program check <slug>
`)
	return nil
}

func programShow(args []string) error {
	fs := newFlagSet("program show", flag.ExitOnError)
	cfgPath := fs.String("config", "", "config file")
	brief := fs.Bool("brief", false, "the declared sections in full, the rest as first lines, bounded")
	here := fs.Bool("here", false,
		"render only where this repository is inside the program's declared boundary")
	rest, err := parsePositional(fs, args)
	if err != nil {
		return err
	}
	cfg, err := loadConfig(*cfgPath)
	if err != nil {
		return err
	}
	slug, err := onlyOrNamedIn(cfg.programsDir(), arg(rest, 0), "mellions program discover")
	if err != nil {
		return err
	}
	p, err := program.Load(cfg.programPath(slug))
	if err != nil {
		return err
	}
	// A program reaches a session only when its declared repository boundary
	// covers that session; an unbounded program is acknowledged without being
	// injected as irrelevant standing context.
	if *here {
		repo := repoOf(".")
		if !p.Covers(repo) {
			fmt.Printf("A program is adopted (`%s`) and its declared boundary does not name %s.\n"+
				"`mellions program show %s` reads it, and `repos:` in its header is what\n"+
				"declares which repositories it is about.\n",
				slug, repoHereName(repo), slug)
			return nil
		}
	}
	noteDelivery(cfg, awareness.KindProgram, p)
	if *brief {
		fmt.Print(p.Brief(time.Now(), cfg.programPath(slug), 8500))
	} else {
		fmt.Print(p.Text(time.Now()))
	}
	return nil
}

func programList(args []string) error {
	fs := newFlagSet("program list", flag.ExitOnError)
	cfgPath := fs.String("config", "", "config file")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg, err := loadConfig(*cfgPath)
	if err != nil {
		return err
	}
	return docList(cfg.programsDir(), "program", "mellions program discover", program.Load)
}

func programCheck(args []string) error {
	fs := newFlagSet("program check", flag.ExitOnError)
	cfgPath := fs.String("config", "", "config file")
	staleDays := fs.Int("stale-days", 45, "evidence older than this wants re-discovery")
	rest, err := parsePositional(fs, args)
	if err != nil {
		return err
	}
	cfg, err := loadConfig(*cfgPath)
	if err != nil {
		return err
	}
	slug, err := onlyOrNamedIn(cfg.programsDir(), arg(rest, 0), "mellions program discover")
	if err != nil {
		return err
	}
	p, err := program.Load(cfg.programPath(slug))
	if err != nil {
		return err
	}
	return docCheck(slug, p, *staleDays)
}

// repoHereName names the repository a rendering was skipped for, or says there
// is none rather than printing an empty gap.
func repoHereName(repo string) string {
	if repo == "" {
		return "this directory, which is not a repository"
	}
	return repo
}
