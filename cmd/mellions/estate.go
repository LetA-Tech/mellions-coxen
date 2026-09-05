// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package main

import (
	"context"
	"flag"
	"fmt"
	"regexp"
	"strings"

	estatepkg "github.com/LetA-Tech/mellions-coxen/internal/estate"
)

const estateUsage = `usage: mellions estate read <path> [flags]

Reads one path out of every configured repository, at a stated ref, and says per
repository what it found.

  -ref string     ref to read at (default: each repository's origin/HEAD, then
                  origin/dev, origin/main, origin/master)
  -fetch          refresh remote-tracking refs first; without it a ref is only
                  as current as the last fetch, and the output says so
  -grep regexp    keep only matching lines
  -worktree       read the working tree instead of a ref — the answer is then
                  whatever branch each checkout was left on, and is labelled
  -repos list     comma-separated repository names (default: all configured)

Every configured repository produces a row, including the ones that could not be
read, so "nothing found" is distinguishable from "never opened".
`

func cmdEstate(ctx context.Context, args []string) error {
	if len(args) == 0 {
		fmt.Print(estateUsage)
		return nil
	}
	switch args[0] {
	case "read":
		return estateRead(ctx, args[1:])
	case "-h", "--help", "help":
		fmt.Print(estateUsage)
		return nil
	default:
		return fmt.Errorf("estate: unknown subcommand %q\n\n%s", args[0], estateUsage)
	}
}

func estateRead(ctx context.Context, args []string) error {
	fs := newFlagSet("estate read", flag.ContinueOnError)
	ref := fs.String("ref", "", "ref to read at")
	doFetch := fs.Bool("fetch", false, "fetch before reading")
	grep := fs.String("grep", "", "keep only matching lines")
	worktree := fs.Bool("worktree", false, "read the working tree, not a ref")
	only := fs.String("repos", "", "comma-separated repository names")
	// The path is taken before parsing when it leads, because flag stops at the
	// first non-flag argument and `estate read go.mod -grep x` is how anyone
	// types it. Trailing form still works.
	var path string
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		path, args = args[0], args[1:]
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if path == "" && fs.NArg() == 1 {
		path = fs.Arg(0)
	}
	if path == "" || (path != "" && fs.NArg() > 0 && fs.Arg(0) != path) {
		return fmt.Errorf("estate read needs exactly one path\n\n%s", estateUsage)
	}

	cfg, err := loadConfig("")
	if err != nil {
		return err
	}
	repos := map[string]string{}
	set := cfg.checkouts()
	wanted := cfg.Repos
	if *only != "" {
		wanted = nil
		for _, n := range strings.Split(*only, ",") {
			if n = strings.TrimSpace(n); n != "" {
				wanted = append(wanted, n)
			}
		}
	}
	for _, n := range wanted {
		dir, _ := set.Dir(n)
		repos[n] = dir
	}
	if len(repos) == 0 {
		return fmt.Errorf("no repositories configured — `mellions config`")
	}

	req := estatepkg.Request{Repos: repos, Path: path, Ref: *ref, Worktree: *worktree, Fetch: *doFetch}
	if *grep != "" {
		re, err := regexp.Compile(*grep)
		if err != nil {
			return fmt.Errorf("bad -grep: %w", err)
		}
		req.Grep = re
	}

	results := estatepkg.Read(ctx, req)
	sum := estatepkg.Summarize(results)

	fmt.Printf("# %s\n\n", path)
	for _, r := range results {
		head := fmt.Sprintf("%-22s %-14s %s", r.Repo, r.Ref, r.Status)
		if r.Commit != "" {
			head = fmt.Sprintf("%-22s %-14s %-8s %s", r.Repo, r.Ref, r.Commit[:min(8, len(r.Commit))], r.Status)
		}
		fmt.Println(strings.TrimRight(head, " "))
		if r.Detail != "" {
			fmt.Printf("    %s\n", r.Detail)
		}
		if req.Grep != nil && r.Status.Measured() {
			if len(r.Matches) == 0 {
				// Said out loud. A repository that matched nothing is an
				// answer, and printing nothing makes it look like a row that
				// was never read.
				fmt.Printf("    (no line matches)\n")
			}
			for _, m := range r.Matches {
				fmt.Printf("    %s\n", strings.TrimSpace(m))
			}
		}
	}

	fmt.Printf("\n%s\n", sum.Line())
	if *worktree {
		fmt.Println("READ FROM WORKING TREES — each answer is whatever branch that checkout was left on, " +
			"not what the repository holds. Re-read at a ref before publishing any of it.")
	}
	if !*doFetch && !*worktree {
		fmt.Println("No fetch: every ref above is as current as that checkout's last fetch. `-fetch` refreshes them.")
	}
	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
