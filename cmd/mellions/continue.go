// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/LetA-Tech/mellions-coxen/internal/assignment"
	"github.com/LetA-Tech/mellions-coxen/internal/continuity"
	"github.com/LetA-Tech/mellions-coxen/internal/presence"
)

// liveHolder is the running session holding this lane, if one is.
//
// Every session the assignment records is checked, not only the most recent: a
// dead session that touched the lane last does not stop a live one that touched
// it earlier from still being in it.
func liveHolder(a *assignment.Assignment, held map[string]presence.Session) (presence.Session, bool) {
	for _, s := range a.Sessions {
		if p, ok := held[s.ID]; ok {
			return p, true
		}
	}
	return presence.Session{}, false
}

// heldNow indexes the sessions whose runtime process is running right now, by
// the session id an assignment records, so a lane can be told from the record
// of one that has ended.
//
// This session's own record is dropped. It registers before the brief is
// rendered, so a session left in would read its own lane as a peer's and stand
// off from the work it was opened to do.
func heldNow(live []presence.Session, me string, mine int) map[string]presence.Session {
	held := make(map[string]presence.Session, len(live))
	for _, s := range live {
		if isSelf(s, me, mine) {
			continue
		}
		held[s.ID] = s
	}
	return held
}

// `mellions continue` is the entry point for a session that did not attend the
// one before it. It stores nothing and remembers nothing: every fact is read at
// the moment it is asked, from the record for what was intended and from the
// world for what is true.
func cmdContinue(ctx context.Context, args []string) error {
	fs := newFlagSet("continue", flag.ContinueOnError)
	cfgPath := fs.String("config", "", "config file")
	offline := fs.Bool("offline", false, "do not ask the tracker; local evidence only")
	brief := fs.Bool("brief", false, "one line per open assignment, for a session start")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg, err := loadConfig(*cfgPath)
	if err != nil {
		return err
	}
	as, err := assignment.NewStore(cfg.assignmentsRoot())
	if err != nil {
		return err
	}
	if *brief {
		sess, cwd := hookContext(os.Stdin)
		if cwd == "" {
			cwd, _ = os.Getwd()
		}
		_, me := presence.Here()
		if me == "" {
			me = sess
		}
		return continueBrief(as, repoOf(cwd), heldNow(cfg.presences().Live(), me, presence.SelfPID()))
	}
	tr := continuity.Tracker{}
	if !*offline {
		tr = ghTracker(ctx, cfg)
	}
	s, err := continuity.Assemble(ctx, as, gitOutput, tr)
	if err != nil {
		return err
	}
	if *offline {
		s.Notes = append(s.Notes, "Run offline: the tracker was not asked. Everything about "+
			"pull requests and issues is unknown here, not absent.")
	}
	fmt.Print(s.Text())
	return nil
}

// continueBrief is the session-start form: what is open, one line each, with
// how to reach the session that last worked it. Establishing what is true
// costs a git read per worktree and a call to GitHub, and paying that on
// every session start is how a hook becomes the thing people disable.
//
// Lanes on the repository the session is standing in come first: a session on
// one repository that reads three lanes on another before its own is reading
// somebody else's desk.
func continueBrief(as *assignment.Store, repo string, held map[string]presence.Session) error {
	open, damaged, err := as.ListWithDamage(false)
	if err != nil {
		return err
	}
	if len(open) == 0 && len(damaged) == 0 {
		return nil
	}
	if repo != "" {
		sort.SliceStable(open, func(i, j int) bool {
			return open[i].Repo == repo && open[j].Repo != repo
		})
	}
	now := time.Now()
	const most = 8
	if len(open) > most {
		fmt.Printf("%d assignments are open; the %d most relevant are below and `mellions assign list` has the rest.\n\n", len(open), most)
	}
	for i, a := range open {
		if i == most {
			break
		}
		where := ""
		if repo != "" && a.Repo != repo {
			where = ", another repository"
		}
		fmt.Printf("- %s (%s%s, %s ago) %s\n", a.ID, a.State, where, humanAge(now.Sub(a.UpdatedAt)), firstLineOf(a.Objective))
		fmt.Printf("    %s on %s", a.Repo, a.Branch)
		// The worktree is the one line here a session acts on directly — it is
		// an address, not a description — and nothing updates it when a lane
		// moves or finishes. One stat is cheaper than sending a session to a
		// directory that is not there and letting it work that out from the
		// error, so a path that is gone is reported gone rather than printed as
		// somewhere to go. Where the work went instead is not guessed at: the
		// branch may be gone too, and `mellions continue` establishes both.
		if a.Worktree != "" {
			if _, err := os.Stat(a.Worktree); err != nil {
				fmt.Printf(" — the worktree it names, %s, is not there", a.Worktree)
			} else {
				fmt.Printf(" in %s", a.Worktree)
			}
		}
		fmt.Println()
		// A lane whose session is still running is not work to pick up, and the
		// difference is invisible in the line above: `active, 0m ago` reads the
		// same whether the holder died an hour ago or is mid-edit. Two shifts
		// launched seconds apart were both handed one lane this way, and the
		// second was invited to `claude --resume` a session that was alive.
		if p, ok := liveHolder(a, held); ok {
			fmt.Printf("    HELD RIGHT NOW by a live %s session %s (pid %d) in %s — not yours to take.\n",
				p.Runtime, shortSHA(p.ID), p.PID, p.Cwd)
			fmt.Printf("    Choose other work. If two lanes on this really are wanted, `mellions assign open ... -alongside` says so deliberately; `mellions who` shows the peer.\n")
		} else if s, ok := a.Latest(); ok && s.Resume() != "" {
			fmt.Printf("    last worked in %s session %s — `%s` if it still opens\n",
				s.Runtime, shortSHA(s.ID), s.Resume())
		}
		if a.Handoff != "" {
			fmt.Printf("    handoff: %s\n", clip(firstLineOf(a.Handoff), 240))
		} else if n := len(a.Findings); n > 0 {
			f := a.Findings[n-1]
			fmt.Printf("    last note (%s): %s\n", f.Kind, clip(firstLineOf(f.Text), 240))
		}
	}
	for _, id := range damaged {
		fmt.Printf("- %s: the record exists and could not be read; its branch and commits are untouched\n", id)
	}
	return nil
}

func clip(s string, n int) string {
	if len(s) > n {
		return s[:n] + " …"
	}
	return s
}

// gitOutput runs git for the reconciliation.
func gitOutput(dir string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	return cmd.Output()
}

// ghTracker asks GitHub what it says about a branch and an issue. Every
// failure returns known=false: a tracker that cannot be reached must never
// render as "there is no pull request".
func ghTracker(ctx context.Context, cfg *Config) continuity.Tracker {
	if cfg.Owner == "" {
		return continuity.Tracker{}
	}
	type answer struct {
		state string
		known bool
	}
	cache := map[string]answer{}
	ask := func(key string, args ...string) (string, bool) {
		if v, ok := cache[key]; ok {
			return v.state, v.known
		}
		c, cancel := context.WithTimeout(ctx, 15*time.Second)
		defer cancel()
		out, err := exec.CommandContext(c, "gh", args...).Output()
		v := answer{state: strings.TrimSpace(string(out)), known: err == nil}
		cache[key] = v
		return v.state, v.known
	}
	repoOf := func(repo string) string {
		if strings.Contains(repo, "/") {
			return repo
		}
		return cfg.Owner + "/" + repo
	}
	return continuity.Tracker{
		PullRequest: func(_ context.Context, repo, branch string) (string, bool) {
			state, known := ask("pr:"+repo+":"+branch, "pr", "list", "--repo", repoOf(repo),
				"--head", branch, "--state", "all", "--json", "number,state,isDraft",
				"--template", `{{range .}}#{{.number}} {{.state}}{{if .isDraft}} (draft){{end}}{{end}}`)
			if !known {
				return "", false
			}
			if state == "" {
				return "none for this branch", true
			}
			return state, true
		},
		Issue: func(_ context.Context, repo, ref string) (string, bool) {
			n := strings.TrimPrefix(ref, "#")
			return ask("issue:"+repo+":"+n, "issue", "view", n,
				"--repo", repoOf(repo), "--json", "state", "--jq", ".state")
		},
	}
}
