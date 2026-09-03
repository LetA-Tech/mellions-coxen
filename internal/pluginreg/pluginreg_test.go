// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package pluginreg

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// hooksJSON is the shape of a real manifest: one hook that declares a
// statusMessage and one that does not, because the runtime records the first by
// its message and the second by its command.
const hooksJSON = `{"hooks":{"SessionStart":[{"matcher":"startup|resume|clear|compact","hooks":[
 {"type":"command","command":"bash \"${CLAUDE_PLUGIN_ROOT}/hooks/session-identity.sh\"; exit 0","statusMessage":"Loading who you are..."},
 {"type":"command","command":"bash \"${CLAUDE_PLUGIN_ROOT}/hooks/session-program.sh\"; exit 0"}
]}],"PreToolUse":[{"hooks":[{"type":"command","command":"other"}]}]}}`

// install writes a runtime home whose registry names the plugin.
func install(t *testing.T, opts ...func(*fixture)) (home string, f *fixture) {
	t.Helper()
	home = t.TempDir()
	f = &fixture{
		home:        home,
		installPath: filepath.Join(home, ".claude", "plugins", "cache", "mellions", "mellions", "0.1.0"),
		registered:  "2026-08-28T16:54:22.441Z",
		enabled:     true,
		withHooks:   true,
	}
	for _, o := range opts {
		o(f)
	}
	f.write(t)
	return home, f
}

type fixture struct {
	home        string
	installPath string
	registered  string
	enabled     bool
	withHooks   bool
	noRegistry  bool
}

func (f *fixture) write(t *testing.T) {
	t.Helper()
	mk := func(p string) { must(t, os.MkdirAll(p, 0o755)) }
	mk(filepath.Join(f.home, ".claude", "plugins"))
	if !f.noRegistry {
		reg := `{"version":2,"plugins":{"other@x":[{"installPath":"/x","version":"1"}],
			"mellions@mellions":[{"scope":"user","installPath":"` + f.installPath +
			`","version":"0.1.0","installedAt":"2026-08-27T00:00:00.000Z","lastUpdated":"` + f.registered + `"}]}}`
		must(t, os.WriteFile(filepath.Join(f.home, ".claude", "plugins", "installed_plugins.json"), []byte(reg), 0o644))
	}
	on := "false"
	if f.enabled {
		on = "true"
	}
	must(t, os.WriteFile(filepath.Join(f.home, ".claude", "settings.json"),
		[]byte(`{"enabledPlugins":{"caveman@caveman":true,"mellions@mellions":`+on+`}}`), 0o644))
	if f.withHooks {
		mk(filepath.Join(f.installPath, "hooks"))
		must(t, os.WriteFile(filepath.Join(f.installPath, "hooks", "hooks.json"), []byte(hooksJSON), 0o644))
	}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func TestReadReadyRegistration(t *testing.T) {
	home, f := install(t)
	r := Read(home, ID)
	if !r.Installed {
		t.Fatalf("registry names the plugin, Read said it does not: %+v", r)
	}
	if !r.Ready() {
		t.Fatalf("registry, settings and installed copy all agree; Ready() false: %v", r.Problems)
	}
	if r.InstallPath != f.installPath {
		t.Errorf("installPath = %q, want %q", r.InstallPath, f.installPath)
	}
	if got := r.Registered.Format(time.RFC3339); got != "2026-08-28T16:54:22Z" {
		// lastUpdated, not installedAt: a reinstall moves the first and not the
		// second, and it is the first a running process is behind.
		t.Errorf("Registered = %s, want the lastUpdated 2026-08-28T16:54:22Z", got)
	}
	want := []string{"Loading who you are...", `bash "${CLAUDE_PLUGIN_ROOT}/hooks/session-program.sh"; exit 0`}
	if len(r.Hooks) != len(want) {
		t.Fatalf("hooks = %q, want %q", r.Hooks, want)
	}
	for i := range want {
		if r.Hooks[i] != want[i] {
			t.Errorf("hook %d = %q, want %q", i, r.Hooks[i], want[i])
		}
	}
}

// An installation the runtime will not load must never read as ready. Each of
// these looks like a successful install from the installer's exit codes alone.
func TestReadNotReady(t *testing.T) {
	t.Run("not enabled", func(t *testing.T) {
		home, _ := install(t, func(f *fixture) { f.enabled = false })
		r := Read(home, ID)
		if !r.Installed {
			t.Fatal("installed and disabled is installed")
		}
		if r.Ready() {
			t.Fatal("a disabled plugin loads nothing; Ready() must be false")
		}
		if !strings.Contains(strings.Join(r.Problems, " "), "explicitly disabled") {
			t.Errorf("problems do not name the disablement: %v", r.Problems)
		}
	})
	t.Run("copy gone", func(t *testing.T) {
		home, _ := install(t, func(f *fixture) { f.withHooks = false })
		r := Read(home, ID)
		if r.Ready() {
			t.Fatal("the registry points at a copy with no hooks manifest; Ready() must be false")
		}
		if len(r.Hooks) != 0 {
			t.Errorf("hooks read from a copy that is not there: %v", r.Hooks)
		}
	})
	t.Run("registry does not name it", func(t *testing.T) {
		home, _ := install(t, func(f *fixture) { f.noRegistry = true })
		r := Read(home, ID)
		if r.Installed || r.Ready() {
			t.Fatal("no registry file means not installed")
		}
		if len(r.Problems) == 0 {
			t.Fatal("an unreadable registry is a problem stated, not silence")
		}
	})
}

// A transcript as the runtime writes one: one attachment per hook it ran, all
// sharing the SessionStart's toolUseID, plus the aggregate record that carries
// no command.
func transcript(t *testing.T, home, project, session string, lines ...string) {
	t.Helper()
	dir := filepath.Join(home, ".claude", "projects", project)
	must(t, os.MkdirAll(dir, 0o755))
	must(t, os.WriteFile(filepath.Join(dir, session+".jsonl"), []byte(strings.Join(lines, "\n")+"\n"), 0o644))
}

func hookLine(ts, kind, tool, cmd, typ string) string {
	return `{"timestamp":"` + ts + `","attachment":{"type":"` + typ + `","hookName":"SessionStart:` + kind +
		`","toolUseID":"` + tool + `","hookEvent":"SessionStart","command":` + quote(cmd) + `}}`
}

func quote(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

func TestReadLoadSessionHasThePlugin(t *testing.T) {
	home, _ := install(t)
	r := Read(home, ID)
	transcript(t, home, "-home-you-mellions", "sess-has",
		hookLine("2026-08-28T16:58:07Z", "startup", "t1", "Loading caveman mode...", "hook_success"),
		hookLine("2026-08-28T16:58:07Z", "startup", "t1", "Loading who you are...", "hook_success"),
		hookLine("2026-08-28T16:58:07Z", "startup", "t1", `bash "${CLAUDE_PLUGIN_ROOT}/hooks/session-program.sh"; exit 0`, "hook_success"),
		// the aggregate record: no command, and counting it would report a
		// hook nobody ran
		`{"timestamp":"2026-08-28T16:58:08Z","attachment":{"type":"hook_additional_context","hookName":"SessionStart","toolUseID":"SessionStart","hookEvent":"SessionStart","content":["x"]}}`,
	)
	l, ok := ReadLoad(home, "sess-has", r.Hooks)
	if !ok {
		t.Fatal("the transcript exists; ReadLoad did not find it")
	}
	if len(l.Events) != 1 {
		t.Fatalf("one SessionStart ran, got %d: %+v", len(l.Events), l.Events)
	}
	if l.Events[0].Total != 3 {
		t.Errorf("total = %d, want 3 — the aggregate record must not be counted", l.Events[0].Total)
	}
	if l.Events[0].Ours != 2 {
		t.Errorf("ours = %d, want 2", l.Events[0].Ours)
	}
	if !l.Has() {
		t.Fatal("every SessionStart included this installation's hooks; Has() must be true")
	}
}

// The defect this exists for: a process born before the registration runs
// SessionStart again on /clear and /compact and still never acquires the
// plugin. Read from the record of what the runtime actually ran.
func TestReadLoadSessionBornBeforeTheRegistration(t *testing.T) {
	home, _ := install(t)
	r := Read(home, ID)
	transcript(t, home, "-home-you-workspace-payments-api", "sess-without",
		hookLine("2026-08-28T03:58:12Z", "clear", "t1", "Loading caveman mode...", "hook_success"),
		hookLine("2026-08-28T03:58:12Z", "clear", "t1", "Loading ponytail mode...", "hook_success"),
		hookLine("2026-08-28T16:25:06Z", "compact", "t2", "Loading caveman mode...", "hook_success"),
		hookLine("2026-08-28T16:25:06Z", "compact", "t2", "Loading ponytail mode...", "hook_success"),
	)
	l, ok := ReadLoad(home, "sess-without", r.Hooks)
	if !ok {
		t.Fatal("the transcript exists; ReadLoad did not find it")
	}
	if l.Has() {
		t.Fatal("no Mellions hook ever ran for this session; Has() must be false")
	}
	if len(l.Events) != 2 {
		t.Fatalf("two SessionStarts ran, got %d", len(l.Events))
	}
	last, ok := l.Latest()
	if !ok || !last.At.Equal(time.Date(2026, 8, 28, 16, 25, 6, 0, time.UTC)) {
		t.Fatalf("Latest = %+v, want the 16:25:06 compact", last)
	}
	if !last.At.Before(r.Registered) {
		t.Fatal("the fixture's last SessionStart must precede the registration")
	}
}

// A hook the runtime ran and that failed is a hook the runtime knew about. A
// session whose only Mellions hook errored has the plugin loaded and a broken
// hook — a different fault from not having it, and the two must not merge.
func TestReadLoadCountsHooksThatFailed(t *testing.T) {
	home, _ := install(t)
	r := Read(home, ID)
	transcript(t, home, "-p", "sess-err",
		hookLine("2026-08-28T10:00:00Z", "startup", "t1", "Loading who you are...", "hook_non_blocking_error"),
		hookLine("2026-08-28T10:00:00Z", "startup", "t1", `bash "${CLAUDE_PLUGIN_ROOT}/hooks/session-program.sh"; exit 0`, "hook_success"),
	)
	l, _ := ReadLoad(home, "sess-err", r.Hooks)
	if !l.Has() {
		t.Fatal("the runtime ran this installation's hooks; a failure is not an absence")
	}
	if l.Events[0].Ours != 2 {
		t.Errorf("ours = %d, want 2", l.Events[0].Ours)
	}
}

// No transcript is unestablished, and must not read as a session that ran no
// hooks: one is "I could not look", the other is "I looked and there was
// nothing", and a session acts differently on each.
func TestReadLoadMissingTranscript(t *testing.T) {
	home, _ := install(t)
	if _, ok := ReadLoad(home, "no-such-session", []string{"x"}); ok {
		t.Fatal("no transcript must report not-found, not an empty load")
	}
	if _, ok := ReadLoad(home, "", []string{"x"}); ok {
		t.Fatal("no session id must report not-found")
	}
}

// The remedy must clear the diagnosis. `claude --resume` keeps the session id
// and appends to the same transcript, so a session that did exactly what doctor
// told it to has both the launch without the plugin and the resume with it. The
// most recent SessionStart is what it is running now.
func TestReadLoadResumeClearsTheDiagnosis(t *testing.T) {
	home, _ := install(t)
	r := Read(home, ID)
	transcript(t, home, "-p", "sess-resumed",
		hookLine("2026-08-27T19:31:29Z", "startup", "t1", "Loading caveman mode...", "hook_success"),
		hookLine("2026-08-28T18:30:27Z", "resume", "t2", "Loading who you are...", "hook_success"),
	)
	l, _ := ReadLoad(home, "sess-resumed", r.Hooks)
	if !l.Has() {
		t.Fatal("the resume ran this installation's hooks; the session has it now, " +
			"and an earlier launch that did not must not outvote the current one")
	}
}

// The converse, so Has() is not simply true: a session whose latest SessionStart
// carried none of these hooks does not have it, whatever an earlier one did.
func TestReadLoadLatestGoverns(t *testing.T) {
	home, _ := install(t)
	r := Read(home, ID)
	transcript(t, home, "-p", "sess-lost",
		hookLine("2026-08-27T21:47:16Z", "clear", "t1", "Loading who you are...", "hook_success"),
		hookLine("2026-08-28T04:47:25Z", "compact", "t2", "Loading caveman mode...", "hook_success"),
	)
	l, _ := ReadLoad(home, "sess-lost", r.Hooks)
	if l.Has() {
		t.Fatal("the latest SessionStart ran none of this installation's hooks; " +
			"a session that lost them is not a session that has them")
	}
}

// An installed plugin with no enabledPlugins key is enabled: the runtime turns
// one off only when something says so. Reading an absent key as disabled makes
// a working install report as broken and fails `make install`.
func TestEnabledByDefaultWhenNothingSaysOtherwise(t *testing.T) {
	home := t.TempDir()
	f := &fixture{
		home:        home,
		installPath: filepath.Join(home, ".claude", "plugins", "cache", "mellions", "mellions", "0.1.0"),
		registered:  "2026-08-28T16:54:22.441Z",
		enabled:     true,
		withHooks:   true,
	}
	f.write(t)
	// Every other key kept; only this plugin's is absent.
	must(t, os.WriteFile(filepath.Join(home, ".claude", "settings.json"),
		[]byte(`{"enabledPlugins":{"caveman@caveman":true}}`), 0o644))
	r := Read(home, ID)
	if !r.Enabled {
		t.Fatal("an absent enabledPlugins key is not a disabled plugin")
	}
	if !r.Ready() {
		t.Fatalf("nothing disables it and the copy is complete; Ready() false: %v", r.Problems)
	}
	if r.EnabledIn != "" {
		t.Errorf("EnabledIn = %q, want empty — no file decided", r.EnabledIn)
	}
}

// CLAUDE_CONFIG_DIR moves the runtime's whole configuration. Reading ~/.claude
// anyway reads a file the runtime does not use.
func TestReadHonoursConfigDirOverride(t *testing.T) {
	home, f := install(t)
	elsewhere := t.TempDir()
	must(t, os.MkdirAll(filepath.Join(elsewhere, "plugins"), 0o755))
	must(t, os.Rename(filepath.Join(home, ".claude", "plugins", "installed_plugins.json"),
		filepath.Join(elsewhere, "plugins", "installed_plugins.json")))
	must(t, os.Rename(filepath.Join(home, ".claude", "settings.json"),
		filepath.Join(elsewhere, "settings.json")))
	if r := Read(home, ID); r.Installed {
		t.Fatal("the registry was moved away; reading ~/.claude must not find it")
	}
	t.Setenv("CLAUDE_CONFIG_DIR", elsewhere)
	r := Read(home, ID)
	if !r.Installed || r.InstallPath != f.installPath {
		t.Fatalf("CLAUDE_CONFIG_DIR not honoured: %+v", r)
	}
}
