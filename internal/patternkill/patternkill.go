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
// or pattern, or "": pkill or killall without a pid-derived selection, and kill
// handed pids by pgrep or pidof, as `$( )`, backquotes or `| xargs kill`.
//
// It reads what a Bash tool call types: a kill inside a string (`bash -c`,
// `trap`), a loop over pgrep's output and a pid list in a variable are not seen.
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
			if !pidScoped(args) {
				return show(words)
			}
		case "killall":
			if !inert(args) {
				return show(words)
			}
		case "kill":
			if fedByName(args) {
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
	return "`" + command + "` signals processes chosen by name or pattern, not by a pid.\n\n" +
		"Every session on this host runs as the same user, so the match reaches other sessions' processes — " +
		"their test runs, gates and servers — and with -f it also matches the shell issuing it, whose command line carries the pattern.\n\n" +
		"Signal the pid this turn started: keep `$!` after `&` (or use the Bash tool's run_in_background and stop it by its id), " +
		"`kill <pid>`, `pkill -P <pid>` for its children, then `kill -0 <pid>` to establish it is gone. " +
		"To find a pid, `pgrep -af <pattern>` lists the matches without signalling them."
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
	shortValued = "dgGPstuUFOr"
	pidShort    = "PsgF"
)

var pidLong = map[string]bool{"parent": true, "session": true, "pgroup": true, "pidfile": true}

// pidScoped reports that pkill or pgrep arguments carry a selection derived
// from a pid. Every criterion must match, so one such criterion bounds the
// whole selection to that pid's family. A value of 0 names the issuing
// command's own session or group, and 1 the init process every orphan is
// reparented to: neither bounds anything.
func pidScoped(args []string) bool {
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			return false
		}
		if !strings.HasPrefix(a, "-") || a == "-" {
			continue
		}
		if strings.HasPrefix(a, "--") {
			name, val, has := strings.Cut(a[2:], "=")
			if !pidLong[name] {
				continue
			}
			if !has && i+1 < len(args) {
				i++
				val = args[i]
			}
			if bounding(val) {
				return true
			}
			continue
		}
		cluster := a[1:]
		if signal(cluster) {
			continue
		}
		for j := 0; j < len(cluster); j++ {
			if !strings.ContainsRune(shortValued, rune(cluster[j])) {
				continue
			}
			val := cluster[j+1:]
			if val == "" && i+1 < len(args) {
				i++
				val = args[i]
			}
			if strings.ContainsRune(pidShort, rune(cluster[j])) && bounding(val) {
				return true
			}
			break
		}
	}
	return false
}

func bounding(val string) bool {
	return val != "" && val != "0" && val != "1"
}

// signal reports a signal given as an option: `-9`, `-KILL`, `-SIGTERM`.
func signal(cluster string) bool {
	if cluster == "" {
		return false
	}
	digits, upper := true, len(cluster) >= 2
	for _, r := range cluster {
		if r < '0' || r > '9' {
			digits = false
		}
		if (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != '+' && r != '-' {
			upper = false
		}
	}
	return digits || upper
}

// inert reports killall arguments that list or version and signal nothing.
func inert(args []string) bool {
	for _, a := range args {
		switch a {
		case "-l", "--list", "-V", "--version":
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
