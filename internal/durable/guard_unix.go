// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

//go:build unix

package durable

import (
	"fmt"
	"os"
	"syscall"
)

// Guard runs fn holding an exclusive lock for path, so that reading a record,
// deciding from it and writing it back is one operation across processes.
//
// The lock is a file beside the record rather than the record itself: Write
// renames a new inode into place, and a lock held on the replaced inode guards
// nothing. It is released when the descriptor closes, which the kernel does for
// a process that dies — so a session killed mid-claim blocks nobody.
func Guard(path string, fn func() error) error {
	f, err := os.OpenFile(path+".lock", os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return fmt.Errorf("durable: lock %s: %w", path, err)
	}
	defer f.Close()
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		return fmt.Errorf("durable: lock %s: %w", path, err)
	}
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	return fn()
}
