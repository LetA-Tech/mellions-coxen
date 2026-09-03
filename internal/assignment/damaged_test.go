// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package assignment

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// TestOneUnreadableRecordDoesNotHideTheRest.
//
// A record is written whole and renamed into place, so a truncated one means
// the write did not survive — a crash, a full disk, an interrupted copy. Found
// by an adversarial session feeding exactly that.
//
// It used to fail the entire listing, which took `mellions continue` with it.
// That is the command for a session that did not attend the one before it, so a
// record truncated by whatever ended that session is its ordinary case: the one
// command that must work in that state was the one that could not.
func TestOneUnreadableRecordDoesNotHideTheRest(t *testing.T) {
	s := fakeStore(t)
	noAmbientSession(t)
	for _, id := range []string{"good-a", "broken", "good-b"} {
		if _, err := s.Open(OpenOptions{ID: id, Repo: "r", Objective: "o",
			Because: "the most valuable work available", Source: t.TempDir()}); err != nil {
			t.Fatal(err)
		}
	}
	// Truncate one, the way an interrupted write leaves it.
	path := s.file("broken")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw[:len(raw)/3], 0o644); err != nil {
		t.Fatal(err)
	}

	list, damaged, err := s.ListWithDamage(true)
	if err != nil {
		t.Fatalf("one unreadable record failed the whole listing: %v", err)
	}
	var ids []string
	for _, a := range list {
		ids = append(ids, a.ID)
	}
	for _, want := range []string{"good-a", "good-b"} {
		if !slices.Contains(ids, want) {
			t.Errorf("%s vanished because a sibling record was damaged: %v", want, ids)
		}
	}
	if slices.Contains(ids, "broken") {
		t.Error("an unreadable record was listed as if it had been read")
	}
	if !slices.Contains(damaged, "broken") {
		t.Errorf("the damaged record was not named: %v — silently listing two of three is how "+
			"somebody concludes the third was closed", damaged)
	}
}

// TestDamagedIsNotAbsent. Absent means the work was never here; damaged means
// it was, and the record of it did not survive. Reading the second as the first
// is how a session concludes unfinished work was finished.
func TestDamagedIsNotAbsent(t *testing.T) {
	s := fakeStore(t)
	noAmbientSession(t)
	if _, err := s.Open(OpenOptions{ID: "x", Repo: "r", Objective: "o",
		Because: "the most valuable work available", Source: t.TempDir()}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s.file("x"), []byte("{\"id\":\"x\""), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := s.Get("x")
	if err == nil {
		t.Fatal("a truncated record read as a valid one")
	}
	if errors.Is(err, ErrNotFound) {
		t.Error("a damaged record reads as absent, so unfinished work reads as never having existed")
	}
	if !errors.Is(err, ErrDamaged) {
		t.Errorf("error does not identify itself as damage: %v", err)
	}
	// And it says what survives, because something does.
	if got := err.Error(); !contains(got, "branch") || !contains(got, "commits") {
		t.Errorf("the error does not say the branch and commits are untouched: %v", got)
	}
	_ = filepath.Base("")
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()
}
