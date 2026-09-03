// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package awareness

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/LetA-Tech/mellions-coxen/internal/durable"
)

// Said is what this session has already been told.
//
// The whole value of an unasked-for note is that it arrives at the moment it
// matters. Repeating it every turn destroys that: a session learns the lines
// are furniture and stops reading them, and the one that mattered goes past
// with the rest. Worse, the repair anybody reaches for is to turn the hook off,
// which takes the honest notes with it.
//
// So a fact is delivered once per session and then held. Not once per process —
// a hook is a new process every time it fires, which is exactly why this is on
// disk rather than in memory.
type Said struct {
	// Path is the file for one session. Empty disables the memory entirely,
	// which makes every note fresh; that is the right behaviour for a caller
	// that has no session to key on, and it is stated rather than silent.
	Path string
}

// SaidPath is where one session's memory lives.
//
// Keyed on the session rather than the clock. The hook fires on startup, resume,
// clear and compact, so one conversation is several starts and a time-keyed
// name would make every one of them a new session with nothing said yet.
func SaidPath(root, runtime, session string) string {
	if root == "" || session == "" {
		return ""
	}
	name := sanitize(runtime) + "-" + sanitize(session) + ".said"
	return filepath.Join(root, "awareness", name)
}

var notID = regexp.MustCompile(`[^a-z0-9]+`)

func sanitize(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return "unknown"
	}
	return notID.ReplaceAllString(s, "-")
}

// Fresh is the notes this session has not been told yet.
//
// Reading is separate from remembering on purpose. A note recorded before it
// reached the session is a note lost forever, and the failure is invisible: the
// session is never told and the file says it was.
func (s Said) Fresh(notes []Note) []Note {
	if s.Path == "" {
		return notes
	}
	seen := s.load()
	var out []Note
	for _, n := range notes {
		if !seen[n.Key()] {
			out = append(out, n)
		}
	}
	return out
}

// Remember records what was delivered. Called after the notes have reached the
// session, never before.
func (s Said) Remember(notes []Note) error {
	if s.Path == "" || len(notes) == 0 {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(s.Path), 0o755); err != nil {
		return err
	}
	// Guarded because two hooks in one session can fire at once — a compact and
	// a prompt, or two panes of the same conversation — and a lost-update here
	// means a note repeats forever, which is the exact failure this file exists
	// to prevent.
	return durable.Guard(s.Path, func() error {
		seen := s.load()
		var b strings.Builder
		for _, n := range notes {
			if seen[n.Key()] {
				continue
			}
			seen[n.Key()] = true
			b.WriteString(n.Key())
			b.WriteString("\n")
		}
		if b.Len() == 0 {
			return nil
		}
		f, err := os.OpenFile(s.Path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			return err
		}
		defer f.Close()
		if _, err := f.WriteString(b.String()); err != nil {
			return err
		}
		return f.Sync()
	})
}

func (s Said) load() map[string]bool {
	seen := map[string]bool{}
	f, err := os.Open(s.Path)
	if err != nil {
		return seen
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if line := strings.TrimSpace(sc.Text()); line != "" {
			seen[line] = true
		}
	}
	return seen
}

// Prune removes the memory of sessions that ended long enough ago that a note
// is worth saying again. Best effort: a session file left behind costs a few
// bytes, and failing a hook over it would be the wrong trade entirely.
func Prune(root string, olderThan time.Duration, now time.Time) {
	pruneSuffix(root, ".said", olderThan, now)
}

func pruneSuffix(root, suffix string, olderThan time.Duration, now time.Time) {
	dir := filepath.Join(root, "awareness")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), suffix) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if now.Sub(info.ModTime()) > olderThan {
			path := filepath.Join(dir, e.Name())
			os.Remove(path)
			// The interprocess lock beside it, which nothing else removes and
			// which outlives by years the record it was guarding.
			os.Remove(path + ".lock")
		}
	}
}
