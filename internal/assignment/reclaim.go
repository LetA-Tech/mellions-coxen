// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package assignment

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// EvidenceBound is the largest file a reclaim keeps. A log, a
// rendered body or a captured test run sits far below it; a binary, an
// archive or a database dump sits above it and is rebuilt from a commit.
const EvidenceBound = 1 << 20

// ReclaimAfter is how long a finished lane's directory stays whole, so the
// session that closed it, or the next lane on the same work, can still read
// what it built.
const ReclaimAfter = 24 * time.Hour

// laneKeeps are the entries of a lane directory a reclaim never enters: the
// record, its lock, and the worktree, which git owns and close removes.
var laneKeeps = map[string]bool{"assignment.json": true, "assignment.json.lock": true, "tree": true}

// Reclaimed is what a reclaim freed from a finished lane's directory.
type Reclaimed struct {
	At    time.Time `json:"at"`
	Bytes int64     `json:"bytes"`
	Files int       `json:"files"`
	Kept  int       `json:"kept"`
	// Unreadable is the entries the reclaim could not read or remove and
	// left in place.
	Unreadable int `json:"unreadable,omitempty"`
}

// treeMarkers are the files whose presence makes a directory something a
// checkout, an archive extraction or a build tool rebuilds: a source tree, a
// Go build cache (trim.txt), a package manager's root.
var treeMarkers = []string{"go.mod", "go.work", ".git", "package.json", "pyproject.toml",
	"setup.py", "Cargo.toml", "trim.txt"}

// toolDirs are directory names build tools create for their own temporary
// state; go-build is the prefix of a Go work directory.
var toolDirs = regexp.MustCompile(`^(go-build.*|node_modules|__pycache__|\.pytest_cache|\.mypy_cache)$`)

// cacheEntry is a Go build cache entry's file name, found outside a cache
// root when a GOCACHE was pointed at a directory without its marker.
var cacheEntry = regexp.MustCompile(`^[0-9a-f]{64}-[ad]$`)

// reclaimLane removes from a lane directory everything a commit or a build
// tool rebuilds, and keeps the record, the worktree, any checkout holding
// unsaved git work, and small text files outside such trees as the lane's
// evidence. Sessions write archive copies,
// arms and logs both under scratch/ and beside the record, so the whole
// directory is read. Without apply it only measures what it would remove.
//
// A symlink is removed as a link and never followed, so a lane directory
// pointing into a worktree or a shared cache cannot reach past itself.
func reclaimLane(dir string, protect map[string]bool, apply bool) (Reclaimed, error) {
	var r Reclaimed
	kept := map[string]bool{}
	info, err := os.Lstat(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return r, nil
	}
	if err != nil {
		return r, err
	}
	if !info.IsDir() {
		return r, fmt.Errorf("assignment: lane %s is not a directory", dir)
	}
	err = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			// One unreadable entry is left where it is and named; the rest of
			// the lane is still read.
			if path == dir {
				return err
			}
			if !errors.Is(err, fs.ErrNotExist) {
				r.Unreadable++
			}
			if d != nil && d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if path == dir {
			return nil
		}
		if filepath.Dir(path) == dir && laneKeeps[d.Name()] {
			if d.IsDir() {
				kept[path] = true
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			// A repository's own store is never read file by file: removing
			// its compressed objects as binaries would corrupt it.
			if d.Name() == ".git" || isRepository(path) {
				r.Kept++
				kept[path] = true
				return filepath.SkipDir
			}
			if protect[path] || isWorktree(path) || holdsUnsavedGit(path) {
				r.Kept++
				kept[path] = true
				return filepath.SkipDir
			}
			if !rebuildable(path, d.Name()) {
				return nil
			}
			n, files, nested := treeSize(path, protect)
			if nested {
				// A worktree or checkout somewhere inside is judged on its
				// own; the walk goes in rather than taking it with the parent.
				return nil
			}
			if apply {
				if err := removeTree(path); err != nil {
					r.Unreadable++
					return filepath.SkipDir
				}
			}
			r.Bytes += n
			r.Files += files
			return filepath.SkipDir
		}
		fi, err := d.Info()
		if err != nil {
			if !errors.Is(err, fs.ErrNotExist) {
				r.Unreadable++
			}
			return nil
		}
		if keepAsEvidence(path, fi) {
			r.Kept++
			return nil
		}
		if apply {
			if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
				r.Unreadable++
				return nil
			}
		}
		r.Bytes += fi.Size()
		r.Files++
		return nil
	})
	if err != nil {
		return r, err
	}
	if apply {
		pruneEmpty(dir, kept)
	}
	return r, nil
}

// isRepository says whether dir is a bare git repository: HEAD, objects/ and
// refs/ with no .git around them. Its packs read as binaries, and a commit
// pushed only to it exists nowhere else.
func isRepository(dir string) bool {
	head, err := os.Lstat(filepath.Join(dir, "HEAD"))
	if err != nil || !head.Mode().IsRegular() {
		return false
	}
	for _, sub := range []string{"objects", "refs"} {
		if fi, err := os.Lstat(filepath.Join(dir, sub)); err != nil || !fi.IsDir() {
			return false
		}
	}
	return true
}

// isWorktree says whether dir is a linked git worktree (.git is a file). Its
// repository registers it, and only git removes it without leaving that
// registration behind, so a reclaim never does; a lane's own worktree goes
// through close.
func isWorktree(dir string) bool {
	fi, err := os.Lstat(filepath.Join(dir, ".git"))
	return err == nil && fi.Mode().IsRegular()
}

// holdsUnsavedGit says whether dir is a git checkout holding work no commit
// on a remote carries: a clone (.git is a directory) with commits on no
// remote, a detached HEAD or a stash among them, or any checkout with
// uncommitted changes. A worktree's commits live
// in its source repository, so for a worktree (.git is a file) only
// uncommitted changes count. Such a directory is kept whole; one git cannot
// read is kept too, because unknown is not rebuildable.
func holdsUnsavedGit(dir string) bool {
	fi, err := os.Lstat(filepath.Join(dir, ".git"))
	if err != nil {
		return false
	}
	git := func(args ...string) (string, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir, "-c", "core.fsmonitor=false"}, args...)...)
		cmd.Env = append(gitFreeEnv(), "GIT_OPTIONAL_LOCKS=0")
		out, err := cmd.Output()
		return string(bytes.TrimSpace(out)), err
	}
	if out, err := git("status", "--porcelain"); err != nil || out != "" {
		return true
	}
	if !fi.IsDir() {
		return false
	}
	if _, err := git("rev-parse", "-q", "--verify", "refs/stash"); err == nil {
		return true
	}
	out, err := git("log", "HEAD", "--branches", "--tags", "--not", "--remotes", "--format=%H", "-n", "1")
	return err != nil || out != ""
}

// gitFreeEnv is the environment without GIT_* variables, so a sweep run from
// inside another repository's hook asks the checkout it names and no other.
func gitFreeEnv() []string {
	var env []string
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "GIT_") {
			env = append(env, kv)
		}
	}
	return env
}

func rebuildable(dir, name string) bool {
	if toolDirs.MatchString(name) {
		return true
	}
	for _, m := range treeMarkers {
		if _, err := os.Lstat(filepath.Join(dir, m)); err == nil {
			return true
		}
	}
	return false
}

// keepAsEvidence is a git bundle, or a regular file within EvidenceBound
// whose first 8000 bytes hold no NUL, the test git applies before it calls a
// file binary.
func keepAsEvidence(path string, fi fs.FileInfo) bool {
	// A bundle is history a session archived on purpose, whatever its size.
	if fi.Mode().IsRegular() && strings.HasSuffix(fi.Name(), ".bundle") {
		return true
	}
	if !fi.Mode().IsRegular() || fi.Size() > EvidenceBound || cacheEntry.MatchString(fi.Name()) {
		return false
	}
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	head := make([]byte, 8000)
	n, err := io.ReadFull(f, head)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return false
	}
	return bytes.IndexByte(head[:n], 0) < 0
}

// treeSize measures dir and says whether anything below it is a git
// checkout or a recorded worktree, which removing dir whole would take.
func treeSize(dir string, protect map[string]bool) (int64, int, bool) {
	var n int64
	var files int
	nested := false
	filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			// What cannot be read cannot be shown to hold no git work.
			nested = true
			return filepath.SkipAll
		}
		ownGit := d.Name() == ".git" && filepath.Dir(path) == dir
		if path != dir && ((d.Name() == ".git" && !ownGit) || protect[path] || (!ownGit && d.IsDir() && isRepository(path))) {
			nested = true
			return filepath.SkipAll
		}
		if ownGit && d.IsDir() {
			return filepath.SkipDir
		}
		if d.IsDir() {
			return nil
		}
		if fi, err := d.Info(); err == nil {
			n += fi.Size()
			files++
		}
		return nil
	})
	return n, files, nested
}

// removeTree is os.RemoveAll that first restores owner write on every
// directory: the Go module cache and a read-only archive extraction leave
// directories at 0555, which RemoveAll cannot empty.
func removeTree(dir string) error {
	if err := os.RemoveAll(dir); err == nil {
		return nil
	}
	filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err == nil && d.IsDir() {
			os.Chmod(path, 0o700)
		}
		return nil
	})
	return os.RemoveAll(dir)
}

// pruneEmpty removes the directories a reclaim left empty, deepest first,
// and never dir itself nor anything inside what the reclaim kept whole: a
// packed repository's empty refs/ is part of the repository.
func pruneEmpty(dir string, kept map[string]bool) {
	var dirs []string
	filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || !d.IsDir() || path == dir {
			return nil
		}
		if kept[path] {
			return filepath.SkipDir
		}
		dirs = append(dirs, path)
		return nil
	})
	for i := len(dirs) - 1; i >= 0; i-- {
		os.Remove(dirs[i])
	}
}

// reclaimDue says whether a finished lane is past its horizon and not yet
// reclaimed.
func (s *Store) reclaimDue(a *Assignment, after time.Duration) bool {
	if a.State != StateClosed && a.State != StateAbandoned {
		return false
	}
	if a.Reclaimed != nil || a.ClosedAt.IsZero() {
		return false
	}
	return s.clock().Sub(a.ClosedAt) >= after
}

// reclaim frees what is rebuildable in a finished lane's directory and
// records what went.
//
// The directory is always <store>/<id>, never a path the record names: a
// record is data, and a reclaim that followed it could be pointed at anything
// on the disk.
//
// The record is read again from that directory and must carry the same id: a
// listed record whose id field names some other directory, the store itself
// or its parent is never acted on. protect is every path a lane records as
// its worktree, which no reclaim enters whatever lane directory it sits in.
func (s *Store) reclaim(a *Assignment, after time.Duration, protect map[string]bool, apply bool) Swept {
	v := Swept{ID: a.ID, Verdict: "unreclaimed"}
	cur, err := s.Get(a.ID)
	if err != nil || cur.ID != a.ID {
		v.Why = fmt.Sprintf("the record's id %q does not name its own directory; not reclaimed", a.ID)
		return v
	}
	if !s.reclaimDue(cur, after) {
		v.Verdict, v.Why = cur.State, "no longer due for a reclaim"
		return v
	}
	a = cur
	v.Verdict = "reclaimable"
	dir := s.dir(a.ID)
	r, err := reclaimLane(dir, protect, apply)
	if err != nil {
		v.Verdict = "unreclaimed"
		v.Why = fmt.Sprintf("reclaiming %s failed after %s: %v", dir, sizeText(r.Bytes), err)
		return v
	}
	v.Why = fmt.Sprintf("%s %s closed %s: %s in %d file(s) rebuildable, %d evidence file(s) kept",
		a.State, a.ID, a.ClosedAt.UTC().Format("2006-01-02 15:04 UTC"), sizeText(r.Bytes), r.Files, r.Kept)
	if r.Unreadable > 0 {
		v.Why += fmt.Sprintf(", %d unreadable entr(ies) left in place", r.Unreadable)
	}
	if !apply {
		return v
	}
	r.At = s.clock()
	if _, err := s.update(a.ID, func(a *Assignment) error {
		a.Reclaimed = &r
		return nil
	}); err != nil {
		v.Verdict, v.Why = "unreclaimed", "freed, but the record could not say so: "+err.Error()
		return v
	}
	v.Verdict = "reclaimed"
	v.Bytes = r.Bytes
	return v
}

func sizeText(n int64) string {
	switch {
	case n >= 1<<30:
		return fmt.Sprintf("%.2f GiB", float64(n)/(1<<30))
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MiB", float64(n)/(1<<20))
	default:
		return fmt.Sprintf("%d KiB", n>>10)
	}
}
