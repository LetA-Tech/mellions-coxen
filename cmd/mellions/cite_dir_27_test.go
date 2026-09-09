package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/LetA-Tech/mellions-coxen/internal/cite"
)

// #27. The hook built one resolver from the session's cwd, so a body published
// by `cd <worktree> && gh pr create` was checked against the shared checkout.
// Both directions of that are wrong and only one of them is loud:
//
//   - loud:      a citation right for the lane is DENIED, and the session sees it.
//   - dangerous: a citation right only for the shared checkout is ACCEPTED, and
//     nobody sees it. That one publishes a wrong claim under a passing check,
//     and it is the arm these tests exist for.
//
// The fixture is two real checkouts whose line 3 differs in both directions, so
// neither verdict can be reached by accident.

func citeFixture(t *testing.T) (session, lane string) {
	t.Helper()
	root := t.TempDir()
	session = filepath.Join(root, "shared")
	lane = filepath.Join(root, "lane")

	for dir, line3 := range map[string]string{
		session: "\thealth, err := neonlink.NewHealthChecker(sdkCfg)",
		lane:    "\tproducer, err := neonlink.NewTransactionalProducer(sdkCfg, logger)",
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		for _, args := range [][]string{{"init", "-q"}, {"config", "user.email", "t@t"}, {"config", "user.name", "t"}} {
			cmd := exec.Command("git", args...)
			cmd.Dir = dir
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("git %v: %v: %s", args, err, out)
			}
		}
		body := "package broker\n\n" + line3 + "\n"
		if err := os.WriteFile(filepath.Join(dir, "neonlink.go"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return session, lane
}

// runCiteHook drives the real entry point: a PreToolUse payload on stdin, the
// decision on stdout. It returns the deny reason, or "" for silence.
func runCiteHook(t *testing.T, sessionCwd, command string) string {
	return runCiteHookRaw(t, sessionCwd, command, nil)
}

// runCiteHookRaw is runCiteHook with the hook's whole output available, which a
// test about a NON-deny output needs: the wrapper returns the deny reason and
// so reads an additionalContext response as silence.
func runCiteHookRaw(t *testing.T, sessionCwd, command string, rawOut *string) string {
	t.Helper()
	t.Setenv("MELLIONS_HOOK", "1")

	payload, err := json.Marshal(map[string]any{
		"tool_name":  "Bash",
		"cwd":        sessionCwd,
		"tool_input": map[string]string{"command": command},
	})
	if err != nil {
		t.Fatal(err)
	}

	in := filepath.Join(t.TempDir(), "payload.json")
	if err := os.WriteFile(in, payload, 0o644); err != nil {
		t.Fatal(err)
	}
	stdinFile, err := os.Open(in)
	if err != nil {
		t.Fatal(err)
	}
	defer stdinFile.Close()

	outPath := filepath.Join(t.TempDir(), "out.json")
	stdoutFile, err := os.Create(outPath)
	if err != nil {
		t.Fatal(err)
	}

	oldIn, oldOut := os.Stdin, os.Stdout
	os.Stdin, os.Stdout = stdinFile, stdoutFile
	err = cmdCiteCheck(context.Background(), nil)
	os.Stdin, os.Stdout = oldIn, oldOut
	stdoutFile.Close()
	if err != nil {
		t.Fatalf("cite-check: %v", err)
	}

	raw, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	if rawOut != nil {
		*rawOut = string(raw)
	}
	if len(strings.TrimSpace(string(raw))) == 0 {
		return ""
	}
	var d struct {
		Output struct {
			Decide string `json:"permissionDecision"`
			Reason string `json:"permissionDecisionReason"`
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal(raw, &d); err != nil {
		t.Fatalf("decode decision %q: %v", raw, err)
	}
	if d.Output.Decide != "deny" {
		return ""
	}
	return d.Output.Reason
}

func citeBody(t *testing.T, dir, quoted string) string {
	t.Helper()
	path := filepath.Join(dir, "body.md")
	doc := "Root cause.\n\n`neonlink.go:3`\n\n```go\n" + quoted + "\n```\n"
	if err := os.WriteFile(path, []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// THE ARM THAT COUNTS. A body quoting the SHARED checkout's line 3 is wrong for
// the tree it is published from. Before #27 the hook read that same shared
// checkout, found the quote matched, and approved it — a wrong citation
// published under a passing check.
func TestCiteHook_RejectsACitationRightOnlyForTheSessionCheckout(t *testing.T) {
	session, lane := citeFixture(t)
	body := citeBody(t, t.TempDir(), "\thealth, err := neonlink.NewHealthChecker(sdkCfg)")

	reason := runCiteHook(t, session, "cd "+lane+" && gh pr create --base dev --body-file "+body)

	if reason == "" {
		t.Fatal("the hook approved a citation whose quoted line is right only for the session " +
			"checkout and wrong in the worktree the body is published from — #27's dangerous " +
			"direction, which publishes a false claim with nothing to show for it")
	}
	if !strings.Contains(reason, "neonlink.go:3") {
		t.Fatalf("denied, but not for the citation under test: %s", reason)
	}
}

// The loud direction, and the control that stops the fix from being "deny more".
// A body quoting the LANE's line 3 is correct for the tree it is published from
// and must pass.
func TestCiteHook_AcceptsACitationCorrectForTheWorktreeItPublishesFrom(t *testing.T) {
	session, lane := citeFixture(t)
	body := citeBody(t, t.TempDir(), "\tproducer, err := neonlink.NewTransactionalProducer(sdkCfg, logger)")

	if reason := runCiteHook(t, session, "cd "+lane+" && gh pr create --base dev --body-file "+body); reason != "" {
		t.Fatalf("a citation correct for the worktree was denied: %s", reason)
	}
}

// Without a cd the session directory is still the subject, so the fix does not
// change what a plain publish is checked against.
func TestCiteHook_WithoutACdStillChecksTheSessionCheckout(t *testing.T) {
	session, _ := citeFixture(t)
	wrong := citeBody(t, t.TempDir(), "\tproducer, err := neonlink.NewTransactionalProducer(sdkCfg, logger)")
	right := citeBody(t, t.TempDir(), "\thealth, err := neonlink.NewHealthChecker(sdkCfg)")

	if reason := runCiteHook(t, session, "gh pr create --base dev --body-file "+wrong); reason == "" {
		t.Fatal("a citation wrong for the session checkout passed a publish that never left it")
	}
	if reason := runCiteHook(t, session, "gh pr create --base dev --body-file "+right); reason != "" {
		t.Fatalf("a citation correct for the session checkout was denied: %s", reason)
	}
}

// A cd to a directory this host does not hold must not become a root that
// resolves nothing: a resolver that resolves nothing reports every citation
// unbacked-but-not-a-citation and passes the body silently, which would turn
// the fix into a wider hole than the defect.
func TestCiteHook_UnreachableCdFallsBackToTheSessionCheckout(t *testing.T) {
	session, _ := citeFixture(t)
	wrong := citeBody(t, t.TempDir(), "\tproducer, err := neonlink.NewTransactionalProducer(sdkCfg, logger)")

	missing := filepath.Join(t.TempDir(), "does-not-exist")
	if reason := runCiteHook(t, session, "cd "+missing+" && gh pr create --base dev --body-file "+wrong); reason == "" {
		t.Fatal("a cd to a nonexistent directory silenced the checker instead of falling back " +
			"to the session checkout")
	}
}

// TestResolver_AWrongSameRepoPathIsAFindingNotSilence drives the REAL resolver,
// which is where claimsTree lives and where the cite package's own tests cannot
// reach: they hand Check a fake read, so the predicate that decides whether a
// path claims this tree is never exercised by them.
//
// That gap was found by its own falsification arm — neutralising claimsTree to
// always return false reddened nothing in the suite, which is the signature of a
// production branch no test drives.
func TestResolver_AWrongSameRepoPathIsAFindingNotSilence(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "internal", "cite"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "internal", "cite", "cite.go"),
		[]byte("package cite\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	read := resolver(context.Background(), root, "")

	// A path whose leading segment is a directory this tree has, naming a file
	// it does not: the hipsys#123 shape.
	if _, err := read("internal/cite/citee.go"); !errors.Is(err, cite.ErrPathClaimsTree) {
		t.Errorf("a wrong same-repo path gave %v, want ErrPathClaimsTree — without it the citation "+
			"is dropped and the body publishes green", err)
	}
	// Another repository's path: the leading segment is no directory here.
	if _, err := read("mcfo-leankit/agentkit/runtime/exec.go"); err == nil {
		t.Error("a cross-repo path resolved, which the fixture makes impossible")
	} else if errors.Is(err, cite.ErrPathClaimsTree) {
		t.Error("a cross-repo path was treated as claiming this tree — it would be DENIED, and " +
			"deep-research expects bodies to cite other repositories and verify them by hand")
	}
	// The control: a path that does resolve must not take either error branch.
	if _, err := read("internal/cite/cite.go"); err != nil {
		t.Errorf("a path this tree holds gave %v; the two assertions above would be vacuous "+
			"against a resolver that fails everything", err)
	}
}

// TestCiteCheck_ReportsTheTreeAndWhatItCouldNotOpen drives the CLI end to end.
//
// The reporting half — which tree answered, and which citations went unchecked —
// had no test at all: Check returning the unresolved slice was covered, and
// nothing asserted that any caller PRINTS it. That is the same gap that let the
// original defect stand, one layer out: a report nobody reads is a silence with
// extra steps, and coverage of the producer says nothing about the consumer.
func TestCiteCheck_ReportsTheTreeAndWhatItCouldNotOpen(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "internal"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "internal", "a.go"),
		[]byte("package a\nsecond line\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	body := filepath.Join(root, "body.md")
	if err := os.WriteFile(body, []byte(
		"see `internal/a.go:2`:\n\n```go\nsecond line\n```\n\nand `runtime/exec.go:1845` upstream.\n",
	), 0o644); err != nil {
		t.Fatal(err)
	}

	var err error
	out := captureStdout(t, func() {
		err = cmdCite(context.Background(), []string{"check", "-file", body, "-dir", root})
	})
	if err != nil {
		t.Fatalf("cmdCite: %v — the body's one resolvable citation is quoted, so this must pass", err)
	}
	if !strings.Contains(out, "resolved against "+root) {
		t.Errorf("output does not name the checkout it graded against; a stale tree is invisible "+
			"without it.\ngot: %s", out)
	}
	if !strings.Contains(out, "runtime/exec.go:1845") {
		t.Errorf("output does not name the citation it could not open, so a body of unverifiable "+
			"citations reads exactly like a verified one.\ngot: %s", out)
	}
}

// TestResolver_CommitGradesTheSameWayAsTheWorkingTree pins the two arms to one
// verdict.
//
// `-commit` returned git's raw error for every unreadable path, so a wrong
// same-repo path was a finding on the working tree and unresolvable at a ref —
// the same body passing under one flag and failing under the other, from a tool
// whose deny message tells sessions `-commit` "reports the same thing".
//
// The predicate asks git rather than the filesystem, because the two disagree
// exactly where it matters: a directory added since the commit is on disk and
// not in the tree.
func TestResolver_CommitGradesTheSameWayAsTheWorkingTree(t *testing.T) {
	root := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, out)
		}
	}
	run("init", "-q", "-b", "main")
	if err := os.MkdirAll(filepath.Join(root, "internal"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "internal", "a.go"), []byte("package a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "-A")
	run("commit", "-qm", "one")

	head := strings.TrimSpace(gitOut(root, "rev-parse", "HEAD"))
	const wrong = "internal/aa.go" // internal/ is a directory here; this file is not

	tree := resolver(context.Background(), root, "")
	atRef := resolver(context.Background(), root, head)

	_, treeErr := tree(wrong)
	_, refErr := atRef(wrong)
	if !errors.Is(treeErr, cite.ErrPathClaimsTree) {
		t.Fatalf("working tree gave %v, want ErrPathClaimsTree — the control for this test", treeErr)
	}
	if !errors.Is(refErr, cite.ErrPathClaimsTree) {
		t.Errorf("-commit gave %v, want ErrPathClaimsTree — the same path must reach the same "+
			"verdict at a ref as in the tree, or the flag silently changes what passes", refErr)
	}

	// The other direction: a path neither knows is unresolvable in both.
	if _, err := tree("elsewhere/x.go"); errors.Is(err, cite.ErrPathClaimsTree) {
		t.Error("working tree treated a cross-repo path as claiming the tree")
	}
	if _, err := atRef("elsewhere/x.go"); errors.Is(err, cite.ErrPathClaimsTree) {
		t.Error("-commit treated a cross-repo path as claiming the tree")
	}
}

// TestRequireRef_ARefThatResolvesToNothingIsRefused closes the largest hole a
// reviewer found in this check: git can read no path at a ref that does not
// exist, so every citation becomes unresolvable, the body is reported as
// unchecked, and the command exits 0 saying "every citation this checkout can
// resolve is quoted in the body" — having resolved none. A typo in -commit
// silently disabled the gate and was reassuring while it did.
func TestRequireRef_ARefThatResolvesToNothingIsRefused(t *testing.T) {
	root := citeGitRepo(t)
	if err := requireRef(context.Background(), root, "no-such-ref"); err == nil {
		t.Error("a ref resolving to no commit was accepted; every citation would read as " +
			"unresolvable and the check would pass having verified nothing")
	}
	if err := requireRef(context.Background(), root, "HEAD"); err != nil {
		t.Errorf("HEAD was refused (%v) — the assertion above would be vacuous against a "+
			"requireRef that rejects everything", err)
	}
	if err := requireRef(context.Background(), root, ""); err != nil {
		t.Errorf("the working-tree case must not be refused: %v", err)
	}

	// And that cmdCite actually calls it. Testing the predicate alone leaves the
	// call site undriven — deleting the guard from cmdCite reddened nothing, so
	// the three assertions above were about a function nothing had to invoke.
	body := filepath.Join(root, "body.md")
	if err := os.WriteFile(body, []byte("see `internal/a.go:1` somewhere.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := cmdCite(context.Background(), []string{"check", "-file", body, "-dir", root, "-commit", "no-such-ref"})
	if err == nil {
		t.Error("cmdCite accepted a ref resolving to no commit — the gate is disabled by a typo " +
			"and reports that every citation it can resolve is quoted")
	}
	if err != nil && !strings.Contains(err.Error(), "resolves to no commit") {
		t.Errorf("cmdCite failed for the wrong reason: %v", err)
	}
}

// TestClaimsTreeAt_OnlyADirectoryClaimsTheTree drives the comparison that
// decides it. `git cat-file -t <commit>:<segment>` answers "tree" for a
// directory and "blob" for a file, and only the first means the path claims
// this tree. Accepting any successful answer would call `README.md/x.go` a
// same-repo path and re-open the arm divergence #69 closed — and it would pull
// a submodule gitlink into the finding path too.
//
// A reviewer found this comparison had no test: mutating it to `err == nil`
// left all 28 packages green.
func TestClaimsTreeAt_OnlyADirectoryClaimsTheTree(t *testing.T) {
	root := citeGitRepo(t)
	ctx := context.Background()
	head := strings.TrimSpace(gitOut(root, "rev-parse", "HEAD"))

	// internal/ is a directory at HEAD: a path under it claims this tree.
	if !claimsTreeAt(ctx, root, head, "internal/missing.go") {
		t.Error("a path under a directory in the commit did not claim the tree")
	}
	// top.txt is a FILE at HEAD. A path under it is not this tree's, and
	// cat-file answers "blob" rather than failing — which is why the comparison
	// and not merely the error is what decides.
	if claimsTreeAt(ctx, root, head, "top.txt/nested.go") {
		t.Error("a path under a FILE was treated as claiming the tree; the check accepts any " +
			"answer git gives rather than requiring a directory")
	}
	// A segment the commit does not have at all.
	if claimsTreeAt(ctx, root, head, "elsewhere/x.go") {
		t.Error("a path under a segment absent from the commit claimed the tree")
	}
}

// citeGitRepo is a one-commit repository with a directory and a top-level file,
// which is what separates "tree" from "blob" above.
func citeGitRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, out)
		}
	}
	run("init", "-q", "-b", "main")
	if err := os.MkdirAll(filepath.Join(root, "internal"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "internal", "a.go"), []byte("package a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "top.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "-A")
	run("commit", "-qm", "one")
	return root
}

// TestCiteHook_AnAllUnresolvableBodyIsNoLongerSilent drives the real hook entry
// point with a PreToolUse payload.
//
// A body whose citations are all unresolvable produced no finding, so no deny,
// so nothing at all — indistinguishable from a body whose citations were every
// one verified. Two comments in this file previously explained that silence
// away, the second asserting the runtime offered no channel to speak without
// blocking. It does, and `mellions state -tool` was already using it.
func TestCiteHook_AnAllUnresolvableBodyIsNoLongerSilent(t *testing.T) {
	root := citeGitRepo(t)
	body := filepath.Join(root, "b.md")
	if err := os.WriteFile(body,
		[]byte("upstream abandons it at `runtime/exec.go:1845`.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out string
	runCiteHookRaw(t, root, "gh pr create --base dev --body-file "+body, &out)

	if strings.TrimSpace(out) == "" {
		t.Fatal("the hook said nothing about a body none of whose citations it could open — " +
			"that silence reads exactly like a body whose citations were all verified")
	}
	if strings.Contains(out, "permissionDecision") {
		t.Error("the hook DENIED a body whose only fault is citing another repository; " +
			"deep-research asks for those citations and this must inform, not block")
	}
	if !strings.Contains(out, "additionalContext") || !strings.Contains(out, "runtime/exec.go:1845") {
		t.Errorf("the context does not name the citation that went unchecked.\ngot: %s", out)
	}
}

// TestCiteHook_AnUnbackedCitationStillDenies is the control: informing about the
// unresolved must not have replaced denying on a real finding.
func TestCiteHook_AnUnbackedCitationStillDenies(t *testing.T) {
	root := citeGitRepo(t)
	body := filepath.Join(root, "b.md")
	if err := os.WriteFile(body, []byte("see `internal/a.go:1` for it.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out string
	runCiteHookRaw(t, root, "gh pr create --base dev --body-file "+body, &out)
	if !strings.Contains(out, `"permissionDecision":"deny"`) {
		t.Errorf("an unbacked same-repo citation no longer denies.\ngot: %s", out)
	}
}

// TestRequireRef_TheCommitPeelIsWhatMakesItACommitProbe drives `^{commit}`.
//
// Without the peel, rev-parse --verify accepts any valid OBJECT, so a blob or
// tree ref passes the guard and then resolves no path — restoring the false
// green the guard exists to stop. A reviewer found the token undriven: dropping
// it left the whole package passing.
func TestRequireRef_TheCommitPeelIsWhatMakesItACommitProbe(t *testing.T) {
	root := citeGitRepo(t)
	ctx := context.Background()

	// A blob ref: a real object, not a commit.
	blob := strings.TrimSpace(gitOut(root, "rev-parse", "HEAD:top.txt"))
	if blob == "" {
		t.Fatal("fixture produced no blob; the assertion below would be vacuous")
	}
	if err := requireRef(ctx, root, blob); err == nil {
		t.Error("a blob object was accepted as a commit — it resolves no path, so every citation " +
			"reads as unresolvable and the check passes having verified nothing")
	}
	// An annotated tag: a tag object, and the peel is what makes it work.
	cmd := exec.Command("git", "tag", "-a", "v1", "-m", "one")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git tag: %v: %s", err, out)
	}
	if err := requireRef(ctx, root, "v1"); err != nil {
		t.Errorf("an annotated tag was refused (%v) — the peel exists so it is not", err)
	}
}

// TestHookOutputs_AllCarryTheEventTheRuntimeDispatchesOn pins the one string
// that silently disables a safeguard.
//
// A runtime that does not recognise hookEventName discards the message without
// error, so a typo leaves every test green while a deny never denies and a
// context never informs. It was a bare literal at six sites in this package —
// the citation check's two outputs, the closing-reference, shared-tree,
// credential-read and merge safeguards, and the awareness state — and pinned at
// none.
//
// The assertions are separate tests on purpose. An earlier version guarded the
// constant with t.Fatalf and then checked both emitters below it, so a mutation
// of the constant aborted before either emitter assertion ran: the arm proved
// the guard fired and nothing about what it was said to protect. Split, each
// mutation reds its own assertion.
func TestHookOutputs_AllCarryTheEventTheRuntimeDispatchesOn(t *testing.T) {
	if preToolUseEvent != "PreToolUse" {
		t.Errorf("preToolUseEvent = %q; the runtime dispatches on this exact string and discards "+
			"anything else in silence", preToolUseEvent)
	}
	// Every place a non-test file names the event must take the constant.
	//
	// The earlier form of this scan searched for the correctly-spelled literal,
	// which caught the tidy mistake and missed the dangerous one: a new emitter
	// writing "PreToolUsee" passed it, and that misspelling IS the defect this
	// exists to prevent — a routing key the runtime does not recognise, so the
	// object is discarded, the hook exits 0 and the guard is deaf with no
	// symptom. It checked the precursor rather than the property.
	//
	// So it reads the right-hand side instead: whatever a site sets the event
	// to, it must be the constant, and then one pin covers all of them.
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	assign := regexp.MustCompile(`(?:Output\.Event\s*=|"hookEventName"\s*:)\s*([^,\n}]+)`)
	scanned, sites := 0, 0
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		b, rerr := os.ReadFile(name)
		if rerr != nil {
			t.Fatal(rerr)
		}
		scanned++
		for _, m := range assign.FindAllStringSubmatch(string(b), -1) {
			rhs := strings.TrimSpace(m[1])
			// The struct tag `json:"hookEventName"` is a declaration, not a
			// site; it has no right-hand side of its own.
			if rhs == "" || strings.HasPrefix(rhs, "string") {
				continue
			}
			sites++
			if rhs != "preToolUseEvent" {
				t.Errorf("%s sets the hook event to %s; it must be preToolUseEvent. A value the "+
					"runtime does not recognise is discarded without error, so the guard emits "+
					"nothing, exits 0 and is indistinguishable from one that had nothing to say.",
					name, rhs)
			}
		}
	}
	if scanned == 0 || sites == 0 {
		t.Fatalf("scanned %d non-test file(s) and found %d event site(s) — this check would pass "+
			"vacuously; the assignment shape it matches has changed", scanned, sites)
	}
}

// TestCiteHook_TheContextOutputCarriesTheEvent is one emitter, with its own arm.
func TestCiteHook_TheContextOutputCarriesTheEvent(t *testing.T) {
	root := citeGitRepo(t)
	body := filepath.Join(root, "b.md")
	if err := os.WriteFile(body,
		[]byte("upstream abandons it at `runtime/exec.go:1845`.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out string
	runCiteHookRaw(t, root, "gh pr create --base dev --body-file "+body, &out)
	if !strings.Contains(out, `"hookEventName":"PreToolUse"`) {
		t.Errorf("the context output does not carry the event name the runtime dispatches on.\ngot: %s", out)
	}
}

// TestCiteHook_TheDenyOutputCarriesTheEvent is the other, and it must be its own
// test for the same reason.
func TestCiteHook_TheDenyOutputCarriesTheEvent(t *testing.T) {
	root := citeGitRepo(t)
	body := filepath.Join(root, "b.md")
	if err := os.WriteFile(body, []byte("see `internal/a.go:1` for it.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out string
	runCiteHookRaw(t, root, "gh pr create --base dev --body-file "+body, &out)
	if !strings.Contains(out, `"hookEventName":"PreToolUse"`) {
		t.Errorf("the deny output does not carry the event name.\ngot: %s", out)
	}
}

// TestUnresolvedLine_NamesTheLikelyCauseCorrectly guards the wording against the
// measurement that corrected it.
//
// Across 1,670 real bodies in this estate, roughly two thirds of the citations
// this check cannot open are paths written without the prefix the checkout needs
// — the body's OWN repository, cited by bare basename — and only about a sixth
// are genuinely another repository's. The message used to name the minority as
// "the usual reason" and call it "not a defect", so the hook spoke and then
// talked the session out of acting on the majority case.
func TestUnresolvedLine_NamesTheLikelyCauseCorrectly(t *testing.T) {
	line := unresolvedLine([]cite.Citation{{Raw: "quotes.go:12", Path: "quotes.go", Line: 12}}, "")
	if !strings.Contains(line, "open them") {
		t.Error("the line does not tell the reader to open the citations, which is the one " +
			"instruction that is right in both cases")
	}
	if strings.Contains(line, "a cross-repo path is the usual reason") {
		t.Error("the line still names the minority case as usual; two thirds of these are a " +
			"same-repo path missing its prefix, and calling that 'not a defect' is wrong")
	}
}
