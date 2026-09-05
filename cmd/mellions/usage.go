package main

import (
	"flag"
	"fmt"
)

// usageForms is where every subcommand's shape is written down, once. The flag
// package prints the flags it was given and nothing else, so a command whose
// argument is positional had no way to say so on the path a wrong flag name
// actually takes: the parse fails, the flag list prints, and the argument the
// command requires is not in it.
//
// The value is the whole invocation after "mellions ". A name mapped to a form
// without a positional still needs an entry, so adding a subcommand is a
// decision about its usage line rather than a step that can be skipped.
var usageForms = map[string]string{
	"estate read":       "estate read [flags] <path>",
	"assign open":       "assign open [flags] [<id>]",
	"assign list":       "assign list [flags]",
	"assign get":        "assign get [flags] <id>",
	"assign record":     "assign record [flags] <id> <text>",
	"assign claim":      "assign claim [flags] <id> -pr <n>",
	"assign handoff":    "assign handoff [flags] <id> [<text>]",
	"assign close":      "assign close [flags] <id>",
	"assign abandon":    "assign abandon [flags] <id>",
	"assign reopen":     "assign reopen [flags] <id>",
	"assign sweep":      "assign sweep [flags]",
	"adopt":             "partner|program adopt [flags] [<slug>]",
	"away":              "away [flags]",
	"back":              "back [flags]",
	"cite check":        "cite check [flags]",
	"cite-check":        "cite-check",
	"config":            "config [flags]",
	"continue":          "continue [flags]",
	"doctor":            "doctor [flags]",
	"here":              "here [flags]",
	"install":           "install [flags]",
	"partner check":     "partner check [flags] [<slug>]",
	"pr-body-check":     "pr-body-check",
	"pr-merge-check":    "pr-merge-check",
	"secret check":      "secret check [flags] <command...>",
	"secret-check":      "secret-check",
	"shared-tree-check": "shared-tree-check",
	"skills":            "skills [flags] [<what you are doing>]",
	"partner establish": "partner establish [flags] [<person>]",
	"partner list":      "partner list [flags]",
	"partner show":      "partner show [flags] [<slug>]",
	"program check":     "program check [flags] [<slug>]",
	"program discover":  "program discover [flags]",
	"program list":      "program list [flags]",
	"program show":      "program show [flags] [<slug>]",
	"renew":             "renew [flags]",
	"report":            "report <write|latest|digest|report> [flags] [<assignment-id>]",
	"sources":           "sources [flags]",
	"state":             "state [flags]",
	"survey":            "survey [flags]",
	"who":               "who [flags]",
}

// usageLine is the one sentence both fs.Usage and the hand-written
// missing-argument errors print, so the two cannot drift apart.
func usageLine(name string) string {
	form, ok := usageForms[name]
	if !ok {
		form = name + " [flags]"
	}
	return "usage: mellions " + form
}

// newFlagSet builds a subcommand's flag set with that usage line in front of
// the flag list. Every flag set in this command is built here, and a test holds
// that, so a subcommand added later cannot reintroduce the hole.
func newFlagSet(name string, errorHandling flag.ErrorHandling) *flag.FlagSet {
	fs := flag.NewFlagSet(name, errorHandling)
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "%s\n\n", usageLine(name))
		fs.PrintDefaults()
	}
	return fs
}
