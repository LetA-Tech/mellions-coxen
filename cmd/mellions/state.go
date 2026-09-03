// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/LetA-Tech/mellions-coxen/internal/assignment"
	"github.com/LetA-Tech/mellions-coxen/internal/awareness"
	"github.com/LetA-Tech/mellions-coxen/internal/pluginreg"
	"github.com/LetA-Tech/mellions-coxen/internal/presence"
	"github.com/LetA-Tech/mellions-coxen/internal/sharedtree"
)

// cmdState answers what is true about this session's situation right now that
// it cannot infer and would act on. Written to be run by a hook on every turn:
// silent when there is nothing to say, each thing said once per session, never
// blocking, and a failure anywhere inside it prints nothing.
func cmdState(args []string) error {
	fs := newFlagSet("state", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	cfgPath := fs.String("config", "", "config file")
	dir := fs.String("C", "", "read from this directory instead of the working one")
	session := fs.String("session", "", "runtime session id, so a fact is said once")
	runtime := fs.String("runtime", "", "which runtime is asking")
	repeat := fs.Bool("repeat", false, "say everything, including what was already said")
	tool := fs.Bool("tool", false, "emit the runtime's PreToolUse JSON form, so the note reaches a session mid-turn")
	if err := fs.Parse(args); err != nil {
		return err
	}
	sess, cwd := hookContext(os.Stdin)
	if *session == "" {
		*session = sess
	}
	// The awareness hooks pipe no payload, so the id comes from what the runtime
	// exports, as it does for `here`. Without it a session cannot recognise its
	// own record, and the heartbeat that says it is still working never runs.
	if *session == "" {
		_, *session = presence.Here()
	}
	if *runtime == "" {
		*runtime = "claude"
	}
	here := firstNonEmpty(*dir, cwd)
	if here == "" {
		var err error
		if here, err = os.Getwd(); err != nil {
			return nil
		}
	}
	cfg, err := loadConfig(*cfgPath)
	if err != nil {
		return nil
	}

	o := observe(cfg, here, *session, *runtime)

	// Every prompt and tool call is proof the session is still working; the
	// record says so, and `who` can tell a working session from an idle one. It
	// also says where: a session registers from wherever the runtime started
	// it, and one that has since moved into another repository would otherwise
	// be looked for in the tree it left. A lane it holds is not displaced —
	// that is what the session works in, whatever directory it stands in.
	if *session != "" {
		_ = cfg.presences().Working(*session, presence.Work{
			Tree: o.Tree, Repo: o.Repo, Branch: o.Branch,
		}, time.Now())
	}
	notes := awareness.Notes(o)
	// A method is carried only if it is read, and the catalog that names them
	// is delivered once, at session start, against work that has not happened
	// yet. These name one Skill at the instant its situation arrives — the
	// command that publishes, closes, files or destroys — and the memory below
	// says each once, so a moment reached ten times costs one line.
	notes = append(notes, awareness.MomentNotes(hookCommand)...)
	said := awareness.Said{Path: awareness.SaidPath(cfg.reportRoot(), *runtime, sessionKey(*session, here))}
	if !*repeat {
		notes = said.Fresh(notes)
	}
	clock := clockLine(time.Now(), os.Getenv("MELLIONS_DEADLINE"))
	if len(notes) == 0 && clock == "" {
		return nil
	}
	if *tool {
		out, err := json.Marshal(map[string]any{"hookSpecificOutput": map[string]string{
			"hookEventName": "PreToolUse", "additionalContext": stateText(notes, clock)}})
		if err != nil {
			return nil
		}
		fmt.Println(string(out))
	} else {
		fmt.Print(stateText(notes, clock))
	}
	if !*repeat {
		_ = said.Remember(notes)
	}
	awareness.Prune(cfg.reportRoot(), 7*24*time.Hour, time.Now())
	awareness.PruneDelivered(cfg.reportRoot(), 7*24*time.Hour, time.Now())
	return nil
}

// observe establishes the situation. Every failure is swallowed: this runs on
// the way to the model's turn and a broken observation must cost silence.
func observe(cfg *Config, here, me, runtime string) awareness.Observation {
	mine := presence.SelfPID()
	live := cfg.presences().Live()
	here, _ = workingIn(live, me, mine, treeOf(here))
	o := awareness.Observation{Tree: here, Repo: repoOf(here)}
	o.Branch = strings.TrimSpace(gitOut(here, "rev-parse", "--abbrev-ref", "HEAD"))
	if o.Repo != "" && here != "" {
		if src, err := cfg.checkout(o.Repo); err == nil && sameTree(src, here) {
			o.Source = true
			o.Tracking = strings.TrimSpace(gitOut(here,
				"rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}")) != ""
		}
	}
	lane := laneFinder(cfg)
	o.Lane = lane(o.Repo, me, here)

	// Where the session stands is not necessarily where a compound command
	// writes, so awareness also resolves checkouts reached within the command.
	if hookCommand != "" {
		if c := sharedtree.Reach(hookCommand, here, sharedEstate(cfg)); c != nil {
			o.Reaching, o.ReachingRepo = c.Dir, c.Repo
			o.ReachingLane = lane(c.Repo, me, here)
		}
	}
	o.Command = hookCommand
	o.Documents = governingDocuments(cfg, runtime, me)

	// A marker that will not parse is silence here, as every other failure in
	// this function is: the session is told nothing rather than told wrongly,
	// and `mellions away` and `mellions back` say so where a person is reading.
	if s, err := readOwner(cfg.ownerMarker()); err == nil {
		o.Owner = ownerPresence(s, time.Now())
	}

	for _, s := range live {
		if isSelf(s, me, mine) {
			continue
		}
		peer := awareness.Peer{Describe: s.Describe(), Resume: s.Resume()}
		switch {
		case sameTree(s.Cwd, here):
			o.Others = append(o.Others, peer)
		case o.Repo != "" && s.Repo == o.Repo:
			o.Elsewhere = append(o.Elsewhere, peer)
		}
	}

	// The session's transcript: the path the hook payload names, else the one
	// under the runtime's projects directory. Its model decides which
	// compactions it is measured against.
	transcript := hookTranscript
	if transcript == "" && me != "" {
		if m, _ := filepath.Glob(filepath.Join(home(), ".claude", "projects", "*", me+".jsonl")); len(m) > 0 {
			sort.Strings(m)
			transcript = m[0]
		}
	}
	if transcript != "" {
		o.CompactAt, o.CompactSamples = pluginreg.CompactionSize(home(), pluginreg.TranscriptModel(transcript))
	}
	o.RenewAt = renewAt(o.CompactAt)
	if transcript != "" && o.RenewAt > 0 {
		// The stat is asked every call; the scan only once the file is big
		// enough to matter.
		if fi, err := os.Stat(transcript); err == nil && fi.Size() >= o.RenewAt {
			if n, ok := pluginreg.ContextSinceIn(transcript); ok {
				o.ContextBytes = n
			}
		}
	}
	o.Idle = nothingInFlight(cfg)
	if o.Idle {
		if brief, age, ok := surveyBrief(cfg.surveyPath()); ok && age < 2*time.Hour {
			o.Survey, o.SurveyBrief = cfg.surveyPath(), brief
		}
	}
	return o
}

func nothingInFlight(cfg *Config) bool {
	store, err := assignment.NewStore(cfg.assignmentsRoot())
	if err != nil {
		return false
	}
	open, err := store.List(false)
	if err != nil {
		return false
	}
	for _, a := range open {
		if a.State == assignment.StateActive || a.State == assignment.StateBlocked || a.State == assignment.StateSuspended {
			return false
		}
	}
	return true
}

// sessionKey is what a fact is remembered against: the session id where there
// is one, otherwise the working tree.
func sessionKey(session, tree string) string {
	if session != "" {
		return session
	}
	if tree == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(tree))
	return "tree-" + hex.EncodeToString(sum[:6])
}

func stateText(notes []awareness.Note, clock string) string {
	var b strings.Builder
	b.WriteString("<mellions-state>\n")
	for i, n := range notes {
		if i > 0 {
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "%s\n", n.Because)
		if n.Why != "" {
			fmt.Fprintf(&b, "%s\n", n.Why)
		}
		fmt.Fprintf(&b, "  %s\n", n.Do)
	}
	if clock != "" {
		if len(notes) > 0 {
			b.WriteString("\n")
		}
		b.WriteString(clock + "\n")
	}
	b.WriteString("</mellions-state>\n")
	return b.String()
}

// clockLine is the one thing said on every call rather than once: a session
// has no clock of its own, and one that estimates elapsed time drifts several
// times over within minutes, then narrows its scope and skips verification to
// fit a deadline that has not arrived. MELLIONS_DEADLINE is the deadline as a
// Unix time, set by the shift script; unset, or unparseable, means no deadline
// and nothing is said.
func clockLine(now time.Time, deadline string) string {
	secs, err := strconv.ParseInt(strings.TrimSpace(deadline), 10, 64)
	if err != nil || secs <= 0 {
		return ""
	}
	at := time.Unix(secs, 0).UTC()
	now = now.UTC()
	left := at.Sub(now).Round(time.Minute)
	if left < 0 {
		return fmt.Sprintf("clock: %s UTC · the deadline %s UTC passed %d min ago — write where the work stands",
			now.Format("15:04"), at.Format("15:04"), int(-left.Minutes()))
	}
	return fmt.Sprintf("clock: %s UTC · deadline %s UTC · %d min left", now.Format("15:04"), at.Format("15:04"), int(left.Minutes()))
}

// meHere is this session's own registration, if any, for the session-start hook
// to show alongside the peers.
func meHere() string {
	_, id := presence.Here()
	return id
}

// renewAt is the transcript size, since the last compaction, past which a
// session is told where it stands: six tenths of the size this host's runtime
// has compacted at on its own, so the session hears it with room to finish the
// piece in hand; 3 MB where no compaction has been observed yet.
// MELLIONS_RENEW_BYTES overrides; 0 turns the note off.
func renewAt(compactAt int64) int64 {
	if v := strings.TrimSpace(os.Getenv("MELLIONS_RENEW_BYTES")); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil || n < 0 {
			return 0
		}
		return n
	}
	if compactAt > 0 {
		return compactAt * 6 / 10
	}
	return 3_000_000
}
