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
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/LetA-Tech/mellions-coxen/internal/cite"
	"github.com/LetA-Tech/mellions-coxen/internal/prbody"
)

// citeLimit bounds a document read. A body past this is not a body.
const citeLimit = 1 << 20

// cmdCite checks a document's citations against the tree they name and prints
// the ones it cannot back. Exit 1 where there are findings, so a script or a
// person can gate on it; the hook form is cite-check.
func cmdCite(ctx context.Context, args []string) error {
	if len(args) == 0 || args[0] != "check" {
		return errors.New("cite: the subcommand is `check`")
	}
	fs := newFlagSet("cite check", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	file := fs.String("file", "-", "document to check, or - for stdin")
	dir := fs.String("dir", ".", "the checkout the citations are relative to")
	commit := fs.String("commit", "", "resolve citations at this commit rather than the working tree")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	doc, err := readDoc(*file)
	if err != nil {
		return err
	}
	root := repoRoot(ctx, *dir)
	// A ref this repository cannot resolve makes EVERY citation unreadable, so
	// the check reports a body of unresolved citations and exits 0 — a typo in
	// -commit silently disables the gate and says something reassuring while it
	// does. Refuse before grading anything.
	if err := requireRef(ctx, root, *commit); err != nil {
		return err
	}
	findings, unresolved := cite.Check(doc, resolverAt(ctx, root, *commit))

	// Say which tree answered, always. A citation is graded against whatever
	// checkout the check happened to read, and a shared checkout sitting behind
	// its remote grades at a commit that is neither the branch under review nor
	// its own origin — so a line correct at the reviewed head is refused, and
	// one that happens to land elsewhere in the stale tree is ACCEPTED. Silence
	// about the tree is what lets that green read as verified.
	fmt.Printf("cite: resolved against %s at %s\n", root, treeRef(ctx, root, *commit))
	reportUnresolved(unresolved)

	if len(findings) == 0 {
		fmt.Println("cite: every citation this checkout can resolve is quoted in the body.")
		return nil
	}
	for _, f := range findings {
		fmt.Println("  " + f.Reason())
	}
	return fmt.Errorf("%d citation(s) the body does not back", len(findings))
}

// cmdCiteCheck reads a PreToolUse payload on stdin and denies a `gh` command
// that publishes a body carrying a citation the tree does not back. Everything
// else is silence.
//
// The rule this enforces is mellions-deep-research's own — "Open every
// citation before it is filed" — and it is enforced here rather than in the
// Skill because the Skill states it correctly and it kept failing to bind. The
// moment a body is handed to `gh` is the last one at which the claim is still
// retractable, and a denial becomes a tool result the session must answer.
func cmdCiteCheck(ctx context.Context, args []string) error {
	fs := newFlagSet("cite-check", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}
	payload := readPayload(os.Stdin)
	if len(payload) == 0 {
		guardUsage("cite-check", "It denies a `gh` command publishing a body that cites a "+
			"path:line this checkout resolves and the body does not quote. To check a body "+
			"by hand: `mellions cite check -file <path>`.")
		return nil
	}
	var ev struct {
		ToolName string `json:"tool_name"`
		Cwd      string `json:"cwd"`
		Input    struct {
			Command string `json:"command"`
		} `json:"tool_input"`
	}
	if json.Unmarshal(payload, &ev) != nil || ev.ToolName != "Bash" {
		return nil
	}
	// This runs on every Bash call, so nothing costs anything until the command
	// turns out to publish: building the resolver runs git, and the overwhelming
	// majority of tool calls are not a gh body.
	calls := prbody.Publishing(ev.Input.Command, ev.Cwd)
	if len(calls) == 0 {
		return nil
	}
	cwd := ev.Cwd
	if cwd == "" {
		cwd = "."
	}
	var reasons []string
	// unchecked carries what nothing denied but nothing verified either.
	var unchecked []string
	for _, call := range calls {
		// Per call, and from the directory the call itself runs in. One
		// resolver built from the session directory is #27: a lane publishes
		// with `cd <worktree> && gh pr create`, so the body describes the
		// worktree while the checker read the session's checkout — and one
		// command line can publish from two different trees.
		dir := citeDir(call.Dir, cwd)
		read := resolver(ctx, dir, "")
		for _, body := range call.Bodies {
			findings, unresolved := cite.Check(body, read)
			for _, f := range findings {
				reasons = append(reasons, "  "+f.Reason())
			}
			// Named whether or not anything else denies. Denying a body for
			// citing another repository would refuse exactly the bodies
			// deep-research asks for, so this must inform without blocking —
			// which PreToolUse allows through additionalContext, the channel
			// `mellions state -tool` already publishes on this same event.
			//
			// It reaches the session either way: as a deny reason when
			// something else is wrong, and as context when nothing is.
			if len(unresolved) > 0 {
				line := "  " + unresolvedLine(unresolved, dir)
				if len(findings) > 0 {
					reasons = append(reasons, line)
				} else {
					unchecked = append(unchecked, line)
				}
			}
		}
	}
	if len(reasons) == 0 {
		return emitUnchecked(unchecked)
	}
	// A deny is read, so it is bounded: a body that got its anchoring wrong
	// throughout wants the first several and a count, not a wall the session
	// scrolls past.
	reasons = dedupe(reasons)
	if n := len(reasons); n > 8 {
		reasons = append(reasons[:8], "  … and "+strconv.Itoa(n-8)+" more.")
	}
	var d decision
	d.Output.Event = "PreToolUse"
	d.Output.Decide = "deny"
	d.Output.Reason = "This body publishes a citation the checkout does not back:\n\n" +
		strings.Join(reasons, "\n") + "\n\n" +
		"A reader cannot tell a citation that landed one line off from one that landed " +
		"on the code it claims, so both read as evidence and one is not. Quote the line " +
		"under the citation — the form mellions-deep-research already asks for, and what " +
		"makes the claim checkable at all.\n\n" +
		"Two things produce this and the remedies differ. The number may be wrong, and " +
		"opening the line fixes it. Or this checkout is the wrong subject: a body about a " +
		"branch, a pull request or an older commit is right about that ref and wrong here, " +
		"and re-deriving the numbers against this tree would make it wrong there instead. " +
		"Check which before editing:\n\n" +
		"  mellions cite check -file body.md -dir <checkout> [-commit <ref>]\n\n" +
		"`-commit` resolves every citation at that ref — the branch under review, not the " +
		"one checked out — and reports the same thing without publishing anything. A ref it " +
		"cannot resolve is refused rather than graded, because every citation would read as " +
		"unresolvable and the check would pass having verified nothing."
	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false)
	return enc.Encode(d)
}

// citeDir is the checkout one call's citations resolve in: the directory the
// command moved to, or the session's where it named none. A path this host does
// not hold is not a checkout to read — the `cd` would have failed and the `gh`
// never run — so it degrades to the session directory rather than to a root
// that resolves nothing, which would silently pass every citation.
func citeDir(dir, cwd string) string {
	if dir == "" {
		return cwd
	}
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return cwd
	}
	return dir
}

func dedupe(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

// resolver answers with a file's lines, at a commit where one is named and
// from the working tree otherwise. An error means the path is not a file this
// checkout holds, which the checker reads as "not a citation" rather than as a
// finding — a URL host, another repository's path, or prose that happens to
// carry a colon.
func resolver(ctx context.Context, dir, commit string) func(string) ([]string, error) {
	return resolverAt(ctx, repoRoot(ctx, dir), commit)
}

// resolverAt is resolver with the repository root already established, so a
// caller that needed the root for its own report does not run git twice.
func resolverAt(ctx context.Context, root, commit string) func(string) ([]string, error) {
	return func(path string) ([]string, error) {
		if commit != "" {
			out, err := exec.CommandContext(ctx, "git", "-C", root, "show", commit+":"+path).Output()
			if err != nil {
				// The same separation the working-tree arm makes. Without it
				// `-commit` graded a wrong same-repo path as unresolvable while
				// the working tree called it a finding — the same body passing
				// under one flag and failing under the other, from a tool whose
				// own deny message says -commit "reports the same thing".
				if claimsTreeAt(ctx, root, commit, path) {
					return nil, cite.ErrPathClaimsTree
				}
				return nil, err
			}
			return split(string(out)), nil
		}
		// A path in a body is not this checker's to trust: it is read only
		// inside the checkout, and only as a regular file.
		full := filepath.Join(root, filepath.Clean("/"+path))
		info, err := os.Stat(full)
		if err != nil || !info.Mode().IsRegular() {
			// Two different failures wear one error today, and only one of them
			// is innocent. `mcfo-leankit/agentkit/runtime/exec.go` is another
			// repository's path and this checkout is right not to hold it.
			// `internal/cite/citee.go` is a typo for a file in THIS tree, and
			// reads as the same silence — which is how a wrong same-repo path
			// passes a check named for verifying citations.
			if claimsTree(root, path) {
				return nil, cite.ErrPathClaimsTree
			}
			return nil, errors.New("not a file in this checkout")
		}
		b, err := os.ReadFile(full)
		if err != nil {
			return nil, err
		}
		return split(string(b)), nil
	}
}

func split(s string) []string {
	s = strings.TrimSuffix(s, "\n")
	return strings.Split(s, "\n")
}

// repoRoot is the checkout a relative citation is relative to. Where dir is
// not in a work tree it is used as it stands, which is what a document checked
// outside a repository wants.
func repoRoot(ctx context.Context, dir string) string {
	out, err := exec.CommandContext(ctx, "git", "-C", dir, "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return dir
	}
	if root := strings.TrimSpace(string(out)); root != "" {
		return root
	}
	return dir
}

func readDoc(file string) (string, error) {
	if file == "-" {
		b, err := io.ReadAll(io.LimitReader(os.Stdin, citeLimit))
		return string(b), err
	}
	f, err := os.Open(file)
	if err != nil {
		return "", err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, citeLimit))
	return string(b), err
}

// treeRef names the commit the resolver answered from: the explicit ref when
// one was given, otherwise the working tree and the HEAD it sits on. The HEAD
// is printed even for a working-tree read because it is what tells a reader
// WHICH tree this was, and a stale one is invisible otherwise.
func treeRef(ctx context.Context, root, commit string) string {
	if commit != "" {
		return commit
	}
	out, err := exec.CommandContext(ctx, "git", "-C", root, "rev-parse", "--short", "HEAD").Output()
	if err != nil {
		return "the working tree (no git HEAD here)"
	}
	return "the working tree, HEAD " + strings.TrimSpace(string(out))
}

// reportUnresolved states what the check did not check.
func reportUnresolved(unresolved []cite.Citation) {
	if len(unresolved) == 0 {
		return
	}
	fmt.Println("  " + unresolvedLine(unresolved, ""))
}

// unresolvedLine is the one sentence a reader needs: how many citations this
// checkout could not open, and which. A cross-repo citation is the common and
// legitimate case — mellions-deep-research says code in another repository is
// not evidence until it has been opened, and this check cannot open it — so
// these are reported rather than denied. Reported is the point: unstated, they
// were indistinguishable from citations that passed.
func unresolvedLine(unresolved []cite.Citation, dir string) string {
	const show = 6
	names := make([]string, 0, len(unresolved))
	for _, c := range unresolved {
		names = append(names, c.Raw)
	}
	shown := names
	suffix := ""
	if len(shown) > show {
		shown = shown[:show]
		suffix = fmt.Sprintf(" and %d more", len(names)-show)
	}
	where := "this checkout"
	if dir != "" {
		where = dir
	}
	return fmt.Sprintf("%d citation(s) %s cannot open, so nothing here checked them: %s%s — "+
		"verify these by hand; a cross-repo path is the usual reason and is not a defect.",
		len(unresolved), where, strings.Join(shown, ", "), suffix)
}

// claimsTree reports whether a path asserts it belongs to this checkout: its
// leading segment names a directory the checkout has.
//
// The rule is deliberately the weakest one that separates the two cases. A
// stronger test — say, that every segment but the last resolves — would call a
// path with one wrong directory "another repository's" and let it through,
// which is the failure being closed. A weaker one, treating every unresolvable
// path as this tree's, would deny the cross-repo citations
// mellions-deep-research expects a body to carry and verify by hand.
//
// A single-segment path (`README.md`) names no directory, so it claims the
// tree only if the root holds it — which the caller has already established it
// does not.
func claimsTree(root, path string) bool {
	head, _, ok := strings.Cut(filepath.ToSlash(filepath.Clean(path)), "/")
	if !ok || head == "" || head == "." || head == ".." {
		return false
	}
	info, err := os.Stat(filepath.Join(root, head))
	return err == nil && info.IsDir()
}

// claimsTreeAt is claimsTree against a commit rather than the working tree: the
// path's leading segment is a directory in that commit's tree.
//
// It asks git rather than the filesystem because the two can disagree — a
// directory added since the commit exists on disk and not in the tree, and a
// directory deleted since exists in the tree and not on disk. Grading a
// citation at a ref means asking that ref.
func claimsTreeAt(ctx context.Context, root, commit, path string) bool {
	head, _, ok := strings.Cut(filepath.ToSlash(filepath.Clean(path)), "/")
	if !ok || head == "" || head == "." || head == ".." {
		return false
	}
	out, err := exec.CommandContext(ctx, "git", "-C", root, "cat-file", "-t", commit+":"+head).Output()
	return err == nil && strings.TrimSpace(string(out)) == "tree"
}

// requireRef refuses a -commit this repository cannot resolve.
//
// Without it the failure is silent and reassuring: git cannot read any path at
// a ref that does not exist, so every citation becomes unresolvable, the check
// reports them as unchecked and exits 0. A body denied on the working tree
// passes under a mistyped ref, and the output says "every citation this
// checkout can resolve is quoted in the body" while having resolved none.
func requireRef(ctx context.Context, root, commit string) error {
	if commit == "" {
		return nil
	}
	if err := exec.CommandContext(ctx, "git", "-C", root,
		"rev-parse", "--verify", "--quiet", commit+"^{commit}").Run(); err != nil {
		return fmt.Errorf("cite: %s resolves to no commit in %s — every citation would read as "+
			"unresolvable and the check would pass having verified nothing", commit, root)
	}
	return nil
}

// emitUnchecked tells the session what this check could not verify, without
// standing in the way of the command.
//
// A body whose citations are all unresolvable used to publish in silence: no
// finding, so no deny, so nothing said — indistinguishable from a body whose
// citations were every one verified. Denying instead would refuse the bodies
// deep-research most wants, which cite another repository and are opened by
// hand.
//
// additionalContext is the third option, and it is not hypothetical here: it is
// the channel `mellions state -tool` publishes on, on this same PreToolUse
// event. An earlier comment in this file asserted the runtime offered no such
// channel. It was wrong, and it foreclosed the remedy for whoever read it next.
func emitUnchecked(unchecked []string) error {
	if len(unchecked) == 0 {
		return nil
	}
	unchecked = dedupe(unchecked)
	var c struct {
		Output struct {
			Event   string `json:"hookEventName"`
			Context string `json:"additionalContext"`
		} `json:"hookSpecificOutput"`
	}
	c.Output.Event = "PreToolUse"
	c.Output.Context = "This body publishes citations this checkout could not open:\n\n" +
		strings.Join(unchecked, "\n") + "\n\n" +
		"Nothing is wrong with citing another repository — deep-research asks for it — but " +
		"nothing here verified those lines, and the body reads to a reader exactly like one " +
		"whose citations were all checked. Open them, or say in the body that they are " +
		"unverified from here."
	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false)
	return enc.Encode(c)
}
