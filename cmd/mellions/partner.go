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
	"github.com/LetA-Tech/mellions-coxen/internal/partner"
	"github.com/LetA-Tech/mellions-coxen/internal/provenance"
)

func cmdPartner(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("partner needs a verb: establish, show, list, check, adopt")
	}
	switch args[0] {
	case "establish":
		return partnerEstablish(ctx, args[1:])
	case "show":
		return partnerShow(args[1:])
	case "list":
		return partnerList(args[1:])
	case "check":
		return partnerCheck(args[1:])
	case "adopt":
		return docAdopt(provenance.KindPartnership, args[1:])
	default:
		return fmt.Errorf("partner: unknown verb %q", args[0])
	}
}

func (c *Config) partnerPath(slug string) string {
	return filepath.Join(c.partnersDir(), slug+".md")
}

// partnerEstablish collects evidence about the people in an estate and writes
// no partnership: a commit histogram can establish when somebody works and
// never what they want from a colleague.
func partnerEstablish(ctx context.Context, args []string) error {
	fs := newFlagSet("partner establish", flag.ExitOnError)
	cfgPath := fs.String("config", "", "config file")
	person := fs.String("person", "", "narrow to one name or email; empty reports everyone")
	window := fs.Int("window-days", 365, "how far back to look")
	save := fs.String("out", "", "also write the evidence here")
	rest, err := parsePositional(fs, args)
	if err != nil {
		return err
	}
	cfg, err := loadConfig(*cfgPath)
	if err != nil {
		return err
	}
	if *person == "" {
		*person = arg(rest, 0)
	}
	ev, err := partner.Discover(ctx, partner.DiscoverOptions{
		WorkRoot: cfg.WorkRoot, Checkouts: cfg.checkouts(), Person: *person, WindowDays: *window,
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

Now write the partnership. One file per person at ` + cfg.partnersDir() + `/<name>.md,
starting with "# Partnership: <name>" and a "discovered: <RFC3339>" line.

This document says how you and one person work together. It is not your
identity — nothing in it changes what you are or what you hold yourself to —
and it is not the program, which says what work you are carrying.

Mark every section with its provenance:

  {DISCOVERED}  established from the evidence above, with the citation
  {INFERRED}    your reading — supported by evidence, not established by it
  {UNKNOWN}     what this could not settle, and what would settle it
  {DECLARED}    theirs. Leave these as questions addressed to them.

You may not write {DECLARED} content. What kind of peer they want, what they
want done without being asked and what they want to see first, what is
delegated to you and what stays theirs, when a question is welcome and when it
interrupts, what they want to hear about at once and what can wait for morning
— all theirs. Inferring any of it from when somebody commits is the mistake
this split exists to prevent.

Then: mellions partner check <name>
`)
	return nil
}

func partnerShow(args []string) error {
	fs := newFlagSet("partner show", flag.ExitOnError)
	cfgPath := fs.String("config", "", "config file")
	brief := fs.Bool("brief", false, "the declared sections in full, the rest as first lines, bounded")
	here := fs.Bool("here", false,
		"leave out sections whose own `repos:` boundary excludes this repository")
	rest, err := parsePositional(fs, args)
	if err != nil {
		return err
	}
	cfg, err := loadConfig(*cfgPath)
	if err != nil {
		return err
	}
	slug, err := onlyOrNamedIn(cfg.partnersDir(), arg(rest, 0), "mellions partner establish")
	if err != nil {
		return err
	}
	p, err := partner.Load(cfg.partnerPath(slug))
	if err != nil {
		return err
	}
	noteDelivery(cfg, awareness.KindPartnership, p)
	// A partnership reaches every session and most of it belongs there. Only
	// the sections that declared their own `repos:` boundary narrow — what the
	// owner delegated is never withheld because the repository is different.
	if *here {
		p = p.Here(repoOf("."))
	}
	if *brief {
		fmt.Print(p.Brief(time.Now(), cfg.partnerPath(slug), 8500))
	} else {
		fmt.Print(partner.Text(p, time.Now()))
	}
	return nil
}

func partnerList(args []string) error {
	fs := newFlagSet("partner list", flag.ExitOnError)
	cfgPath := fs.String("config", "", "config file")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg, err := loadConfig(*cfgPath)
	if err != nil {
		return err
	}
	return docList(cfg.partnersDir(), "partnership", "mellions partner establish", partner.Load)
}

func partnerCheck(args []string) error {
	fs := newFlagSet("partner check", flag.ExitOnError)
	cfgPath := fs.String("config", "", "config file")
	staleDays := fs.Int("stale-days", 180, "a partnership unreviewed for this long wants revisiting")
	rest, err := parsePositional(fs, args)
	if err != nil {
		return err
	}
	cfg, err := loadConfig(*cfgPath)
	if err != nil {
		return err
	}
	slug, err := onlyOrNamedIn(cfg.partnersDir(), arg(rest, 0), "mellions partner establish")
	if err != nil {
		return err
	}
	p, err := partner.Load(cfg.partnerPath(slug))
	if err != nil {
		return err
	}
	return docCheck(slug, p, *staleDays)
}
