// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

// Package durable makes a file-backed record survive the two things that end a
// session without warning: another process, and the machine.
//
// Both stores that carry consequence — an approved action about to run, and the
// responsibility a session is holding — are a JSON file that several processes
// read, change and write back. Nothing about that is safe by default. Two
// readers see the same eligible decision and both execute it; two writers share
// one temporary name and one of them loses; a write that is not flushed reads
// back as a truncated file to whichever session resumes.
//
// The two operations here are the whole answer. Guard makes a read-modify-write
// one operation across processes. Write replaces a file so that a reader sees
// either all of the old content or all of the new, and so that content is on
// the disk rather than in a cache the crash discards.
package durable

import (
	"fmt"
	"os"
	"path/filepath"
)

// Write replaces path with b, atomically and durably.
//
// A uniquely named temporary file beside it, flushed, renamed into place, and
// the directory flushed after so the rename itself survives. The uniqueness
// matters as much as the atomicity: a shared temporary name turns two
// concurrent writers into one truncated file and one failed rename.
func Write(path string, b []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	f, err := os.CreateTemp(dir, filepath.Base(path)+".*.tmp")
	if err != nil {
		return fmt.Errorf("durable: stage %s: %w", path, err)
	}
	tmp := f.Name()
	defer os.Remove(tmp)

	if _, err := f.Write(b); err != nil {
		f.Close()
		return fmt.Errorf("durable: write %s: %w", tmp, err)
	}
	if err := f.Chmod(perm); err != nil {
		f.Close()
		return fmt.Errorf("durable: mode %s: %w", tmp, err)
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return fmt.Errorf("durable: flush %s: %w", tmp, err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("durable: close %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("durable: commit %s: %w", path, err)
	}
	return syncDir(dir)
}

// syncDir flushes the directory entry, without which a rename can be lost while
// the file it points at is intact.
func syncDir(dir string) error {
	d, err := os.Open(dir)
	if err != nil {
		return fmt.Errorf("durable: open %s: %w", dir, err)
	}
	defer d.Close()
	if err := d.Sync(); err != nil {
		return fmt.Errorf("durable: flush %s: %w", dir, err)
	}
	return nil
}
