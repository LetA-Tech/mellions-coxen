// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

// Package issuegate checks a written issue against the code it cites, before
// the issue is filed.
//
// It exists because the standard alone did not hold. The engineering skill
// already says, in these words, that "a comment claiming the behaviour exists"
// is not proof and that a fixture edited to turn a run green is a failure
// however green the result. A session carrying that text still built a root
// cause on a comment asserting fidelity to code in another repository, without
// opening that repository — which was checked out on the same disk.
//
// The rules here are the same standard expressed as something that either
// happened or did not. None of them asks for more care, and none constrains how
// the work is reasoned about: they read the finished artifact and compare it to
// the tree.
package issuegate

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Finding is one rule violation. Rule is stable and machine-readable; Detail
// says what to do about it.
type Finding struct {
	Rule   string
	Detail string
}

func (f Finding) String() string { return f.Rule + ": " + f.Detail }

// Rule names. Stable — the skill and the tests both refer to them.
const (
	// RuleUncitedRepo fires when the body names a repository other than the
	// one being worked and cites nothing inside it. This is the rule that the
	// cash-cliff issue failed: three prose references to a consumer service,
	// zero citations into it, and the central claim — that a test-local model
	// faithfully reproduces that service's gate ladder — false at HEAD.
	RuleUncitedRepo = "uncited-cross-repo-claim"
	// RuleMissingFile fires when a citation names a path that is not there.
	RuleMissingFile = "citation-does-not-resolve"
	// RuleQuoteMismatch fires when a quoted block is not in the file the
	// nearest citation names.
	RuleQuoteMismatch = "quote-not-in-cited-file"
	// RuleNoCitations fires when a body makes a case and cites nothing at all.
	RuleNoCitations = "no-citations"
)

// Citation is one path:line reference in the body.
type Citation struct {
	// Repo is the repository the path is rooted in. Empty means the working
	// repository — a path written relative to it, which is how a session
	// working inside one checkout naturally writes.
	Repo string
	// Path is the path within Repo.
	Path string
	// Line is the cited line number.
	Line int
	// At is the 0-based line in the body where the citation appears, used to
	// anchor the quoted blocks that follow it.
	At int
}

// String renders the citation the way it appears in a body.
func (c Citation) String() string {
	if c.Repo == "" {
		return fmt.Sprintf("%s:%d", c.Path, c.Line)
	}
	return fmt.Sprintf("%s/%s:%d", c.Repo, c.Path, c.Line)
}

// citePattern matches path:line. The extension list is deliberate: a bare
// word:number ("issue 530:1") is not a citation, and treating it as one would
// produce a missing-file finding for prose.
var citePattern = regexp.MustCompile(
	`([A-Za-z0-9_.-]+(?:/[A-Za-z0-9_.-]+)*\.(?:go|md|sql|ya?ml|json|sh|ts|tsx|js|py|proto|toml)):(\d+)`)

// Citations returns every path:line in the body, attributed to a repository.
//
// Attribution is by prefix and nothing else: a path beginning with a known
// repository name belongs to that repository, anything else is relative to the
// working repository. That is a convention rather than a discovery, and it is
// the convention precisely because it is unambiguous — a session cannot cite
// another repository by accident, and cannot satisfy the cross-repo rule
// without naming the repository in the path it read.
func Citations(body string, known map[string]string) []Citation {
	var out []Citation
	fenced := false
	for i, line := range strings.Split(body, "\n") {
		if isFence(line) {
			fenced = !fenced
			continue
		}
		// Text inside a fence is quoted material, not a claim the author is
		// making. A test transcript names file.go:NNN on every failing line,
		// and reading those as citations turns pasted evidence into findings.
		if fenced {
			continue
		}
		for _, m := range citePattern.FindAllStringSubmatch(line, -1) {
			path := m[1]
			n, err := strconv.Atoi(m[2])
			if err != nil {
				continue
			}
			c := Citation{Path: path, Line: n, At: i}
			if head, rest, ok := strings.Cut(path, "/"); ok {
				if _, isRepo := known[head]; isRepo {
					c.Repo, c.Path = head, rest
				}
			}
			out = append(out, c)
		}
	}
	return out
}

// NamedRepos returns the known repositories the body mentions, excluding the
// one being worked.
func NamedRepos(body string, working string, known map[string]string) []string {
	var out []string
	for repo := range known {
		if repo == working {
			continue
		}
		// Word boundaries keep a repository prefix from matching a longer name.
		// A repository named inside a longer identifier does not count either.
		re := regexp.MustCompile(`\b` + regexp.QuoteMeta(repo) + `\b`)
		if re.MatchString(body) {
			out = append(out, repo)
		}
	}
	sort.Strings(out)
	return out
}

// Check reports every rule the body fails.
//
// known maps repository name to checkout path; working is the repository the
// issue is filed against. A nil or empty result means the body's citations all
// resolve and every repository it names is cited.
func Check(body, working string, known map[string]string) []Finding {
	var findings []Finding
	cites := Citations(body, known)

	if len(cites) == 0 {
		findings = append(findings, Finding{
			Rule:   RuleNoCitations,
			Detail: "the body cites no file:line at all; a claim about code is unverifiable without one",
		})
	}

	// 1. Every repository named must be cited into.
	cited := map[string]bool{}
	for _, c := range cites {
		if c.Repo != "" {
			cited[c.Repo] = true
		}
	}
	for _, repo := range NamedRepos(body, working, known) {
		if cited[repo] {
			continue
		}
		findings = append(findings, Finding{
			Rule: RuleUncitedRepo,
			Detail: fmt.Sprintf(
				"names %s but cites nothing in it; read %s and cite it as %s/<path>:<line>, or drop the claim",
				repo, known[repo], repo),
		})
	}

	// 2. Every citation must resolve to a file with that many lines.
	seen := map[string]bool{}
	for _, c := range cites {
		abs, err := resolve(c, working, known)
		if err == errAmbiguous {
			continue
		}
		if err != nil {
			if !seen[c.String()] {
				seen[c.String()] = true
				findings = append(findings, Finding{RuleMissingFile, fmt.Sprintf("%s: %v", c, err)})
			}
			continue
		}
		n, err := countLines(abs)
		if err != nil {
			if !seen[c.String()] {
				seen[c.String()] = true
				findings = append(findings, Finding{RuleMissingFile, fmt.Sprintf("%s: %v", c, err)})
			}
			continue
		}
		if c.Line > n && !seen[c.String()] {
			seen[c.String()] = true
			findings = append(findings, Finding{
				Rule:   RuleMissingFile,
				Detail: fmt.Sprintf("%s: file has %d lines", c, n),
			})
		}
	}

	// 3. Every quoted block must be in the file its nearest citation names.
	findings = append(findings, checkQuotes(body, working, known, cites)...)
	return findings
}

// resolve turns a citation into an absolute path.
// Locate reports where a citation's file actually is, and whether it was found
// at all.
//
// The distinction matters to any caller asking whether a claim has gone stale.
// "The file exists and is shorter than the cited line" establishes that the tree
// moved. "No file of that name anywhere in this checkout" establishes nothing on
// its own: the body may be citing a sibling repository the caller does not have.
// Treating the second as evidence of staleness invents findings — a body citing
// eighteen migrations that live in another repository is not eighteen deletions.
func Locate(c Citation, working string, known map[string]string) (path string, found bool) {
	abs, err := resolve(c, working, known)
	return abs, err == nil
}

// Checkable reports whether a citation that did not resolve is one this could
// have checked at all.
//
// The distinction decides what a failure means. A path this repository does not
// hold is a claim it does not satisfy, and worth reading the code over. An
// elided path is prose, and an ambiguous one names two files — neither is a
// claim anybody made, and reporting them as moved premises invents findings
// about files nobody named. Collapsing every failure to "not found" made both
// indistinguishable from the first.
func Checkable(c Citation, working string, known map[string]string) bool {
	_, err := resolve(c, working, known)
	switch {
	case err == nil:
		return true
	case errors.Is(err, errElided), errors.Is(err, errAmbiguous):
		return false
	default:
		return true
	}
}

func resolve(c Citation, working string, known map[string]string) (string, error) {
	repo := c.Repo
	if repo == "" {
		repo = working
	}
	root, ok := known[repo]
	if !ok {
		return "", fmt.Errorf("no checkout known for %s", repo)
	}
	abs := filepath.Join(root, c.Path)
	if _, err := os.Stat(abs); err == nil {
		return abs, nil
	}
	// Absent here, and the estate has other checkouts. A cross-repository issue
	// cites sibling paths without naming the sibling — the writer knew which
	// repository they meant — and calling those "no such path" made twenty-one
	// issues read as stale premises when every cited file was present one
	// directory over. The information to tell the two apart was already in this
	// map and was not being used.
	//
	// Exactly one sibling holding it is an answer. Two is ambiguous, and
	// guessing which was meant would invent a finding.
	if c.Repo == "" && strings.Contains(c.Path, "/") {
		var hits []string
		for name, r := range known {
			if name == repo {
				continue
			}
			cand := filepath.Join(r, c.Path)
			if _, err := os.Stat(cand); err == nil {
				hits = append(hits, cand)
			}
		}
		switch len(hits) {
		case 1:
			return hits[0], nil
		case 0:
		default:
			return "", errAmbiguous
		}
		// The tail of a path, which is also how people write once a file has
		// been named in full higher up: `periodclose/models.go` for
		// `internal/edge/service/periodclose/models.go`. Same rule as a
		// basename — one match answers, more than one is uncheckable.
		var tails []string
		for _, r := range known {
			tails = append(tails, findBySuffix(r, c.Path)...)
			if len(tails) > 1 {
				break
			}
		}
		if len(tails) == 1 {
			return tails[0], nil
		}
	}
	// An elided path is prose, not a claim. People write
	// `internal/.../period_close.go` to mean "somewhere under internal", and
	// treating that as a path this repository should hold reports a moved
	// premise about a file nobody ever named.
	if strings.Contains(c.Path, "/.../") || strings.Contains(c.Path, "…") {
		return "", errElided
	}
	// A body that has already established a file names it by basename after
	// that, which is how people write. Resolve it by search rather than
	// refusing it, and treat an ambiguous name as uncheckable rather than
	// wrong: guessing which of two matches was meant would invent a finding.
	if !strings.Contains(c.Path, "/") {
		switch matches := findByBase(root, c.Path); len(matches) {
		case 1:
			return matches[0], nil
		case 0:
			return "", fmt.Errorf("no file named %s in %s", c.Path, repo)
		default:
			return "", errAmbiguous
		}
	}
	return "", fmt.Errorf("not found at %s", abs)
}

// errAmbiguous marks a basename that matches more than one file. It is not
// reported: the citation may well be correct, and the gate refuses only what it
// can establish is wrong.
var errAmbiguous = fmt.Errorf("ambiguous basename")

// errElided is a path written with a gap in it. It is prose rather than a
// claim, and reporting it as a path this repository does not hold invents a
// finding about a file nobody named.
var errElided = fmt.Errorf("elided path")

// baseIndex caches one walk per repository root. An issue cites the same few
// files repeatedly, and walking a large checkout per citation is the difference
// between a gate that runs in the filing path and one that gets skipped.
var baseIndex = map[string]map[string][]string{}

func findByBase(root, base string) []string {
	idx, ok := baseIndex[root]
	if !ok {
		idx = map[string][]string{}
		_ = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				switch n := d.Name(); {
				case n == ".git", n == "node_modules", n == "vendor":
					return filepath.SkipDir
				}
				// A nested checkout is another copy of the same repository, not a
				// sibling candidate for citation resolution.
				if p != root {
					if _, err := os.Stat(filepath.Join(p, ".git")); err == nil {
						return filepath.SkipDir
					}
				}
				return nil
			}
			idx[d.Name()] = append(idx[d.Name()], p)
			return nil
		})
		baseIndex[root] = idx
	}
	return idx[base]
}

// findBySuffix locates files whose path ends with the given tail.
//
// The same reuse as findByBase, one level up: the basename index already knows
// every file, so a tail is a filter over the entries sharing its last segment
// rather than a second walk.
func findBySuffix(root, tail string) []string {
	tail = strings.TrimPrefix(tail, "/")
	base := tail
	if i := strings.LastIndex(tail, "/"); i >= 0 {
		base = tail[i+1:]
	}
	var out []string
	for _, p := range findByBase(root, base) {
		if strings.HasSuffix(filepath.ToSlash(p), "/"+filepath.ToSlash(tail)) {
			out = append(out, p)
		}
	}
	return out
}

func countLines(path string) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	n := 0
	for sc.Scan() {
		n++
	}
	return n, sc.Err()
}

// quoteAnchorWindow is how far above a fenced block a citation may sit and
// still be read as naming the block's source. A body writes the citation, a
// sentence, a blank line, then the fence; beyond a few lines the association is
// a guess, and a guess would produce a false finding.
const quoteAnchorWindow = 6

// checkQuotes verifies that fenced code blocks appear in the file cited just
// above them.
//
// A block with no citation above it is not checked: plenty of blocks are shell
// transcripts, diffs or diagrams that no file contains. The rule bites exactly
// where a body says "here is the code, at this line" — the claim that reads as
// strongest and is the one worth being able to trust.
func checkQuotes(body, working string, known map[string]string, cites []Citation) []Finding {
	lines := strings.Split(body, "\n")
	var findings []Finding
	reported := map[string]bool{}

	for i := 0; i < len(lines); i++ {
		if !isFence(lines[i]) {
			continue
		}
		lang := strings.TrimSpace(strings.TrimLeft(lines[i], "`~"))
		end := i + 1
		for end < len(lines) && !isFence(lines[end]) {
			end++
		}
		block := lines[i+1 : min(end, len(lines))]
		i = end

		// Only code blocks are checkable against a source file.
		if !checkableLang(lang) {
			continue
		}
		anchor, ok := nearestCitation(cites, i-len(block)-1)
		if !ok {
			continue
		}
		abs, err := resolve(anchor, working, known)
		if err != nil {
			continue // already reported by rule 2
		}
		src, err := os.ReadFile(abs)
		if err != nil {
			continue
		}
		missing := firstMissingLine(block, string(src))
		if missing == "" {
			continue
		}
		key := anchor.String() + "\x00" + missing
		if reported[key] {
			continue
		}
		reported[key] = true
		findings = append(findings, Finding{
			Rule: RuleQuoteMismatch,
			Detail: fmt.Sprintf("block attributed to %s quotes a line that is not in %s: %q",
				anchor, abs, truncate(missing, 90)),
		})
	}
	return findings
}

func isFence(s string) bool {
	t := strings.TrimSpace(s)
	return strings.HasPrefix(t, "```") || strings.HasPrefix(t, "~~~")
}

// checkableLang reports whether a fence's language tag names something that
// lives in a source file. A text, shell or diff block is output, not source.
func checkableLang(lang string) bool {
	fields := strings.Fields(lang)
	if len(fields) == 0 {
		return false // a bare fence is output, not source
	}
	switch strings.ToLower(fields[0]) {
	case "go", "sql", "yaml", "yml", "json", "ts", "tsx", "js", "py", "python", "proto", "toml":
		return true
	}
	return false
}

// nearestCitation returns the last citation at or above line n, within the
// anchor window.
func nearestCitation(cites []Citation, n int) (Citation, bool) {
	best, ok := Citation{}, false
	for _, c := range cites {
		if c.At <= n && n-c.At <= quoteAnchorWindow {
			if !ok || c.At > best.At {
				best, ok = c, true
			}
		}
	}
	return best, ok
}

// firstMissingLine returns the first meaningful line of the block that does not
// appear in src, or "" when every line does.
//
// Compared trimmed, because a quote is reindented to sit inside prose, and
// order is not required: a body legitimately quotes a signature and its guard
// with the middle elided. What it catches is a line that is not in the file at
// all — which is what a remembered or reconstructed quote produces.
func firstMissingLine(block []string, src string) string {
	// Index the source once, trimmed, so a long block is not O(n·m) over bytes.
	inSrc := make(map[string]struct{}, 1024)
	for _, l := range strings.Split(src, "\n") {
		inSrc[strings.TrimSpace(l)] = struct{}{}
	}
	for _, raw := range block {
		l := strings.TrimSpace(raw)
		if !meaningful(l) {
			continue
		}
		if _, ok := inSrc[l]; !ok {
			return l
		}
	}
	return ""
}

// meaningful reports whether a quoted line carries enough to be worth
// matching. Elisions, bare delimiters and very short fragments are skipped:
// they appear in every file or in none, and either way say nothing about
// whether the quote is real.
func meaningful(l string) bool {
	if len(l) < 8 {
		return false
	}
	switch l {
	case "...", "…", "// ...", "// …", "# ...", "/* ... */":
		return false
	}
	if strings.HasPrefix(l, "// ...") || strings.HasPrefix(l, "# ...") {
		return false
	}
	return true
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
