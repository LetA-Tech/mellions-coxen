// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package program

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/LetA-Tech/mellions-coxen/internal/checkout"
)

// Evidence is what discovery established, and nothing more.
//
// It carries no conclusions. Which repositories form a program, what the program
// is for, and whether six quiet months mean abandoned or finished are all
// judgements — this is the material they are made from. The same rule the survey
// follows, for the same reason: software collects, the model decides.
type Evidence struct {
	At       time.Time  `json:"at"`
	WorkRoot string     `json:"work_root"`
	Repos    []RepoFact `json:"repos"`
	// CrossRefs records which repositories name which others, and where. It is
	// the cheapest honest signal of a relationship: a repository that never
	// mentions another is unlikely to depend on it.
	CrossRefs []CrossRef `json:"cross_refs,omitempty"`
	// Failures are what could not be examined. Present so a thin picture is
	// never mistaken for a small estate.
	Failures []string `json:"failures,omitempty"`
}

// RepoFact is what can be established about one repository without judgement.
type RepoFact struct {
	Name         string    `json:"name"`
	Branch       string    `json:"branch"`
	Head         string    `json:"head,omitempty"`
	LastCommitAt time.Time `json:"last_commit_at,omitzero"`
	// CommitsInWindow and Authors say whether anyone is still working here.
	CommitsInWindow int      `json:"commits_in_window"`
	WindowDays      int      `json:"window_days"`
	Authors         []string `json:"authors,omitempty"`
	// Languages is a rough file-count profile, most files first.
	Languages []string `json:"languages,omitempty"`
	// Docs are the governing documents present, which is where a reader should
	// go next rather than having them copied into the program.
	Docs []string `json:"docs,omitempty"`
	// Migrations says whether this repository owns schema, and how recently.
	Migrations      int    `json:"migrations,omitempty"`
	NewestMigration string `json:"newest_migration,omitempty"`
}

// Quiet reports how long since the last commit.
func (r RepoFact) Quiet(now time.Time) time.Duration {
	if r.LastCommitAt.IsZero() {
		return 0
	}
	return now.Sub(r.LastCommitAt)
}

// CrossRef is one repository naming another.
type CrossRef struct {
	From string `json:"from"`
	To   string `json:"to"`
	// Hits is how many files mention it, and Sample is one of them, so a reader
	// can check rather than take the count on trust.
	Hits   int    `json:"hits"`
	Sample string `json:"sample,omitempty"`
	// InCode and InDocs split the mentions, because they mean different things.
	// A name in source or configuration is usually a dependency; a name in an
	// archived design note is usually history. Reporting the split is more
	// evidence; deciding which matters is the model's job.
	InCode int `json:"in_code"`
	InDocs int `json:"in_docs"`
	// CodeSample is a mention from source or configuration, when there is one.
	CodeSample string `json:"code_sample,omitempty"`
}

// DiscoverOptions configures a discovery run.
type DiscoverOptions struct {
	WorkRoot string
	// Checkouts resolves a repository to its directory. Authoritative where
	// set, so an installation spanning more than one root is ordinary.
	Checkouts checkout.Set
	// Repos limits the scan; empty discovers every checkout under WorkRoot,
	// which is the point of discovery.
	Repos []string
	// WindowDays bounds "is anyone still working here".
	WindowDays int
	// Run executes a command in a directory; nil uses the real one.
	Run func(ctx context.Context, dir, name string, args ...string) (string, error)
}

// Discover collects evidence about the engineering environment.
func Discover(ctx context.Context, o DiscoverOptions) (*Evidence, error) {
	if o.WorkRoot == "" && len(o.Checkouts) == 0 {
		return nil, fmt.Errorf("program: no work root to discover from")
	}
	if o.WindowDays <= 0 {
		o.WindowDays = 90
	}
	if o.Run == nil {
		o.Run = runCmd
	}

	names := o.Repos
	set := o.Checkouts
	// Enumeration only where nobody said which. A named list is already the
	// answer, and scanning for it would turn a typo into silence.
	if len(names) == 0 {
		if len(set) == 0 {
			var err error
			if set, err = checkout.Discover(o.WorkRoot); err != nil {
				return nil, err
			}
		}
		names = set.Names()
	}

	ev := &Evidence{At: time.Now().UTC(), WorkRoot: o.WorkRoot}
	for _, name := range names {
		dir, ok := set.Dir(name)
		if !ok {
			dir = filepath.Join(o.WorkRoot, name)
		}
		fact, err := repoFact(ctx, o, dir, name)
		if err != nil {
			ev.Failures = append(ev.Failures, fmt.Sprintf("%s: %v", name, err))
			continue
		}
		ev.Repos = append(ev.Repos, fact)
	}
	if len(ev.Repos) == 0 {
		return nil, fmt.Errorf("program: no repository could be examined under %s", o.WorkRoot)
	}
	ev.CrossRefs = crossRefs(ctx, o, set, names)
	return ev, nil
}

func repoFact(ctx context.Context, o DiscoverOptions, dir, name string) (RepoFact, error) {
	f := RepoFact{Name: name, WindowDays: o.WindowDays}

	branch, err := o.Run(ctx, dir, "git", "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return f, err
	}
	f.Branch = strings.TrimSpace(branch)
	if head, err := o.Run(ctx, dir, "git", "rev-parse", "--short", "HEAD"); err == nil {
		f.Head = strings.TrimSpace(head)
	}
	if last, err := o.Run(ctx, dir, "git", "log", "-1", "--format=%cI"); err == nil {
		f.LastCommitAt, _ = time.Parse(time.RFC3339, strings.TrimSpace(last))
	}
	since := fmt.Sprintf("--since=%d.days.ago", o.WindowDays)
	if n, err := o.Run(ctx, dir, "git", "rev-list", "--count", since, "HEAD"); err == nil {
		f.CommitsInWindow, _ = strconv.Atoi(strings.TrimSpace(n))
	}
	if a, err := o.Run(ctx, dir, "git", "log", since, "--format=%an"); err == nil {
		f.Authors = topN(counts(lines(a)), 5)
	}

	f.Languages = languages(dir)
	f.Docs = docs(dir)
	f.Migrations, f.NewestMigration = migrations(dir)
	return f, nil
}

// crossRefs finds which repositories name which others.
//
// Deliberately a mention count and one example rather than a dependency graph.
// Establishing a real dependency means reading imports, schemas and wire
// contracts per language and per transport; a mention is cheap, honest about
// what it is, and enough for the model to know where to look.
func crossRefs(ctx context.Context, o DiscoverOptions, set checkout.Set, names []string) []CrossRef {
	var out []CrossRef
	for _, from := range names {
		dir, ok := set.Dir(from)
		if !ok {
			dir = filepath.Join(o.WorkRoot, from)
		}
		for _, to := range names {
			if from == to {
				continue
			}
			res, err := o.Run(ctx, dir, "git", "grep", "-l", "-F", "--", to)
			if err != nil {
				continue // no match is an error exit for git grep
			}
			hits := lines(res)
			if len(hits) == 0 {
				continue
			}
			c := CrossRef{From: from, To: to, Hits: len(hits), Sample: hits[0]}
			for _, h := range hits {
				if isCode(h) {
					c.InCode++
					if c.CodeSample == "" {
						c.CodeSample = h
					}
				} else {
					c.InDocs++
				}
			}
			if c.CodeSample != "" {
				c.Sample = c.CodeSample
			}
			out = append(out, c)
		}
	}
	// Code mentions first within a repository: they are the ones most likely to
	// be a real edge, and a reader scanning the top of a long list should meet
	// those before archived design notes.
	sort.Slice(out, func(i, j int) bool {
		if out[i].From != out[j].From {
			return out[i].From < out[j].From
		}
		if out[i].InCode != out[j].InCode {
			return out[i].InCode > out[j].InCode
		}
		return out[i].Hits > out[j].Hits
	})
	return out
}

// isCode reports whether a path is source or configuration rather than prose.
//
// Used only to split a count and choose a sample, never to drop a mention: a
// name in a document is still evidence, and discarding it because a heuristic
// called it prose would be exactly the silent judgement this package avoids.
func isCode(path string) bool {
	lower := strings.ToLower(path)
	for _, marker := range []string{"/_archived/", "/archive/", "/docs/", "/adr/"} {
		if strings.Contains(lower, marker) {
			return false
		}
	}
	switch strings.ToLower(filepath.Ext(path)) {
	case ".md", ".txt", ".rst", ".adoc":
		return false
	}
	return true
}

// Text renders evidence as the material a session drafts a program from.
func (e *Evidence) Text() string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Discovery evidence — %s\n\n", e.At.Format(time.RFC3339))
	fmt.Fprintf(&b, "%d repositories under %s.\n", len(e.Repos), e.WorkRoot)
	b.WriteString("\nThese are facts, not conclusions. Which of these form one program, what that " +
		"program is for, and what a quiet repository means are judgements — yours, not this " +
		"file's.\n")

	if len(e.Failures) > 0 {
		b.WriteString("\n## Could not examine\n\n")
		b.WriteString("Treat these as unknown, not as absent:\n\n")
		for _, f := range e.Failures {
			fmt.Fprintf(&b, "- %s\n", f)
		}
	}

	b.WriteString("\n## Repositories\n")
	now := time.Now()
	for _, r := range e.Repos {
		fmt.Fprintf(&b, "\n### %s\n\n", r.Name)
		fmt.Fprintf(&b, "- branch `%s`", r.Branch)
		if r.Head != "" {
			fmt.Fprintf(&b, " at `%s`", r.Head)
		}
		b.WriteString("\n")
		if !r.LastCommitAt.IsZero() {
			fmt.Fprintf(&b, "- last commit %s (%s ago)\n",
				r.LastCommitAt.Format("2006-01-02"), human(r.Quiet(now)))
		}
		fmt.Fprintf(&b, "- %d commits in %d days", r.CommitsInWindow, r.WindowDays)
		if len(r.Authors) > 0 {
			fmt.Fprintf(&b, ", by %s", strings.Join(r.Authors, ", "))
		}
		b.WriteString("\n")
		if len(r.Languages) > 0 {
			fmt.Fprintf(&b, "- %s\n", strings.Join(r.Languages, ", "))
		}
		if len(r.Docs) > 0 {
			fmt.Fprintf(&b, "- governing docs: %s — read these rather than copying them\n",
				strings.Join(r.Docs, ", "))
		}
		if r.Migrations > 0 {
			fmt.Fprintf(&b, "- owns schema: %d migrations, newest `%s`\n", r.Migrations, r.NewestMigration)
		}
	}

	if len(e.CrossRefs) > 0 {
		b.WriteString("\n## Which repositories name which\n\n")
		b.WriteString("A mention, not an established dependency — but a repository that never " +
			"mentions another is unlikely to depend on it.\n\n")
		for _, c := range e.CrossRefs {
			where := fmt.Sprintf("%d in code", c.InCode)
			if c.InDocs > 0 {
				where += fmt.Sprintf(", %d in prose", c.InDocs)
			}
			if c.InCode == 0 {
				where += " — prose only, so probably history rather than a dependency"
			}
			fmt.Fprintf(&b, "- %s → %s (%s; e.g. `%s`)\n", c.From, c.To, where, c.Sample)
		}
	}
	return b.String()
}

// ---- helpers ----------------------------------------------------------------

var docNames = []string{
	"CLAUDE.md", "AGENTS.md", "README.md", "ARCHITECTURE.md",
	"docs/architecture.md", "repo-binding.yaml", "CONTRIBUTING.md",
}

func docs(dir string) []string {
	var out []string
	for _, n := range docNames {
		if _, err := os.Stat(filepath.Join(dir, n)); err == nil {
			out = append(out, n)
		}
	}
	return out
}

var langByExt = map[string]string{
	".go": "Go", ".py": "Python", ".ts": "TypeScript", ".tsx": "TypeScript",
	".js": "JavaScript", ".sql": "SQL", ".rs": "Rust", ".java": "Java",
	".rb": "Ruby", ".sh": "shell", ".proto": "protobuf",
}

// languages profiles a checkout by file count, skipping the places that would
// otherwise report every project as mostly vendored dependencies.
func languages(dir string) []string {
	count := map[string]int{}
	_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "node_modules", "vendor", "dist", "build", ".venv", "target":
				return filepath.SkipDir
			}
			return nil
		}
		if l, ok := langByExt[strings.ToLower(filepath.Ext(path))]; ok {
			count[l]++
		}
		return nil
	})
	return topN(count, 3)
}

func migrations(dir string) (int, string) {
	var found []string
	for _, candidate := range []string{"migrations", "db/migrations", "migrations/postgres"} {
		_ = filepath.WalkDir(filepath.Join(dir, candidate), func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			if strings.HasSuffix(path, ".sql") {
				found = append(found, filepath.Base(path))
			}
			return nil
		})
		if len(found) > 0 {
			break
		}
	}
	if len(found) == 0 {
		return 0, ""
	}
	sort.Strings(found)
	return len(found), found[len(found)-1]
}

func runCmd(ctx context.Context, dir, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	return string(out), err
}

func lines(s string) []string {
	var out []string
	for l := range strings.SplitSeq(s, "\n") {
		if t := strings.TrimSpace(l); t != "" {
			out = append(out, t)
		}
	}
	return out
}

func counts(in []string) map[string]int {
	m := map[string]int{}
	for _, s := range in {
		m[s]++
	}
	return m
}

func topN(m map[string]int, n int) []string {
	type kv struct {
		k string
		v int
	}
	var all []kv
	for k, v := range m {
		all = append(all, kv{k, v})
	}
	sort.Slice(all, func(i, j int) bool {
		if all[i].v != all[j].v {
			return all[i].v > all[j].v
		}
		return all[i].k < all[j].k
	})
	if len(all) > n {
		all = all[:n]
	}
	out := make([]string, 0, len(all))
	for _, e := range all {
		out = append(out, e.k)
	}
	return out
}

func human(d time.Duration) string {
	switch {
	case d < 48*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	case d < 60*24*time.Hour:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	default:
		return fmt.Sprintf("%dmo", int(d.Hours()/24/30))
	}
}
