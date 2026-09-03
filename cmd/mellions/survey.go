// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/LetA-Tech/mellions-coxen/internal/assignment"
	"github.com/LetA-Tech/mellions-coxen/internal/durable"
	sig "github.com/LetA-Tech/mellions-coxen/internal/signal"
	"github.com/LetA-Tech/mellions-coxen/internal/sources/assignmentsrc"
	"github.com/LetA-Tech/mellions-coxen/internal/sources/githubsrc"
	"github.com/LetA-Tech/mellions-coxen/internal/sources/gitsrc"
	"github.com/LetA-Tech/mellions-coxen/internal/sources/programs"
	"github.com/LetA-Tech/mellions-coxen/internal/sources/stale"
	"github.com/LetA-Tech/mellions-coxen/internal/survey"
)

// build wires the configured sources into a registry. This is the only place a
// provider package is named; everything above it works through signal.Source.
func (c *Config) build() (*sig.Registry, error) {
	reg := sig.NewRegistry()
	want := map[string]bool{}
	for _, n := range c.Sources {
		want[n] = true
	}
	if want["programs"] {
		if err := reg.Register(programs.New(programs.Options{Dir: c.programsDir()})); err != nil {
			return nil, err
		}
	}
	if want["github"] {
		if c.Owner == "" {
			return nil, errors.New("source github needs \"owner\" in config")
		}
		if err := reg.Register(githubsrc.New(githubsrc.Options{
			Owner: c.Owner, Repos: c.Repos,
			OwnerLabels: c.OwnerLabels, PerRepoLimit: c.PerRepoLimit,
		})); err != nil {
			return nil, err
		}
	}
	if want["assignments"] {
		st, err := assignment.NewStore(c.assignmentsRoot())
		if err != nil {
			return nil, err
		}
		if err := reg.Register(assignmentsrc.New(st)); err != nil {
			return nil, err
		}
	}
	if want["git"] {
		if len(c.roots()) == 0 && len(c.CheckoutAt) == 0 {
			return nil, errors.New("source git needs \"work_root\" or \"work_roots\" in config")
		}
		if err := reg.Register(gitsrc.New(gitsrc.Options{
			WorkRoot: c.WorkRoot, Repos: c.Repos, Checkouts: c.checkouts(),
			Since: time.Duration(c.GitSinceHours) * time.Hour,
		})); err != nil {
			return nil, err
		}
	}
	if want["stale"] {
		if len(c.roots()) == 0 && len(c.CheckoutAt) == 0 {
			return nil, errors.New("source stale needs \"work_root\" or \"work_roots\" in config: a claim cannot be checked without the code")
		}
		checkouts := map[string]string(c.checkouts())
		if len(checkouts) == 0 {
			var err error
			if checkouts, err = stale.DiscoverCheckouts(c.WorkRoot); err != nil {
				return nil, err
			}
		}
		if err := reg.Register(stale.New(stale.Options{
			Owner: c.Owner, Repos: c.Repos, Checkouts: checkouts,
			MinAge: time.Duration(c.StaleMinAgeHours) * time.Hour,
		})); err != nil {
			return nil, err
		}
	}
	for _, n := range c.Sources {
		if _, ok := reg.Get(n); !ok {
			return nil, fmt.Errorf("source %q is configured but not built into this binary; known: %s",
				n, strings.Join(reg.Names(), ", "))
		}
	}
	return reg, nil
}

func cmdSurvey(ctx context.Context, args []string) error {
	fs := newFlagSet("survey", flag.ExitOnError)
	cfgPath := fs.String("config", "", "config file")
	repos := fs.String("repos", "", "comma-separated repositories, overriding config")
	sources := fs.String("sources", "", "comma-separated sources, overriding config")
	asJSON := fs.Bool("json", false, "emit the raw result as JSON instead of prose")
	full := fs.Bool("full", false, "print every field of every signal collected, uncapped")
	kinds := fs.String("kind", "", "comma-separated kinds to print; the summary still counts every kind")
	since := fs.Duration("since", 0, "how far back time-ordered sources reach")
	limit := fs.Int("limit", 0, "cap items per source")
	timeout := fs.Duration("timeout", 90*time.Second, "bound on one source")
	save := fs.Bool("save", false, "also write the survey where the session-start hook looks for it")
	if err := fs.Parse(args); err != nil {
		return err
	}

	// Before anything is collected: a mistyped kind must not cost a full sweep
	// of the estate before it is refused.
	wantKinds, err := parseKinds(*kinds)
	if err != nil {
		return err
	}

	cfg, err := loadConfig(*cfgPath)
	if err != nil {
		return err
	}
	if *sources != "" {
		cfg.Sources = splitList(*sources)
	}
	reg, err := cfg.build()
	if err != nil {
		return err
	}
	runner, err := survey.NewRunner(reg, cfg.Sources)
	if err != nil {
		return err
	}
	runner.Timeout = *timeout

	scope := sig.Scope{Repos: cfg.Repos, Limit: *limit}
	if *repos != "" {
		scope.Repos = splitList(*repos)
	}
	if *since > 0 {
		scope.Since = time.Now().Add(-*since)
	}

	res := runner.Run(ctx, scope)

	opts := survey.Default()
	if *full {
		opts = survey.Everything()
	}
	opts.Kinds = wantKinds
	opts.CollectionLimit = collectionLimit(cfg, *limit)

	if *save {
		// Always the default form, whatever this invocation asked to print: the
		// hook points a session with nothing in flight at this file, and that
		// session needs the whole estate rather than one caller's slice of it.
		saved := survey.Default()
		saved.CollectionLimit = opts.CollectionLimit
		if err := saveSurvey(cfg, res.Render(saved)); err != nil {
			fmt.Fprintf(os.Stderr, "mellions: survey not saved: %v\n", err)
		}
	}
	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(jsonResult{r: res}); err != nil {
			return err
		}
	} else {
		fmt.Print(res.Render(opts))
	}

	// A partial survey exits non-zero so a scheduled run cannot report success
	// on an incomplete picture. The evidence is still printed.
	if !res.Complete() {
		return fmt.Errorf("%d of %d sources did not answer; the picture above is incomplete",
			len(res.Failures), len(res.Ran))
	}
	return nil
}

// surveyPath is where the latest saved survey lives, for the hooks to hand to a
// session that has nothing in flight.
func (c *Config) surveyPath() string { return filepath.Join(c.reportRoot(), "survey-latest.md") }

func saveSurvey(cfg *Config, rendered string) error {
	if err := os.MkdirAll(cfg.reportRoot(), 0o755); err != nil {
		return err
	}
	return durable.Write(cfg.surveyPath(), []byte(rendered), 0o644)
}

// parseKinds turns the -kind flag into kinds to print.
//
// An unrecognised name is an error, not a silent no-match, for the same reason
// an unknown source is: a mistyped filter that renders an empty survey reads
// exactly like an estate with nothing in it.
func parseKinds(list string) ([]sig.Kind, error) {
	var out []sig.Kind
	for _, n := range splitList(list) {
		k := sig.Kind(n)
		if !slices.Contains(sig.Kinds, k) {
			names := make([]string, 0, len(sig.Kinds))
			for _, known := range sig.Kinds {
				names = append(names, string(known))
			}
			return nil, fmt.Errorf("unknown kind %q; known: %s", n, strings.Join(names, ", "))
		}
		out = append(out, k)
	}
	return out, nil
}

// collectionLimit is the per-repository cap the sources actually ran under, so
// the render can say when a repository's list is bounded by the cap rather than
// by reality.
func collectionLimit(cfg *Config, flagLimit int) int {
	if flagLimit > 0 {
		return flagLimit
	}
	if cfg.PerRepoLimit > 0 {
		return cfg.PerRepoLimit
	}
	return githubsrc.DefaultPerRepoLimit
}

// surveyBrief summarises a saved survey in one line, and reports how old it is.
func surveyBrief(path string) (brief string, age time.Duration, ok bool) {
	info, err := os.Stat(path)
	if err != nil {
		return "", 0, false
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", 0, false
	}
	// Read the count the heading states rather than counting bullets under it.
	// A rendered section holds lines that are not signals — what a cap held
	// back, and the command that prints it — and the heading is the collected
	// count whatever the render did below it.
	counts := map[string]int{}
	incomplete := false
	for line := range strings.SplitSeq(string(raw), "\n") {
		if !strings.HasPrefix(line, "## ") {
			continue
		}
		head := strings.TrimSpace(strings.TrimPrefix(line, "## "))
		if strings.HasPrefix(head, "INCOMPLETE") {
			incomplete = true
			continue
		}
		open := strings.LastIndex(head, " (")
		if open < 0 || !strings.HasSuffix(head, ")") {
			continue
		}
		n, err := strconv.Atoi(head[open+2 : len(head)-1])
		if err != nil {
			continue
		}
		counts[head[:open]] = n
	}
	var parts []string
	for _, k := range []string{"Stale premises — recorded claims the current tree contradicts", "Failing checks", "Waiting on the owner", "Changes under review", "Tracked work items"} {
		if n := counts[k]; n > 0 {
			short := k
			if i := strings.Index(short, " —"); i > 0 {
				short = short[:i]
			}
			parts = append(parts, fmt.Sprintf("%d %s", n, strings.ToLower(short)))
		}
	}
	if incomplete {
		parts = append(parts, "INCOMPLETE")
	}
	brief = "Survey"
	if len(parts) > 0 {
		brief += ": " + strings.Join(parts, ", ")
	}
	return brief + ".", time.Since(info.ModTime()), true
}

// jsonResult is the machine-readable shape. Errors become strings because a
// consumer needs to read them, not unwrap them.
type jsonResult struct{ r survey.Result }

func (j jsonResult) MarshalJSON() ([]byte, error) {
	type failure struct {
		Source string `json:"source"`
		Error  string `json:"error"`
	}
	out := struct {
		At       time.Time    `json:"at"`
		Complete bool         `json:"complete"`
		Ran      []string     `json:"ran"`
		Elapsed  string       `json:"elapsed"`
		Failures []failure    `json:"failures,omitempty"`
		Signals  []sig.Signal `json:"signals"`
		Note     string       `json:"note"`
	}{
		At: j.r.At, Complete: j.r.Complete(), Ran: j.r.Ran,
		Elapsed: j.r.Elapsed.Round(time.Millisecond).String(),
		Signals: j.r.Signals,
		Note:    "collected evidence, in source order. nothing here is ranked, scored or recommended.",
	}
	for _, f := range j.r.Failures {
		out.Failures = append(out.Failures, failure{Source: f.Source, Error: f.Err.Error()})
	}
	return json.Marshal(out)
}

func cmdSources(args []string) error {
	fs := newFlagSet("sources", flag.ExitOnError)
	cfgPath := fs.String("config", "", "config file")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg, err := loadConfig(*cfgPath)
	if err != nil {
		return err
	}
	fmt.Printf("config: %s\n", cfg.path)
	fmt.Printf("owner:  %s\n", cfg.Owner)
	fmt.Printf("repos:  %s\n", strings.Join(cfg.Repos, ", "))
	reg, err := cfg.build()
	if err != nil {
		return err
	}
	fmt.Printf("\nbuilt sources: %s\n", strings.Join(reg.Names(), ", "))
	fmt.Printf("configured order: %s\n", strings.Join(cfg.Sources, ", "))
	return nil
}
