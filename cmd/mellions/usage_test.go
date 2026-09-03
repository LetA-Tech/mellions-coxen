package main

import (
	"bytes"
	"flag"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// commandFiles is the command's own source, tests excluded: the guards below
// are about what the shipped subcommands do, not what a test constructs.
func commandFiles(t *testing.T) map[string]*ast.File {
	t.Helper()
	paths, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	files := map[string]*ast.File{}
	for _, p := range paths {
		if strings.HasSuffix(p, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, p, nil, 0)
		if err != nil {
			t.Fatalf("%s: %v", p, err)
		}
		files[p] = f
	}
	if len(files) == 0 {
		t.Fatal("no command source found; the guards below would pass vacuously")
	}
	return files
}

// calls reports the string literal first argument of every call to fn, by file.
func calls(files map[string]*ast.File, pkg, fn string) map[string][]string {
	found := map[string][]string{}
	for path, f := range files {
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			switch c := call.Fun.(type) {
			case *ast.SelectorExpr:
				id, ok := c.X.(*ast.Ident)
				if !ok || pkg == "" || id.Name != pkg || c.Sel.Name != fn {
					return true
				}
			case *ast.Ident:
				if pkg != "" || c.Name != fn {
					return true
				}
			default:
				return true
			}
			name := ""
			if len(call.Args) > 0 {
				if lit, ok := call.Args[0].(*ast.BasicLit); ok && lit.Kind == token.STRING {
					if s, err := strconv.Unquote(lit.Value); err == nil {
						name = s
					}
				}
			}
			found[path] = append(found[path], name)
			return true
		})
	}
	return found
}

// A subcommand that builds its flag set directly gets flag's own usage output,
// which lists flags and cannot mention a positional argument. newFlagSet is the
// only construction, so that cannot happen by omission.
func TestEveryFlagSetIsBuiltByNewFlagSet(t *testing.T) {
	for path, names := range calls(commandFiles(t), "flag", "NewFlagSet") {
		if path == "usage.go" {
			continue // the single construction newFlagSet wraps
		}
		t.Errorf("%s builds %d flag set(s) with flag.NewFlagSet %v; use newFlagSet so the usage line names the positional arguments", path, len(names), names)
	}
}

func TestEveryFlagSetNameHasAUsageForm(t *testing.T) {
	seen := map[string]bool{}
	for path, names := range calls(commandFiles(t), "", "newFlagSet") {
		if path == "usage.go" {
			continue // the definition, not a call site
		}
		for _, name := range names {
			if name == "" {
				t.Errorf("%s: newFlagSet called without a string literal name; the usage line is keyed on it", path)
				continue
			}
			seen[name] = true
			if _, ok := usageForms[name]; !ok {
				t.Errorf("%s: subcommand %q has no entry in usageForms, so an unknown flag prints a bare flag list", path, name)
			}
		}
	}
	if len(seen) == 0 {
		t.Fatal("no newFlagSet call sites found; this guard would pass vacuously")
	}
	for name := range usageForms {
		if !seen[name] {
			t.Errorf("usageForms has %q, which no subcommand builds; a form nobody prints drifts unnoticed", name)
		}
	}
}

// The path the defect was on: a wrong flag name, not a missing argument.
func TestUnknownFlagPrintsTheUsageLineNotOnlyTheFlagList(t *testing.T) {
	fs := newFlagSet("assign record", flag.ContinueOnError)
	fs.String("kind", "note", "hypothesis, found, next or note")
	var out bytes.Buffer
	fs.SetOutput(&out)

	if err := fs.Parse([]string{"-note", "text"}); err == nil {
		t.Fatal("expected -note to be rejected; the test no longer reaches the usage path")
	}
	got := out.String()
	for _, want := range []string{"usage: mellions assign record", "<id>", "<text>"} {
		if !strings.Contains(got, want) {
			t.Errorf("usage output does not mention %q, which is what hid the argument:\n%s", want, got)
		}
	}
	if !strings.Contains(got, "-kind") {
		t.Errorf("usage output dropped the flag list:\n%s", got)
	}
}

// The hand-written missing-argument error and the usage line are one string, so
// a change to the command's shape cannot correct one and leave the other.
func TestMissingArgumentErrorCarriesTheSameUsageLine(t *testing.T) {
	t.Setenv("MELLIONS_CONFIG", filepath.Join(t.TempDir(), "missing.json"))
	err := assignRecord(nil)
	if err == nil {
		t.Fatal("assign record with no id and no text should be an error")
	}
	if !strings.Contains(err.Error(), usageLine("assign record")) {
		t.Errorf("missing-argument error and usage line have drifted apart:\n%s\nwant to contain: %s", err.Error(), usageLine("assign record"))
	}
}

func TestUsageLineOfAnUnknownNameStillNamesTheCommand(t *testing.T) {
	if got, want := usageLine("no such subcommand"), "usage: mellions no such subcommand [flags]"; got != want {
		t.Errorf("usageLine fallback = %q, want %q", got, want)
	}
}
