// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

// Package patternkill decides whether a Bash command signals processes chosen
// by name or command-line pattern instead of by a pid.
//
// Every session on a host runs as one user, so a name or a pattern matches
// other sessions' processes as readily as this one's, and `pkill -f X` also
// matches the shell issuing it, whose own command line carries X. A pid the
// turn recorded names one process; a pattern names whatever matches when it
// runs. Selection derived from a pid — `pkill -P <pid>`, a session, a process
// group, a pidfile — stays bounded to that pid's family and is not refused.
package patternkill

import (
	"path/filepath"
	"strings"

	"github.com/LetA-Tech/mellions-coxen/internal/shellsplit"
)

// Find returns the command in command that signals processes chosen by name
// or pattern, or "": pkill or killall without a pid-derived selection, kill
// handed pids by pgrep or pidof, as `$( )`, backquotes or `| xargs kill`, and
// kill of pid -1, which is every process the user may signal.
//
// It reads what a Bash tool call types: a kill inside a string (`bash -c`,
// `trap`), a loop over pgrep's output, a pid list in a variable and a lookup
// more than one pipeline stage before xargs are not seen.
func Find(command string) string {
	cmds := shellsplit.Split(command)
	for i, c := range cmds {
		words := shellsplit.Program(c.Words)
		if len(words) == 0 {
			continue
		}
		args := uncommented(words[1:])
		switch filepath.Base(words[0]) {
		case "pkill":
			if !inert(args) && !pidScoped(args) {
				return show(words)
			}
		case "killall":
			if !inert(args) {
				return show(words)
			}
		case "kill":
			if fedByName(args) || everyProcess(args) {
				return show(words)
			}
		case "xargs":
			if i > 0 && runsKill(args) && lookup(shellsplit.Program(cmds[i-1].Words)) {
				return show(shellsplit.Program(cmds[i-1].Words)) + " | " + show(words)
			}
		}
	}
	return ""
}

// Reason is what the session is told when Find names a command.
func Reason(command string) string {
	return "`" + command + "` signals processes chosen by name, pattern or -1, not by a pid.\n\n" +
		"Every session on this host runs as the same user, so the selection reaches other sessions' processes — " +
		"their test runs, gates and servers — and with -f it also matches the shell issuing it, whose command line carries the pattern.\n\n" +
		"Signal the pid you started. Each Bash tool call is a new shell, so write it down where it is made " +
		"(`cmd & echo $! > run.pid`), then `pkill -F run.pid`, `kill \"$(cat run.pid)\"`, or `pkill -P <pid>` for its children; " +
		"a job started with the Bash tool's run_in_background is stopped by its task id (TaskStop). " +
		"It is gone when `timeout 10 tail --pid=<pid> -f /dev/null` returns and `kill -0 <pid>` fails. " +
		"`pgrep -af <pattern>` lists matches without signalling them; confirm a pid is yours " +
		"(`ps -o pid,ppid,lstart,args -p <pid>`) before signalling it."
}

func show(words []string) string {
	return strings.Join(words, " ")
}

// uncommented drops an unquoted comment and what follows it.
func uncommented(args []string) []string {
	for i, a := range args {
		if strings.HasPrefix(a, "#") {
			return args[:i]
		}
	}
	return args
}

// shortValued are pkill and pgrep's short options whose value follows the
// letter or is the next word; pidShort are those whose value is a pid, a
// session, a process group or a pidfile.
const (
	shortValued = "dgGPqstuUFOr"
	pidShort    = "PsgF"
)

// longs are pkill and pgrep's long options, which getopt also accepts by any
// unique prefix (`--par` is `--parent`).
var longs = []string{
	"delimiter", "list-name", "list-full", "inverse", "lightweight", "ignore-case", "count",
	"full", "pgroup", "group", "ignore-ancestors", "newest", "oldest", "older", "parent",
	"session", "signal", "terminal", "euid", "uid", "exact", "pidfile", "logpidfile",
	"runstates", "ns", "nslist", "cgroup", "env", "echo", "queue", "require-handler",
	"help", "version",
}

var pidLong = map[string]bool{"parent": true, "session": true, "pgroup": true, "pidfile": true}

// long resolves a long option name, abbreviated or not, to the option it
// names, or "" when it names none or more than one.
func long(name string) string {
	found := ""
	for _, l := range longs {
		if l == name {
			return l
		}
		if name != "" && strings.HasPrefix(l, name) {
			if found != "" {
				return ""
			}
			found = l
		}
	}
	return found
}

// pidScoped reports that pkill or pgrep arguments carry a selection derived
// from a pid. Every criterion must match, so one such criterion bounds the
// whole selection to that pid's family, unless the selection is inverted
// (`--inverse`, pgrep's `-v`), which turns it into every other process.
func pidScoped(args []string) bool {
	scoped := false
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			break
		}
		if !strings.HasPrefix(a, "-") || a == "-" {
			continue
		}
		if strings.HasPrefix(a, "--") {
			name, val, has := strings.Cut(a[2:], "=")
			switch opt := long(name); {
			case opt == "inverse":
				return false
			case pidLong[opt]:
				if !has && i+1 < len(args) {
					i++
					val = args[i]
				}
				if bounding(opt, val) {
					scoped = true
				}
			}
			continue
		}
		cluster := a[1:]
		if signal(cluster) {
			continue
		}
		for j := 0; j < len(cluster); j++ {
			l := cluster[j]
			if l == 'v' {
				return false
			}
			if !strings.ContainsRune(shortValued, rune(l)) {
				continue
			}
			val := cluster[j+1:]
			if val == "" && i+1 < len(args) {
				i++
				val = args[i]
			}
			if strings.ContainsRune(pidShort, rune(l)) && bounding(string(l), val) {
				scoped = true
			}
			break
		}
	}
	return scoped
}

// bounding reports that every element of a comma-separated selection value
// names a specific family. 0 for a session or group is the caller's own, and
// in a Bash tool call so is $$: the shell issuing the command, which the
// selection then takes. 1 is init, parent of every orphan on the host.
func bounding(opt, val string) bool {
	if val == "" {
		return false
	}
	own := opt == "s" || opt == "g" || opt == "session" || opt == "pgroup"
	for _, v := range strings.Split(val, ",") {
		if v == "" || v == "0" || v == "1" || (own && (v == "$$" || v == "${$}")) {
			return false
		}
	}
	return true
}

// signals are the signal names kill(1) and pkill accept, without "SIG".
var signals = map[string]bool{
	"HUP": true, "INT": true, "QUIT": true, "ILL": true, "TRAP": true, "ABRT": true, "IOT": true,
	"BUS": true, "FPE": true, "KILL": true, "USR1": true, "SEGV": true, "USR2": true, "PIPE": true,
	"ALRM": true, "TERM": true, "STKFLT": true, "CHLD": true, "CLD": true, "CONT": true, "STOP": true,
	"TSTP": true, "TTIN": true, "TTOU": true, "URG": true, "XCPU": true, "XFSZ": true, "VTALRM": true,
	"PROF": true, "WINCH": true, "POLL": true, "IO": true, "PWR": true, "SYS": true, "UNUSED": true,
	"NULL": true, "RTMIN": true, "RTMAX": true,
}

// signal reports a signal given as an option, which pkill reads before any
// other option and in any case: `-9`, `-KILL`, `-sigterm`, `-stop`,
// `-RTMIN+1`. `-P1234` is a parent pid and `-s4321` a session.
func signal(cluster string) bool {
	if cluster == "" {
		return false
	}
	if strings.Trim(cluster, "0123456789") == "" {
		return true
	}
	name := strings.TrimPrefix(strings.ToUpper(cluster), "SIG")
	if i := strings.IndexAny(name, "+-"); i > 0 && strings.HasPrefix(name, "RT") {
		name = name[:i]
	}
	return signals[name]
}

// everyProcess reports kill arguments that target pid -1: `-1` after a signal
// option or after `--` (`kill -9 -1`, `kill -- -1`). As the first argument
// `-1` is itself a signal, SIGHUP.
func everyProcess(args []string) bool {
	signalled, ended := false, false
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case ended || signalled:
			if a == "-1" {
				return true
			}
			if a == "--" {
				ended = true
			}
		case a == "--":
			ended = true
		case a == "-s" || a == "-n":
			i++
			signalled = true
		case strings.HasPrefix(a, "-"):
			signalled = true
		}
	}
	return false
}

// inert reports pkill or killall arguments that list, help or version and
// signal nothing.
func inert(args []string) bool {
	for _, a := range args {
		switch a {
		case "-l", "--list", "-V", "--version", "-h", "--help":
		default:
			return false
		}
	}
	return true
}

// lookup reports a command that prints pids chosen by name or pattern.
func lookup(words []string) bool {
	if len(words) == 0 {
		return false
	}
	switch filepath.Base(words[0]) {
	case "pidof":
		return true
	case "pgrep":
		return !pidScoped(uncommented(words[1:]))
	}
	return false
}

// fedByName reports a kill operand that is the output of a lookup, as `$( )`
// or backquotes.
func fedByName(args []string) bool {
	for i, a := range args {
		var inner string
		switch {
		case strings.HasPrefix(a, "$(") && !strings.HasPrefix(a, "$(("):
			inner = strings.TrimSuffix(a[2:], ")")
		case strings.HasPrefix(a, "`"):
			inner = strings.TrimPrefix(strings.Join(args[i:], " "), "`")
			if j := strings.IndexByte(inner, '`'); j >= 0 {
				inner = inner[:j]
			}
		default:
			continue
		}
		for _, c := range shellsplit.Split(inner) {
			if lookup(shellsplit.Program(c.Words)) {
				return true
			}
		}
	}
	return false
}

// xargsValued are xargs's short options whose value is the next word when not
// attached.
var xargsValued = map[string]bool{"-n": true, "-I": true, "-L": true, "-P": true, "-d": true, "-E": true, "-s": true, "-a": true}

// runsKill reports xargs arguments whose command is kill.
func runsKill(args []string) bool {
	for i := 0; i < len(args); i++ {
		a := args[i]
		if xargsValued[a] {
			i++
			continue
		}
		if strings.HasPrefix(a, "-") {
			continue
		}
		return filepath.Base(a) == "kill"
	}
	return false
}
