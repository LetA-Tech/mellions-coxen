// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package main

import (
	"flag"
	"reflect"
	"testing"
	"time"
)

// TestArgumentsParseInAnyOrder.
//
// Every command in this binary mixes an id with flags, and people write them in
// whichever order the sentence takes. Go's flag package stops at the first
// non-flag argument, so one order worked and the rest silently produced a
// different command: the id landing in the flag list, or `-config` never being
// read and the answer coming from another installation entirely.
func TestArgumentsParseInAnyOrder(t *testing.T) {
	build := func() (*flag.FlagSet, *string, *string, *bool, *time.Duration) {
		fs := flag.NewFlagSet("t", flag.ContinueOnError)
		fs.SetOutput(discard{})
		cfg := fs.String("config", "", "")
		by := fs.String("by", "", "")
		force := fs.Bool("open", false, "")
		ttl := fs.Duration("for", 0, "")
		return fs, cfg, by, force, ttl
	}

	for _, tc := range []struct {
		name string
		args []string
		want []string
		cfg  string
		by   string
		open bool
	}{
		{"id then flags", []string{"d-1", "-by", "Ada", "-config", "/c"}, []string{"d-1"}, "/c", "Ada", false},
		{"flags then id", []string{"-by", "Ada", "-config", "/c", "d-1"}, []string{"d-1"}, "/c", "Ada", false},
		{"flag, id, flag", []string{"-by", "Ada", "d-1", "-config", "/c"}, []string{"d-1"}, "/c", "Ada", false},
		{"bool between", []string{"-open", "d-1", "-config", "/c"}, []string{"d-1"}, "/c", "", true},
		{"equals form", []string{"-by=Ada", "d-1", "-config=/c"}, []string{"d-1"}, "/c", "Ada", false},
		{"two positionals split", []string{"a", "-by", "Ada", "b"}, []string{"a", "b"}, "", "Ada", false},
		{"a lone dash is a value", []string{"d-1", "-by", "-"}, []string{"d-1"}, "", "-", false},
		{"double dash ends flags", []string{"-by", "Ada", "--", "-not-a-flag"}, []string{"-not-a-flag"}, "", "Ada", false},
		{"no arguments", nil, nil, "", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fs, cfg, by, open, _ := build()
			got, err := parsePositional(fs, tc.args)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("positionals = %v, want %v", got, tc.want)
			}
			if *cfg != tc.cfg {
				t.Errorf("-config = %q, want %q — the wrong configuration would be read", *cfg, tc.cfg)
			}
			if *by != tc.by {
				t.Errorf("-by = %q, want %q", *by, tc.by)
			}
			if *open != tc.open {
				t.Errorf("-open = %v, want %v", *open, tc.open)
			}
		})
	}
}

// TestAnUnknownFlagIsAnErrorNotAPositional. Swallowing it would turn a typo
// into a command that ran and did something else.
func TestAnUnknownFlagIsAnErrorNotAPositional(t *testing.T) {
	fs := flag.NewFlagSet("t", flag.ContinueOnError)
	fs.SetOutput(discard{})
	fs.String("by", "", "")
	if _, err := parsePositional(fs, []string{"d-1", "-nope", "x"}); err == nil {
		t.Error("an undefined flag was accepted")
	}
}

type discard struct{}

func (discard) Write(p []byte) (int, error) { return len(p), nil }
