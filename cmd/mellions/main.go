// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

// Command mellions is the small deterministic surface under the engineer.
//
// Everything here is something a session cannot see for itself or should not
// reconstruct by hand: what needs attention across an estate, what it was in
// the middle of before a break, who else is working where, and the program and
// partnership it carries. Judgment lives in the model, and none of it is here.
//
// It is usable standalone — a terminal, a Codex session, a timer, CI — so the
// Claude Code plugin is a convenience over it rather than the only way in.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"syscall"

	"github.com/LetA-Tech/mellions-coxen/internal/checkout"
)

// Version and Commit are set at build time.
var (
	Version = "dev"
	Commit  = "unknown"
)

const usage = `mellions — the second engineer's deterministic surface

  mellions survey [-repos a,b] [-sources x,y] [-since 168h] [-kind k,l] [-full] [-json] [-save]
        What needs attention across the estate: open work, changes under
        review, failing checks, work waiting on the owner, recent change, and
        stale premises — issues whose own account of the code no longer matches
        the tree. Collected, never ranked: deciding what matters is yours.

  mellions assign open <id> -repo R [-issue "#N"] -objective "..." -because "..."
                      [-not-chosen "..."] [-branch b] [-base ref] [-worktree dir]
                      [-budget 4h] [-alongside]
  mellions assign open <id>          # an id that already has a record: claim it
  mellions assign list [-all] | get <id> | record <id> <text> [-kind found|hypothesis|next|note]
  mellions assign handoff <id> [-file f|-] | reopen <id> | close <id> | abandon <id> -discarding "..."
  mellions assign sweep [-repo R] [-apply]
        Every verb here takes the id as the argument or as -id, and means the
        same by both, so a spelling learned on one verb is never wrong on the
        next. What follows -id is the verb's own text, never a second id.
        One piece of work: a worktree, a branch, and where it stands. The record
        lives outside every target repository. Open takes work up whether or not
        it has been opened before: an id with no record needs -repo, -objective
        and -because, and an id that has one is claimed from the record, so
        taking back work that was handed off asks for none of them again. An
        issue a lane here holds — one not yet closed or abandoned, handed off
        included — is refused, naming that lane; -alongside takes it anyway,
        which reconciling two lanes needs. A lane on an issue publishes a
        mellions:claimed label and a comment naming it and its host, so a lane
        another machine holds is refused too, and one unrestated for 24h is
        swept rather than obeyed. A claim that cannot be published refuses the
        open; -unpublished accepts a lane no other machine can see. Where a
        repository records its work somewhere other than the tracker, name that
        path under work_registers in the configuration: a work unit there is
        then a real -issue rather than a reference gh cannot address, the local
        collision check holds for it, and every lane opened on that repository
        is told where the rows are.
        -worktree adopts a tree a repository's own process created instead of
        cutting one; an adopted tree and its branch are never removed. Handoff
        says where it stands; reopen takes handed-off or blocked work back up,
        re-cutting the worktree if it is gone; close removes the worktree and keeps the
        branch, and refuses while anything exists only in the worktree; abandon
        says what is thrown away and deletes the branch. Sweep reads every open
        lane against the tracker, one line each: a handed-off lane whose pull
        request is merged or closed is closable, and -apply closes it the
        ordinary way with the record saying so; an open pull request, none, a
        tracker that could not answer or a worktree holding work keeps it and
        says why; active work is never closed.

  mellions continue [-offline] [-brief]
        The slate for a session that did not attend the one before it: what an
        earlier session recorded, what the world says now, never merged. It
        reaches no conclusion — deciding what survived is the work. -brief is
        the one line per open lane the session-start block prints; a lane whose
        worktree is gone says so rather than naming it as somewhere to go.

` + renewUsage + `
  mellions who [-all] [-C dir]
        Who else is working on this repository: live sessions that registered,
        and every working tree git knows about. Absence is not an empty tree,
        and liveness is a process check, so this is this machine.

  mellions skills [<what you are doing>]
        What methods this installation carries and what each is for — the
        toolbox, not the tools. Named alone it lists every one; with words it
        narrows to those that name your situation, and a single match prints
        the whole description and the Skill call that loads it. It loads
        nothing: whether a method applies is yours to decide, and this costs
        no context until you ask.

  mellions program discover | show | list | check | adopt -by "<name>"
  mellions partner establish [<name-or-email>] | show | list | check | adopt -by "<name>"
        What the work is for, and who it is carried with. Discovered from the
        estate and drafted by the session; every section carries its provenance
        (DISCOVERED, DECLARED, INFERRED, UNKNOWN). What a person declared is
        theirs: propose a change, never rewrite it.

  mellions report write [-id id] -did "..." [-established "..."] [-blocked "..."]
                        [-next "..."] [-needs-owner "..."] [-file f|-]
  mellions report latest [-n 3]
  mellions report digest [-brief]
        What the owner reads instead of the session. Derived from the record,
        never authoritative over it. -file reads the body from a document
        instead of the flags; the flags are the only thing that says what a
        report carries, so one written that way is told nothing about whether
        it needs the owner. Digest is what needs the owner since it was last
        said — finished shifts, reports that stopped on them, lanes handed off
        to them; -brief is the session-start hook's form, said once per eight
        hours across the sessions on this host.

  mellions away [-until 8h|22:30|<stamp>] [-because "..."]
  mellions back
        Unattended is a state you enter and leave, not how a session was
        started. away records that nobody is reachable on this host: every
        session is told at its next turn, and the runner starts shifts back to
        back. back records that you are here, tells the sessions, stops the
        runner, and prints what happened while you were away. -until lapses the
        away state on its own.

  mellions pr-body-check
        Reads a PreToolUse payload on stdin and denies a gh pr create or gh pr
        edit whose body declares that it closes an issue into a base branch
        GitHub will not resolve it on. Silence otherwise, including where the
        default branch cannot be read. hooks/pr-reference.sh is what calls it.

  mellions pr-merge-check
        Reads a PreToolUse payload on stdin and denies a gh pr merge whose
        state cannot support the decision: mergeability GitHub still reports as
        UNKNOWN, or a branch behind its base where the base has since changed a
        file the pull request also changes. Being behind alone is not refused.
        Silence otherwise, including where the tracker cannot answer.
        hooks/pr-merge-check.sh is what calls it, and MELLIONS_MERGE_CHECK=off
        silences it.

  mellions shared-tree-check
        Reads a PreToolUse payload on stdin and denies a Bash call that runs a
        tree-mutating git command inside a checkout every lane on this host is
        cut from, rather than in a worktree of the session's own. Silence
        otherwise, including where the directory cannot be resolved.
        hooks/shared-tree.sh is what calls it.
  mellions cite check [-file f|-] [-dir checkout] [-commit sha]
        Reports the path:line citations a document makes that the checkout does
        not back — a line the file does not have, and a line that exists and
        says something the document quotes nowhere. Existence is not the
        predicate: every citation this was built for resolved and named the
        wrong line. Ranges, paths the checkout does not hold, and path:line
        inside a fenced block or blockquote are quotation or another
        repository's, and are silent. Exit 1 where there are findings.

  mellions cite-check
        The same check as a PreToolUse decision: reads a payload on stdin and
        denies a gh pr or gh issue create, edit, comment or review whose body
        publishes a citation the checkout does not back. Silence otherwise.
        hooks/citation-check.sh is what calls it, and MELLIONS_CITE_CHECK=off
        silences it.

  mellions secret check [-path <file>] <command...>
        Report what in a command would read a credential-bearing file's
        content onto stdout. Metadata (wc, stat, ls, chmod), a capture into a
        variable and a source of the file are silent; a bare path handed to
        anything else is not, because a transcript is sent as it is written
        and the fix afterwards is a rotation. Exit 1 where there are findings.

  mellions secret-check
        The same check as a PreToolUse decision: reads a payload on stdin and
        denies a Bash, Read, Grep or NotebookRead call that would put a
        credential in the transcript. Silence otherwise.
        hooks/secret-check.sh is what calls it, and MELLIONS_SECRET_CHECK=off
        — in the session environment, not as an inline prefix — silences it.

  mellions config init | show | path | home | reports
        home and reports print one directory each and nothing else: where this
        host's shifts land, and where a report is written. scripts/shift.sh
        asks rather than defaulting to a path of its own.
  mellions doctor
  mellions install [-from <path|owner/repo>] [-runtime claude|codex] [-dry-run]
  mellions version

Configuration is JSON at -config, $MELLIONS_CONFIG, ./mellions.json, or
~/.mellions/config.json. ` + "`mellions config init`" + ` writes one.
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var err error
	switch os.Args[1] {
	case "survey":
		err = cmdSurvey(ctx, os.Args[2:])
	case "sources":
		err = cmdSources(os.Args[2:])
	case "assign":
		err = cmdAssign(ctx, os.Args[2:])
	case "continue", "where":
		err = cmdContinue(ctx, os.Args[2:])
	case "renew":
		err = cmdRenew(os.Args[2:])
	case "who", "territory":
		err = cmdWho(os.Args[2:])
	case "here":
		err = cmdHere(os.Args[2:])
	case "state":
		err = cmdState(os.Args[2:])
	case "away":
		err = cmdAway(os.Args[2:])
	case "back":
		err = cmdBack(os.Args[2:])
	case "skills", "skill":
		err = cmdSkills(ctx, os.Args[2:])
	case "program":
		err = cmdProgram(ctx, os.Args[2:])
	case "partner":
		err = cmdPartner(ctx, os.Args[2:])
	case "report":
		err = cmdReport(os.Args[2:])
	case "pr-body-check":
		err = cmdPRBodyCheck(ctx, os.Args[2:])
	case "pr-merge-check":
		err = cmdPRMergeCheck(ctx, os.Args[2:])
	case "shared-tree-check":
		err = cmdSharedTreeCheck(os.Args[2:])
	case "cite":
		err = cmdCite(ctx, os.Args[2:])
	case "cite-check":
		err = cmdCiteCheck(ctx, os.Args[2:])
	case "secret":
		err = cmdSecret(os.Args[2:])
	case "secret-check":
		err = cmdSecretCheck(os.Args[2:])
	case "config":
		err = cmdConfig(os.Args[2:])
	case "doctor", "inspect":
		err = cmdDoctor(ctx, os.Args[2:])
	case "install":
		err = cmdInstall(ctx, os.Args[2:])
	case "version":
		fmt.Printf("%s (%s)\n", Version, Commit)
	case "-h", "--help", "help":
		fmt.Print(usage)
	default:
		fmt.Fprintf(os.Stderr, "mellions: unknown command %q\n\n%s", os.Args[1], usage)
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "mellions: %v\n", err)
		os.Exit(1)
	}
}

// Config is the installation's description of itself: which repositories, where
// their checkouts are, which tracker, and where the engineer's own records
// live. None of it is a property of the engineer, which is why another
// installation points this at a different estate without a code change.
type Config struct {
	// Owner is the tracker org or user.
	Owner string `json:"owner"`
	// Repos is what the engineer is responsible for.
	Repos []string `json:"repos"`
	// WorkRoot holds repository checkouts; WorkRoots where there are several.
	WorkRoot  string   `json:"work_root"`
	WorkRoots []string `json:"work_roots,omitempty"`
	// CheckoutAt names where a repository is when its directory is not named
	// after it.
	CheckoutAt map[string]string `json:"checkouts,omitempty"`
	// Sources names which survey sources run, in order.
	Sources []string `json:"sources"`
	// ProgramsDir and PartnersDir hold one file per program and per person.
	ProgramsDir string `json:"programs_dir"`
	PartnersDir string `json:"partners_dir"`
	// OwnerLabels mark work waiting on the owner.
	OwnerLabels []string `json:"owner_labels"`
	// StaleMinAgeHours skips re-checking issue bodies younger than this.
	StaleMinAgeHours int `json:"stale_min_age_hours"`
	// PerRepoLimit caps items of each type per repository.
	PerRepoLimit int `json:"per_repo_limit"`
	// AssignmentsRoot holds per-assignment state, outside every target repository.
	AssignmentsRoot string `json:"assignments_root"`
	// WorkRegisters names, per repository, the path inside it where that
	// repository records its own work — rows, decisions, open remediations —
	// for the repositories whose register is not the tracker. A lane on a work
	// unit there is a real claim rather than a reference gh cannot address, and
	// every lane opened on such a repository is told where the rows are.
	WorkRegisters map[string]string `json:"work_registers,omitempty"`
	// ReportRoot holds reports, the saved survey and session registrations.
	ReportRoot string `json:"report_root"`
	// GitSinceHours bounds how far back recent-change reporting looks.
	GitSinceHours int `json:"git_since_hours"`

	path string
}

// runtimeKeys are settings that belong to the coding-agent runtime, with where
// each one actually lives. Mellions is an overlay: permissions, tools, hooks,
// MCP, sandboxing, credentials and model settings are inherited unchanged, and
// a copy here would be read by nobody while looking authoritative.
var runtimeKeys = map[string]string{
	"allowed_tools": "the runtime's own permission settings", "allowedTools": "the runtime's own permission settings",
	"tools": "the runtime's own tool configuration", "shell_allowlist": "the runtime's permission settings",
	"permissions": "the runtime's permission settings", "permission_mode": "the runtime's permission settings",
	"permissionMode": "the runtime's permission settings", "approval_policy": "Codex's own configuration",
	"approvalPolicy": "Codex's own configuration", "hooks": "hooks/hooks.json in the plugin",
	"mcp": "the runtime's MCP configuration", "mcp_servers": "the runtime's MCP configuration",
	"mcpServers": "the runtime's MCP configuration", "sandbox": "the runtime's sandbox",
	"sandbox_mode": "the runtime's sandbox", "model": "the runtime's model selection",
	"model_settings": "the runtime's model configuration", "max_tokens": "the runtime's model configuration",
	"credentials": "the environment, and never a file the engineer can read",
	"api_key":     "the environment, and never a file the engineer can read",
	"skills":      "the skills/ directory in the plugin",
}

// checkOverlay refuses configuration that belongs to the runtime.
func checkOverlay(raw []byte, path string) error {
	var keys map[string]json.RawMessage
	if err := json.Unmarshal(raw, &keys); err != nil {
		return nil // the real parse reports it better
	}
	var found []string
	for k := range keys {
		if where, ok := runtimeKeys[k]; ok {
			found = append(found, fmt.Sprintf("  %q belongs to %s", k, where))
		}
	}
	if len(found) == 0 {
		return nil
	}
	sort.Strings(found)
	return fmt.Errorf("%s configures the runtime, not Mellions:\n\n%s\n\n"+
		"Mellions is an overlay. The runtime owns permissions, tools, hooks, MCP, sandboxing, "+
		"credentials and model settings, and all of it is inherited unchanged.",
		path, strings.Join(found, "\n"))
}

var errNoConfig = errors.New("no config found")

func configCandidates(explicit string) []string {
	c := []string{explicit, os.Getenv("MELLIONS_CONFIG"), "mellions.json"}
	if home, err := os.UserHomeDir(); err == nil {
		c = append(c, filepath.Join(home, ".mellions", "config.json"))
	}
	return c
}

func loadConfig(explicit string) (*Config, error) {
	candidates := configCandidates(explicit)
	for _, p := range candidates {
		if p == "" {
			continue
		}
		raw, err := os.ReadFile(p)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("read config %s: %w", p, err)
		}
		if err := checkOverlay(raw, p); err != nil {
			return nil, err
		}
		var c Config
		if err := json.Unmarshal(raw, &c); err != nil {
			return nil, fmt.Errorf("parse config %s: %w", p, err)
		}
		c.path = p
		if len(c.Sources) == 0 {
			c.Sources = []string{"programs", "assignments", "github", "git", "stale"}
		}
		return &c, nil
	}
	return nil, fmt.Errorf("%w. Write one:\n\n  mellions config init\n\nLooked for %s",
		errNoConfig, strings.Join(nonEmpty(candidates), ", "))
}

// workRegisters is the configured map, keyed lower-case so a repository named
// with different case in the configuration and on the command line is one
// repository.
func (c *Config) workRegisters() map[string]string {
	if len(c.WorkRegisters) == 0 {
		return nil
	}
	out := make(map[string]string, len(c.WorkRegisters))
	for repo, path := range c.WorkRegisters {
		repo, path = strings.TrimSpace(repo), strings.TrimSpace(path)
		if repo != "" && path != "" {
			out[strings.ToLower(repo)] = path
		}
	}
	return out
}

func (c *Config) assignmentsRoot() string {
	if c.AssignmentsRoot != "" {
		return c.AssignmentsRoot
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, "mellions", "assignments")
	}
	return "mellions-assignments"
}

// reportRoot is where everything that is not an assignment lives: reports, the
// saved survey, session registrations, and what each session was already told.
func (c *Config) reportRoot() string {
	if c.ReportRoot != "" {
		return c.ReportRoot
	}
	return filepath.Dir(c.assignmentsRoot())
}

// home is the state directory a host's shifts, runner and digest marker share:
// $MELLIONS_HOME where it is set, else the report root.
//
// This is the only place the answer is computed. The shift script and the
// runner ask for it (`mellions config home`) rather than defaulting to
// $HOME/mellions on their own: where the two differ — an installation whose
// report_root is not under $HOME/mellions — a script with its own default
// writes a second, configless home beside the real one, and the shifts in it
// are invisible to `report digest`, to `doctor` and to the script's own report
// backstop.
func (c *Config) home() string {
	if h := strings.TrimSpace(os.Getenv("MELLIONS_HOME")); h != "" {
		return h
	}
	return c.reportRoot()
}

func (c *Config) programsDir() string {
	if c.ProgramsDir != "" {
		return c.ProgramsDir
	}
	return filepath.Join(c.reportRoot(), "programs")
}

func (c *Config) partnersDir() string {
	if c.PartnersDir != "" {
		return c.PartnersDir
	}
	return filepath.Join(c.reportRoot(), "partners")
}

// roots is every parent directory holding checkouts, in search order.
func (c *Config) roots() []string {
	var out []string
	if c.WorkRoot != "" {
		out = append(out, c.WorkRoot)
	}
	for _, r := range c.WorkRoots {
		if r != "" && !slices.Contains(out, r) {
			out = append(out, r)
		}
	}
	return out
}

func (c *Config) checkouts() checkout.Set {
	return checkout.Resolve(c.roots(), c.CheckoutAt, c.Repos)
}

// checkout resolves a repository name to its local checkout.
//
// An explicit "checkouts" entry answers on its own, without the repository also
// being in "repos". The two say different things: "repos" is the scope the
// engineer is responsible for, and "checkouts" is where a repository is. Naming
// a location is how this installation reaches a checkout it works in but does
// not survey — its own source among them — and requiring the scope list as well
// would make that impossible without widening what every source collects.
func (c *Config) checkout(repo string) (string, error) {
	if len(c.roots()) == 0 && len(c.CheckoutAt) == 0 {
		return "", errors.New("no \"work_root\" or \"work_roots\" in config, so there is no checkout to work in")
	}
	if repo == "" {
		return "", errors.New("no repository named (-repo)")
	}
	if dir := c.CheckoutAt[repo]; dir != "" {
		if !checkout.IsCheckout(dir) {
			return "", fmt.Errorf("%q names %s for %s, which is not a git checkout",
				"checkouts", dir, repo)
		}
		return dir, nil
	}
	if dir, ok := c.checkouts().Dir(repo); ok {
		return dir, nil
	}
	return "", fmt.Errorf("no checkout of %s under %s.\n\n"+
		"Add its parent to \"work_roots\", or name where it is with\n"+
		"  \"checkouts\": {%q: \"/path/to/it\"}",
		repo, strings.Join(c.roots(), ", "), repo)
}

// hookTranscript is the transcript path the runtime named in the hook payload,
// when it did; the glob under the projects directory is the fallback.
// hookCommand is the Bash command line a PreToolUse payload carries, which
// says where the call reaches — cwd says only where the session stands, and
// the two differ every time a command line begins with `cd`.
var hookTranscript, hookCommand string

// hookContext reads what the runtime sent a hook on stdin, where it sent
// anything. Never an error: a payload it cannot read means it knows less.
func hookContext(r io.Reader) (session, cwd string) {
	if f, ok := r.(*os.File); ok {
		info, err := f.Stat()
		if err != nil || info.Mode()&os.ModeCharDevice != 0 {
			return "", ""
		}
	}
	raw, err := io.ReadAll(io.LimitReader(r, 1<<20))
	if err != nil || len(raw) == 0 {
		return "", ""
	}
	var ev struct {
		Session    string `json:"session_id"`
		Cwd        string `json:"cwd"`
		Transcript string `json:"transcript_path"`
		ToolName   string `json:"tool_name"`
		Input      struct {
			Command string `json:"command"`
		} `json:"tool_input"`
	}
	if err := json.Unmarshal(raw, &ev); err != nil {
		return "", ""
	}
	hookTranscript = strings.TrimSpace(ev.Transcript)
	if ev.ToolName == "Bash" {
		hookCommand = ev.Input.Command
	}
	return strings.TrimSpace(ev.Session), strings.TrimSpace(ev.Cwd)
}

func readAll(r io.Reader) ([]byte, error) { return io.ReadAll(io.LimitReader(r, 8<<20)) }

func splitList(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func nonEmpty(in []string) []string {
	var out []string
	for _, s := range in {
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

func firstLine(s string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(s), "\n")
	return line
}
