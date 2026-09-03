// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package partner

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/LetA-Tech/mellions-coxen/internal/checkout"
)

// Evidence is what discovery established about the people working in an estate,
// and nothing more.
//
// Almost nothing that matters about a relationship is in here, which is the
// point of collecting it separately. Git can establish that someone commits at
// midnight, lands other people's work, and has not touched a repository in four
// months. It cannot establish whether they want to be challenged, what they
// would rather hear about in the morning than at 2am, or what kind of peer they
// are looking for. Those are declared, never inferred.
type Evidence struct {
	At         time.Time    `json:"at"`
	WorkRoot   string       `json:"work_root"`
	WindowDays int          `json:"window_days"`
	People     []PersonFact `json:"people"`
	// Overlaps are addresses that commit under more than one name. Reported and
	// never resolved: see Overlap.
	Overlaps []Overlap `json:"overlaps,omitempty"`
	// Failures are what could not be examined, so a thin picture is never
	// mistaken for a quiet estate.
	Failures []string `json:"failures,omitempty"`
}

// Overlap is one email address committing under several names.
//
// Never merged automatically, because the two explanations are indistinguishable
// from here and lead opposite ways: one person whose git config drifted between
// machines, or several people sharing a credential. Merging the first is
// correct and merging the second invents a person who does not exist. In a real
// estate both occur at once — an address can be one human's two spellings while
// another address is a laptop three people have used.
//
// So it is reported as a question. Whoever reads it knows which it is, and
// .mailmap is where the answer belongs once they do.
type Overlap struct {
	Email string   `json:"email"`
	Names []string `json:"names"`
}

// PersonFact is what git says about one identity.
type PersonFact struct {
	Name    string   `json:"name"`
	Emails  []string `json:"emails,omitempty"`
	Commits int      `json:"commits"`
	// Repos is where they commit, most-committed first.
	Repos   []string  `json:"repos,omitempty"`
	FirstAt time.Time `json:"first_at,omitzero"`
	LastAt  time.Time `json:"last_at,omitzero"`
	// HoursLocal counts commits by hour of *their* clock, taken from the offset
	// in each commit's own timestamp. It is the cheapest honest read on when a
	// person is working, and it says nothing about when they want to be
	// disturbed.
	HoursLocal [24]int `json:"hours_local"`
	// Offsets are the UTC offsets seen, which is roughly where they were.
	Offsets []string `json:"offsets,omitempty"`
	// Landed counts commits they committed but did not author — someone
	// merging other people's work, which is a role rather than a volume.
	Landed int `json:"landed"`
}

// DiscoverOptions configures a discovery run.
type DiscoverOptions struct {
	WorkRoot string
	// Checkouts resolves a repository to its directory. Authoritative where
	// set, so an installation spanning more than one root is ordinary.
	Checkouts checkout.Set
	// Repos limits the scan; empty examines every checkout under WorkRoot.
	Repos []string
	// Person narrows to identities whose name or email contains this, case
	// insensitively. Empty reports everyone, which is how you find out who is
	// actually here.
	Person string
	// WindowDays bounds how far back to look.
	WindowDays int
	// Run executes a command in a directory; nil uses the real one.
	Run func(ctx context.Context, dir, name string, args ...string) (string, error)
}

// Discover collects evidence about the people committing in an estate.
func Discover(ctx context.Context, o DiscoverOptions) (*Evidence, error) {
	if o.WorkRoot == "" {
		return nil, fmt.Errorf("partner: no work root to discover from")
	}
	if o.WindowDays <= 0 {
		o.WindowDays = 365
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

	ev := &Evidence{At: time.Now().UTC(), WorkRoot: o.WorkRoot, WindowDays: o.WindowDays}
	agg := map[string]*PersonFact{}
	emails := map[string]map[string]bool{}
	offsets := map[string]map[string]bool{}
	repoHits := map[string]map[string]int{}

	since := fmt.Sprintf("--since=%d.days.ago", o.WindowDays)
	for _, repo := range names {
		dir, ok := set.Dir(repo)
		if !ok {
			dir = filepath.Join(o.WorkRoot, repo)
		}
		// --use-mailmap honours what the repository itself declared about who is
		// who. It is the only identity resolution here that is not a guess: a
		// .mailmap is a person's own statement, and applying it is reading that
		// statement rather than inferring from a name.
		out, err := o.Run(ctx, dir, "git", "log", since, "--use-mailmap",
			"--format=%aN%x09%aE%x09%aI%x09%cN")
		if err != nil {
			ev.Failures = append(ev.Failures, fmt.Sprintf("%s: %v", repo, err))
			continue
		}
		for _, line := range lines(out) {
			f := strings.Split(line, "\t")
			if len(f) < 4 {
				continue
			}
			name, email, stamp, committer := f[0], f[1], f[2], f[3]
			p := agg[name]
			if p == nil {
				p = &PersonFact{Name: name}
				agg[name] = p
				emails[name] = map[string]bool{}
				offsets[name] = map[string]bool{}
				repoHits[name] = map[string]int{}
			}
			p.Commits++
			emails[name][email] = true
			repoHits[name][repo]++

			if at, err := time.Parse(time.RFC3339, stamp); err == nil {
				if p.FirstAt.IsZero() || at.Before(p.FirstAt) {
					p.FirstAt = at
				}
				if at.After(p.LastAt) {
					p.LastAt = at
				}
			}
			// Their local hour and offset come from the timestamp's own offset,
			// not from converting to UTC: the question is what time it was
			// where they were sitting.
			if len(stamp) >= 13 {
				var h int
				if _, err := fmt.Sscanf(stamp[11:13], "%d", &h); err == nil && h >= 0 && h < 24 {
					p.HoursLocal[h]++
				}
			}
			if off := offsetOf(stamp); off != "" {
				offsets[name][off] = true
			}
			if committer != name {
				// Counted against the committer, not the author: this is a fact
				// about who lands work.
				c := agg[committer]
				if c == nil {
					c = &PersonFact{Name: committer}
					agg[committer] = c
					emails[committer] = map[string]bool{}
					offsets[committer] = map[string]bool{}
					repoHits[committer] = map[string]int{}
				}
				c.Landed++
			}
		}
	}

	for name, p := range agg {
		if !matches(o.Person, name, emails[name]) {
			continue
		}
		p.Emails = sortedKeys(emails[name])
		p.Offsets = sortedKeys(offsets[name])
		p.Repos = byCount(repoHits[name])
		ev.People = append(ev.People, *p)
	}
	ev.Overlaps = overlaps(emails)
	sort.Slice(ev.People, func(i, j int) bool {
		if ev.People[i].Commits != ev.People[j].Commits {
			return ev.People[i].Commits > ev.People[j].Commits
		}
		return ev.People[i].Name < ev.People[j].Name
	})

	if len(ev.People) == 0 && len(ev.Failures) == len(names) {
		return nil, fmt.Errorf("partner: no repository could be examined under %s", o.WorkRoot)
	}
	return ev, nil
}

// Text renders evidence as the material a session drafts a partnership from.
func (e *Evidence) Text() string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Partnership evidence — %s\n\n", e.At.Format(time.RFC3339))
	fmt.Fprintf(&b, "%d identities committing under %s in the last %d days.\n",
		len(e.People), e.WorkRoot, e.WindowDays)
	b.WriteString("\nThis is where a person works and when. It is not who they are, and it is " +
		"emphatically not how they want to be worked with. Everything that makes a " +
		"partnership — what kind of peer they expect, how much initiative they want taken " +
		"without being asked, when a question is welcome and when it is an interruption — has " +
		"to be declared by them. Do not infer any of it from a commit histogram.\n")

	if len(e.Failures) > 0 {
		b.WriteString("\n## Could not examine\n\nTreat these as unknown, not as empty:\n\n")
		for _, f := range e.Failures {
			fmt.Fprintf(&b, "- %s\n", f)
		}
	}

	if len(e.Overlaps) > 0 {
		b.WriteString("\n## Identities that share an address\n\n")
		b.WriteString("One address, several names. This is not resolved here and must not be " +
			"resolved by guessing: it is either one person whose git config drifted between " +
			"machines, or several people sharing a credential, and those lead opposite ways. " +
			"Ask, then put the answer in the repository's `.mailmap` where every tool will " +
			"honour it.\n\n")
		for _, o := range e.Overlaps {
			fmt.Fprintf(&b, "- `%s` commits as %s\n", o.Email, strings.Join(o.Names, ", "))
		}
	}

	now := time.Now()
	b.WriteString("\n## People\n")
	for _, p := range e.People {
		fmt.Fprintf(&b, "\n### %s\n\n", p.Name)
		if len(p.Emails) > 0 {
			fmt.Fprintf(&b, "- %s\n", strings.Join(p.Emails, ", "))
		}
		fmt.Fprintf(&b, "- %d commits", p.Commits)
		if p.Landed > 0 {
			fmt.Fprintf(&b, ", landed %d authored by someone else", p.Landed)
		}
		b.WriteString("\n")
		if !p.LastAt.IsZero() {
			fmt.Fprintf(&b, "- last commit %s (%s ago), first in window %s\n",
				p.LastAt.Format("2006-01-02"), human(now.Sub(p.LastAt)), p.FirstAt.Format("2006-01-02"))
		}
		if len(p.Repos) > 0 {
			fmt.Fprintf(&b, "- works in: %s\n", strings.Join(p.Repos, ", "))
		}
		if len(p.Offsets) > 0 {
			fmt.Fprintf(&b, "- commits carry offset %s\n", strings.Join(p.Offsets, ", "))
		}
		if h := histogram(p.HoursLocal); h != "" {
			fmt.Fprintf(&b, "- by hour on their own clock: %s\n", h)
		}
	}
	return b.String()
}

// histogram renders the hours that actually have commits, in order.
//
// Only the non-empty hours: twenty-four buckets of which nineteen are zero
// reads as a table to skip, and the shape is the whole point.
func histogram(h [24]int) string {
	var parts []string
	for hour, n := range h {
		if n > 0 {
			parts = append(parts, fmt.Sprintf("%02d:00×%d", hour, n))
		}
	}
	return strings.Join(parts, " ")
}

// overlaps finds addresses committing under more than one name.
func overlaps(byName map[string]map[string]bool) []Overlap {
	names := map[string]map[string]bool{}
	for name, addrs := range byName {
		for addr := range addrs {
			if names[addr] == nil {
				names[addr] = map[string]bool{}
			}
			names[addr][name] = true
		}
	}
	var out []Overlap
	for addr, ns := range names {
		if len(ns) > 1 {
			out = append(out, Overlap{Email: addr, Names: sortedKeys(ns)})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Email < out[j].Email })
	return out
}

func matches(want, name string, emails map[string]bool) bool {
	if want == "" {
		return true
	}
	want = strings.ToLower(want)
	if strings.Contains(strings.ToLower(name), want) {
		return true
	}
	for e := range emails {
		if strings.Contains(strings.ToLower(e), want) {
			return true
		}
	}
	return false
}

func offsetOf(stamp string) string {
	if len(stamp) < 6 {
		return ""
	}
	tail := stamp[len(stamp)-6:]
	if tail[0] == '+' || tail[0] == '-' {
		return tail
	}
	if strings.HasSuffix(stamp, "Z") {
		return "+00:00"
	}
	return ""
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func byCount(m map[string]int) []string {
	type kv struct {
		k string
		v int
	}
	all := make([]kv, 0, len(m))
	for k, v := range m {
		all = append(all, kv{k, v})
	}
	sort.Slice(all, func(i, j int) bool {
		if all[i].v != all[j].v {
			return all[i].v > all[j].v
		}
		return all[i].k < all[j].k
	})
	out := make([]string, 0, len(all))
	for _, e := range all {
		out = append(out, fmt.Sprintf("%s (%d)", e.k, e.v))
	}
	return out
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
