// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/LetA-Tech/mellions-coxen/internal/provenance"
)

// Shared handling for the provenance-marked documents the engineer keeps: a
// program of responsibility, and a partnership with a person.

// slugsIn lists the documents in a directory.
func slugsIn(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
			out = append(out, strings.TrimSuffix(e.Name(), ".md"))
		}
	}
	return out, nil
}

// onlyOrNamedIn resolves which document a command means: the named one, or the
// only one. With several, guessing would act on the wrong thing.
func onlyOrNamedIn(dir, named, remedy string) (string, error) {
	if named != "" {
		return named, nil
	}
	slugs, err := slugsIn(dir)
	if err != nil || len(slugs) == 0 {
		return "", fmt.Errorf("nothing in %s yet — run `%s`", dir, remedy)
	}
	if len(slugs) > 1 {
		return "", fmt.Errorf("several exist (%s); name the one you mean", strings.Join(slugs, ", "))
	}
	return slugs[0], nil
}

func docList(dir, noun, remedy string, load func(string) (*provenance.Doc, error)) error {
	slugs, err := slugsIn(dir)
	if err != nil || len(slugs) == 0 {
		fmt.Printf("no %ss yet — run `%s`\n", noun, remedy)
		return nil
	}
	now := time.Now()
	for _, s := range slugs {
		d, err := load(filepath.Join(dir, s+".md"))
		if err != nil {
			fmt.Printf("%-28s UNREADABLE: %v\n", s, err)
			continue
		}
		state := "draft"
		if d.Adopted != "" {
			state = "adopted"
		}
		age := ""
		if a := d.Age(now); a > 0 {
			age = fmt.Sprintf("evidence %dd old", int(a.Hours()/24))
		}
		fmt.Printf("%-28s %-8s %-3d sections  %s\n", s, state, len(d.Sections), age)
	}
	return nil
}

func docCheck(slug string, d *provenance.Doc, staleDays int) error {
	findings := d.Check(time.Now(), time.Duration(staleDays)*24*time.Hour)
	if len(findings) == 0 {
		fmt.Printf("%s: well formed, %d sections\n", slug, len(d.Sections))
		return nil
	}
	fmt.Printf("%s — %d finding(s)\n\n", slug, len(findings))
	for _, f := range findings {
		fmt.Printf("  %s\n", f)
	}
	return errors.New("the document needs work")
}

// docAdopt records that a person read a document and accepted it. Until then
// it is rendered everywhere as a draft, because a draft read as intent looks
// like the person said something they did not.
func docAdopt(kind provenance.Kind, args []string) error {
	fs := newFlagSet("adopt", flag.ExitOnError)
	cfgPath := fs.String("config", "", "config file")
	by := fs.String("by", "", "who is adopting it")
	rest, err := parsePositional(fs, args)
	if err != nil {
		return err
	}
	if strings.TrimSpace(*by) == "" {
		return errors.New("adoption needs a name: an adopted document asserts that a person read it")
	}
	cfg, err := loadConfig(*cfgPath)
	if err != nil {
		return err
	}
	dir, remedy := cfg.programsDir(), "mellions program discover"
	if kind == provenance.KindPartnership {
		dir, remedy = cfg.partnersDir(), "mellions partner establish"
	}
	slug, err := onlyOrNamedIn(dir, arg(rest, 0), remedy)
	if err != nil {
		return err
	}
	path := filepath.Join(dir, slug+".md")
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	stamp := fmt.Sprintf("adopted: %s by %s", time.Now().UTC().Format("2006-01-02"), *by)
	body := string(raw)
	if strings.Contains(body, "\nadopted:") {
		var out []string
		for line := range strings.SplitSeq(body, "\n") {
			if strings.HasPrefix(line, "adopted:") {
				line = stamp
			}
			out = append(out, line)
		}
		body = strings.Join(out, "\n")
	} else {
		body = strings.Replace(body, "\n\n", "\n"+stamp+"\n\n", 1)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		return err
	}
	fmt.Printf("%s adopted by %s\n", slug, *by)
	return nil
}

// parsePositional parses flags and positional arguments in any order. Go's
// flag package stops at the first non-flag argument, and people write flags
// where the sentence puts them.
func parsePositional(fs *flag.FlagSet, args []string) ([]string, error) {
	var positional, flags []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			positional = append(positional, args[i+1:]...)
			break
		}
		if a == "-" || !strings.HasPrefix(a, "-") {
			positional = append(positional, a)
			continue
		}
		flags = append(flags, a)
		if strings.Contains(a, "=") {
			continue
		}
		f := fs.Lookup(strings.TrimLeft(a, "-"))
		if f == nil {
			continue
		}
		if b, ok := f.Value.(interface{ IsBoolFlag() bool }); ok && b.IsBoolFlag() {
			continue
		}
		if i+1 < len(args) {
			i++
			flags = append(flags, args[i])
		}
	}
	if err := fs.Parse(flags); err != nil {
		return nil, err
	}
	return positional, nil
}

func arg(rest []string, i int) string {
	if i < len(rest) {
		return rest[i]
	}
	return ""
}
