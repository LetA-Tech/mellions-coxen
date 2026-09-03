package main

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/LetA-Tech/mellions-coxen/internal/awareness"
)

// The whole path the defect lives on: the session-start hook renders a
// governing document, the owner edits it hours later, and the next prompt is
// what has to carry the fact. Everything here goes through the same entry
// points the hooks call.

const partnershipV1 = `# Partnership: alex
adopted: 2026-08-28 by the operator

## What is delegated to me, and what stays yours {DECLARED}

Merging is yours to propose and mine to approve.

## Rhythm {DISCOVERED}

Commits cluster between 09:00 and 11:00.
`

const partnershipV2 = `# Partnership: alex
adopted: 2026-08-28 by the operator

## What is delegated to me, and what stays yours {DECLARED}

Merge authority is granted for work you have established is ready.

## Rhythm {DISCOVERED}

Commits cluster between 09:00 and 11:00.
`

const programV1 = `# Program: sample-platform
adopted: 2026-08-28 by the operator

## Correctness {DECLARED}

A wrong posting is the expensive one.
`

func estate(t *testing.T) (root string) {
	t.Helper()
	root = t.TempDir()
	for _, d := range []string{"partners", "programs", "assignments"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	write(t, filepath.Join(root, "partners", "alex.md"), partnershipV1)
	write(t, filepath.Join(root, "programs", "sample-platform.md"), programV1)
	cfg, err := json.Marshal(map[string]any{
		"owner": "example-org", "repos": []string{}, "report_root": root,
		"assignments_root": filepath.Join(root, "assignments"),
	})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "config.json")
	write(t, path, string(cfg))
	t.Setenv("MELLIONS_CONFIG", path)
	t.Setenv("MELLIONS_RUNTIME", "claude")
	t.Setenv("MELLIONS_HOOK", "1")
	return root
}

// held stands the session's turns apart in time: it backdates what the session
// has seen so far, so the next turn is a turn taken after the settle window
// rather than a test that sleeps through one.
//
// It moves the record, never the mechanism — the clock the mechanism reads is
// the real one, and TestTheSettleWindowIsRealTime waits it out.
func held(t *testing.T, root, session string) {
	t.Helper()
	path := awareness.DeliveredPath(root, "claude", session)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("no record to hold: %v", err)
	}
	var rec struct {
		Delivered map[string]json.RawMessage `json:"delivered,omitempty"`
		Seen      map[string]struct {
			State string    `json:"state"`
			First time.Time `json:"first"`
			Told  string    `json:"told,omitempty"`
		} `json:"seen,omitempty"`
	}
	if err := json.Unmarshal(raw, &rec); err != nil {
		t.Fatal(err)
	}
	if len(rec.Seen) == 0 {
		t.Fatalf("nothing was held for confirmation, so there is nothing to settle: %s", raw)
	}
	for k, s := range rec.Seen {
		s.First = s.First.Add(-time.Hour)
		rec.Seen[k] = s
	}
	out, err := json.Marshal(rec)
	if err != nil {
		t.Fatal(err)
	}
	write(t, path, string(out))

	// And the writer finished an hour ago too: a file touched inside the window
	// is one somebody may still be writing, whatever two readings agreed on.
	past := time.Now().Add(-time.Hour)
	for _, dir := range []string{"partners", "programs"} {
		entries, err := os.ReadDir(filepath.Join(root, dir))
		if err != nil {
			continue
		}
		for _, e := range entries {
			if err := os.Chtimes(filepath.Join(root, dir, e.Name()), past, past); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// asSession runs f the way a hook does: the runtime's payload on stdin, output
// captured rather than printed.
func asSession(t *testing.T, session string, f func() error) string {
	t.Helper()
	payload := filepath.Join(t.TempDir(), "payload.json")
	write(t, payload, `{"session_id":"`+session+`","cwd":"`+t.TempDir()+`"}`)
	in, err := os.Open(payload)
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	stdin, stdout := os.Stdin, os.Stdout
	os.Stdin, os.Stdout = in, w
	done := make(chan string, 1)
	go func() {
		out, _ := io.ReadAll(r)
		done <- string(out)
	}()
	err = f()
	w.Close()
	os.Stdin, os.Stdout = stdin, stdout
	out := <-done
	r.Close()
	if err != nil {
		t.Fatalf("command failed: %v\n%s", err, out)
	}
	return out
}

func TestAnEditAfterTheSessionStartedReachesTheNextPrompt(t *testing.T) {
	root := estate(t)
	const session = "sess-partnership-drift"

	// Session start: the hooks render both documents.
	if out := asSession(t, session, func() error { return partnerShow([]string{"-brief", "alex"}) }); !strings.Contains(out, "Partnership") {
		t.Fatalf("the partnership was not rendered: %q", out)
	}
	asSession(t, session, func() error { return programShow([]string{"-brief", "sample-platform"}) })

	// Nothing has changed, so nothing is said about either document.
	quiet := asSession(t, session, func() error { return cmdState([]string{"-session", session, "-C", root}) })
	if strings.Contains(quiet, "changed after it reached this session") {
		t.Fatalf("a document nobody touched was reported changed: %q", quiet)
	}

	// Hours later the owner writes the grant into the partnership. The first
	// reading of it is a sample of a moment and says nothing.
	write(t, filepath.Join(root, "partners", "alex.md"), partnershipV2)
	fresh := asSession(t, session, func() error { return cmdState([]string{"-session", session, "-C", root}) })
	if strings.Contains(fresh, "partnership with alex") {
		t.Fatalf("spoke on a single unconfirmed reading:\n%s", fresh)
	}
	held(t, root, session)

	told := asSession(t, session, func() error { return cmdState([]string{"-session", session, "-C", root}) })
	for _, want := range []string{
		"partnership with alex changed after it reached this session",
		`"What is delegated to me, and what stays yours" {DECLARED}`,
		"mellions partner show alex",
	} {
		if !strings.Contains(told, want) {
			t.Errorf("the session was never told %q:\n%s", want, told)
		}
	}
	if strings.Contains(told, "Merge authority is granted") {
		t.Errorf("the note carried the changed text instead of pointing at the document:\n%s", told)
	}
	if strings.Contains(told, "program sample-platform changed") {
		t.Errorf("an untouched program was reported changed:\n%s", told)
	}

	// Said once: the next prompt is silent about it.
	again := asSession(t, session, func() error { return cmdState([]string{"-session", session, "-C", root}) })
	if strings.Contains(again, "partnership with alex") {
		t.Errorf("the same change was repeated on the next turn:\n%s", again)
	}

	// A second edit is a second fact, and is delivered.
	write(t, filepath.Join(root, "partners", "alex.md"),
		strings.Replace(partnershipV2, "you have established is ready", "you have proven ready", 1))
	asSession(t, session, func() error { return cmdState([]string{"-session", session, "-C", root}) })
	held(t, root, session)
	third := asSession(t, session, func() error { return cmdState([]string{"-session", session, "-C", root}) })
	if !strings.Contains(third, "partnership with alex changed after it reached this session") {
		t.Errorf("a second change never reached the session:\n%s", third)
	}

	// A hook rendering the document again — a resume, a compact — re-baselines
	// it, so a later prompt says nothing more. The same command run from a
	// terminal carries no session and re-baselines nothing; the memory of what
	// was said is what stops the repeat there.
	asSession(t, session, func() error { return partnerShow([]string{"alex"}) })
	settled := asSession(t, session, func() error { return cmdState([]string{"-session", session, "-C", root}) })
	if strings.Contains(settled, "partnership with alex") {
		t.Errorf("reading the document did not settle the note:\n%s", settled)
	}
}

func TestASessionWithNoRecordedDeliveryIsToldNothing(t *testing.T) {
	root := estate(t)
	write(t, filepath.Join(root, "partners", "alex.md"), partnershipV2)
	out := asSession(t, "sess-never-handed-anything", func() error {
		return cmdState([]string{"-session", "sess-never-handed-anything", "-C", root})
	})
	if strings.Contains(out, "changed after it reached this session") {
		t.Fatalf("a change was claimed against a baseline that was never recorded:\n%s", out)
	}
}

func TestOneSessionsBaselineIsNotAnother(t *testing.T) {
	root := estate(t)
	asSession(t, "sess-a", func() error { return partnerShow([]string{"-brief", "alex"}) })
	write(t, filepath.Join(root, "partners", "alex.md"), partnershipV2)
	asSession(t, "sess-b", func() error { return partnerShow([]string{"-brief", "alex"}) })

	asSession(t, "sess-a", func() error { return cmdState([]string{"-session", "sess-a", "-C", root}) })
	held(t, root, "sess-a")
	a := asSession(t, "sess-a", func() error { return cmdState([]string{"-session", "sess-a", "-C", root}) })
	if !strings.Contains(a, "partnership with alex changed") {
		t.Errorf("the session that holds the old version was not told:\n%s", a)
	}
	b := asSession(t, "sess-b", func() error { return cmdState([]string{"-session", "sess-b", "-C", root}) })
	if strings.Contains(b, "partnership with alex changed") {
		t.Errorf("a session handed the current version was told it had changed:\n%s", b)
	}
}

func TestAPartnershipHalfWrittenIsNotAChange(t *testing.T) {
	root := estate(t)
	const session = "sess-mid-write"
	asSession(t, session, func() error { return partnerShow([]string{"-brief", "alex"}) })

	// What a hook firing during an editor's save sees.
	write(t, filepath.Join(root, "partners", "alex.md"), "# Partnership: alex\n\n## Half a heading\n")
	mid := asSession(t, session, func() error { return cmdState([]string{"-session", session, "-C", root}) })
	if strings.Contains(mid, "partnership with alex") {
		t.Errorf("an unparseable document was reported on:\n%s", mid)
	}

	// And the change is still delivered once the file parses again.
	write(t, filepath.Join(root, "partners", "alex.md"), partnershipV2)
	asSession(t, session, func() error { return cmdState([]string{"-session", session, "-C", root}) })
	held(t, root, session)
	after := asSession(t, session, func() error { return cmdState([]string{"-session", session, "-C", root}) })
	if !strings.Contains(after, "partnership with alex changed") {
		t.Errorf("the change was lost by skipping the half-written read:\n%s", after)
	}
}

// TestARenameSaveThroughTheRealPathNeverSaysGone.
//
// The reviewer's reproduction, driven through the command a hook runs: 3.6 KB
// of partnership, content never changed, saved 200 times the way an editor
// saves — the original moved aside, copied back, the backup removed. Against
// an unconfirmed read this produced a note telling the session its partnership
// was gone, keyed on a constant so nothing ever retracted it.
func TestARenameSaveThroughTheRealPathNeverSaysGone(t *testing.T) {
	root := estate(t)
	const session = "sess-rename-save"
	asSession(t, session, func() error { return partnerShow([]string{"-brief", "alex"}) })

	path := filepath.Join(root, "partners", "alex.md")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for i := range 25 {
		if err := os.Rename(path, path+"~"); err != nil {
			t.Fatal(err)
		}
		// The window every rename-style save opens, sampled exactly as the hook
		// samples it — on the prompt and on every tool call.
		out := asSession(t, session, func() error { return cmdState([]string{"-session", session, "-C", root}) })
		if strings.Contains(out, "partnership") {
			t.Fatalf("save %d: spoke about a partnership that was mid-save:\n%s", i, out)
		}
		if err := os.WriteFile(path, body, 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(path + "~"); err != nil {
			t.Fatal(err)
		}
		out = asSession(t, session, func() error { return cmdState([]string{"-session", session, "-C", root}) })
		if strings.Contains(out, "partnership") {
			t.Fatalf("save %d: spoke about a partnership nobody changed:\n%s", i, out)
		}
	}
}

// TestATornRewriteThroughTheRealPathNamesNoSection.
//
// An in-place rewrite in chunks: each intermediate state parses, digests
// differently from what was delivered, and has lost every section the writer
// has not reached yet. Against an unconfirmed read every one of them was a
// distinct new fact, so saying a thing once gave no protection — four notes to
// one session, three of them naming sections nobody touched.
func TestATornRewriteThroughTheRealPathNamesNoSection(t *testing.T) {
	root := estate(t)
	const session = "sess-torn-rewrite"
	asSession(t, session, func() error { return partnerShow([]string{"-brief", "alex"}) })

	path := filepath.Join(root, "partners", "alex.md")
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < len(partnershipV2); i += 40 {
		end := min(i+40, len(partnershipV2))
		if _, err := f.WriteString(partnershipV2[i:end]); err != nil {
			t.Fatal(err)
		}
		out := asSession(t, session, func() error { return cmdState([]string{"-session", session, "-C", root}) })
		if strings.Contains(out, "partnership with alex") {
			t.Fatalf("byte %d: a half-written document was reported as the owner's change:\n%s", end, out)
		}
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	// The writer finished. The change is the finished document, said once.
	asSession(t, session, func() error { return cmdState([]string{"-session", session, "-C", root}) })
	held(t, root, session)
	told := asSession(t, session, func() error { return cmdState([]string{"-session", session, "-C", root}) })
	if !strings.Contains(told, "partnership with alex changed after it reached this session") {
		t.Fatalf("the real change never arrived:\n%s", told)
	}
	if strings.Contains(told, "{gone}") {
		t.Fatalf("the note claims the owner deleted a section:\n%s", told)
	}
}

// TestTheSettleWindowIsRealTime: the window is read off the clock the process
// runs on, not off a record a test can move. Every other end-to-end test here
// backdates the record to stay fast; this one waits.
func TestTheSettleWindowIsRealTime(t *testing.T) {
	if testing.Short() {
		t.Skip("waits out the settle window")
	}
	root := estate(t)
	const session = "sess-real-clock"
	asSession(t, session, func() error { return partnerShow([]string{"-brief", "alex"}) })
	write(t, filepath.Join(root, "partners", "alex.md"), partnershipV2)

	out := asSession(t, session, func() error { return cmdState([]string{"-session", session, "-C", root}) })
	if strings.Contains(out, "partnership with alex") {
		t.Fatalf("spoke on the first reading:\n%s", out)
	}
	time.Sleep(3200 * time.Millisecond)
	out = asSession(t, session, func() error { return cmdState([]string{"-session", session, "-C", root}) })
	if !strings.Contains(out, "partnership with alex changed after it reached this session") {
		t.Fatalf("a reading that held past the window was never confirmed:\n%s", out)
	}
}

// TestShowDoesNotConsumeStdinOutsideAHook: `partner show` and `program show`
// are run by people, scripts, cron and CI. Reading stdin to find a session id
// makes every one of those block on a pipe nobody will close, and eats the rest
// of a `while read` loop's input.
func TestShowDoesNotConsumeStdinOutsideAHook(t *testing.T) {
	root := estate(t)
	t.Setenv("MELLIONS_HOOK", "")

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if _, err := w.WriteString("sample-platform\n"); err != nil {
		t.Fatal(err)
	}
	stdin := os.Stdin
	os.Stdin = r
	done := make(chan error, 1)
	go func() { done <- partnerShow([]string{"-brief", "alex"}) }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("partner show failed: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("partner show blocked on a pipe nobody is going to close")
	}
	os.Stdin = stdin

	// Only now, so that what it did not block on was a pipe still open.
	w.Close()
	rest, err := io.ReadAll(io.LimitReader(r, 64))
	if err != nil {
		t.Fatal(err)
	}
	if string(rest) != "sample-platform\n" {
		t.Errorf("the caller's remaining input was consumed: %q", rest)
	}
	if p := awareness.DeliveredPath(root, "claude", "sess-x"); p != "" {
		if _, err := os.Stat(p); err == nil {
			t.Error("a terminal run recorded a delivery")
		}
	}
}
