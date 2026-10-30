// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package assignment

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"time"
)

// EvidenceBound is the largest scratch file a reclaim keeps. A log, a
// rendered body or a captured test run sits far below it; a binary, an
// archive or a database dump sits above it and is rebuilt from a commit.
const EvidenceBound = 1 << 20

// ReclaimAfter is how long a finished lane's scratch stays whole. A lane
// closed by mistake is reopened inside it with every artifact intact.
const ReclaimAfter = 24 * time.Hour

// Reclaimed is what a reclaim freed from a finished lane's scratch.
type Reclaimed struct {
	At    time.Time `json:"at"`
	Bytes int64     `json:"bytes"`
	Files int       `json:"files"`
	Kept  int       `json:"kept"`
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

// reclaimScratch removes from dir everything a commit or a build tool
// rebuilds, and keeps small text files outside such trees as the lane's
// evidence. Without apply it only measures what it would remove.
//
// A symlink is removed as a link and never followed, so a scratch pointing
// into a worktree or a shared cache cannot reach past itself.
func reclaimScratch(dir string, apply bool) (Reclaimed, error) {
	var r Reclaimed
	info, err := os.Lstat(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return r, nil
	}
	if err != nil {
		return r, err
	}
	if !info.IsDir() {
		return r, fmt.Errorf("assignment: scratch %s is not a directory", dir)
	}
	err = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == dir {
			return nil
		}
		if d.IsDir() {
			if !rebuildable(path, d.Name()) {
				return nil
			}
			n, files := treeSize(path)
			if apply {
				if err := removeTree(path); err != nil {
					return err
				}
			}
			r.Bytes += n
			r.Files += files
			return filepath.SkipDir
		}
		fi, err := d.Info()
		if err != nil {
			return err
		}
		if keepAsEvidence(path, fi) {
			r.Kept++
			return nil
		}
		if apply {
			if err := os.Remove(path); err != nil {
				return err
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
		pruneEmpty(dir)
	}
	return r, nil
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

// keepAsEvidence is a regular file within EvidenceBound whose first 8000
// bytes hold no NUL, the test git applies before it calls a file binary.
func keepAsEvidence(path string, fi fs.FileInfo) bool {
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

func treeSize(dir string) (int64, int) {
	var n int64
	var files int
	filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if fi, err := d.Info(); err == nil {
			n += fi.Size()
			files++
		}
		return nil
	})
	return n, files
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
// and never dir itself.
func pruneEmpty(dir string) {
	var dirs []string
	filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err == nil && d.IsDir() && path != dir {
			dirs = append(dirs, path)
		}
		return nil
	})
	for i := len(dirs) - 1; i >= 0; i-- {
		os.Remove(dirs[i])
	}
}

// reclaimDue says whether a finished lane's scratch is past its horizon and
// not yet reclaimed.
func (s *Store) reclaimDue(a *Assignment, after time.Duration) bool {
	if a.State != StateClosed && a.State != StateAbandoned {
		return false
	}
	if a.Reclaimed != nil || a.ClosedAt.IsZero() {
		return false
	}
	return s.clock().Sub(a.ClosedAt) >= after
}

// reclaim frees a finished lane's rebuildable scratch and records what went.
//
// The directory is always <store>/<id>/scratch, never the path the record
// names: a record is data, and a reclaim that followed it could be pointed at
// anything on the disk.
func (s *Store) reclaim(a *Assignment, apply bool) Swept {
	v := Swept{ID: a.ID, Verdict: "reclaimable"}
	dir := filepath.Join(s.dir(a.ID), "scratch")
	r, err := reclaimScratch(dir, apply)
	if err != nil {
		v.Verdict = "unreclaimed"
		v.Why = fmt.Sprintf("reclaiming %s failed after %s: %v", dir, sizeText(r.Bytes), err)
		return v
	}
	v.Why = fmt.Sprintf("%s %s closed %s: %s in %d file(s) rebuildable, %d evidence file(s) kept",
		a.State, a.ID, a.ClosedAt.UTC().Format("2006-01-02 15:04 UTC"), sizeText(r.Bytes), r.Files, r.Kept)
	if !apply {
		return v
	}
	r.At = s.clock()
	if _, err := s.update(a.ID, func(a *Assignment) error {
		a.Reclaimed = &r
		return nil
	}); err != nil {
		v.Verdict, v.Why = "unreclaimed", "scratch freed but the record could not say so: "+err.Error()
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
