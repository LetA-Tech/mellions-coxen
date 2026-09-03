// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package pluginreg

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestContextSinceCountsOnlyAfterTheNewestCompaction(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "s.jsonl")
	before := strings.Repeat(`{"type":"assistant","x":"aaaaaaaaaa"}`+"\n", 50)
	boundary := `{"type":"system","subtype":"compact_boundary"}` + "\n"
	after := strings.Repeat(`{"type":"user","x":"bbbbbbbbbbbbbbb"}`+"\n", 20)
	if err := os.WriteFile(p, []byte(before+boundary+after), 0o644); err != nil {
		t.Fatal(err)
	}
	got, ok := ContextSinceIn(p)
	if !ok || got != int64(len(after)) {
		t.Fatalf("got %d, %v; want %d — the bytes after the boundary only", got, ok, len(after))
	}
	if err := os.WriteFile(p, []byte(before), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, _ := ContextSinceIn(p); got != int64(len(before)) {
		t.Fatalf("no boundary: got %d, want the whole file %d", got, len(before))
	}
	if _, ok := ContextSinceIn(filepath.Join(dir, "missing.jsonl")); ok {
		t.Fatal("a missing transcript is not a zero-sized one")
	}
}

func TestCompactionSizeIsTheMedianOfAutomaticGapsOnly(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, ".claude", "projects", "p")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	pad := func(n int) string {
		return strings.Repeat(`{"type":"assistant","message":{"model":"claude-opus-5"},"x":"`+strings.Repeat("a", 60)+`"}`+"\n", n)
	}
	auto := `{"type":"system","subtype":"compact_boundary","compactMetadata":{"trigger":"auto"}}` + "\n"
	manual := `{"type":"system","subtype":"compact_boundary","compactMetadata":{"trigger":"manual"}}` + "\n"
	// 3 MB then auto, 1 MB then manual (ignored), 5 MB then auto
	body := pad(30000) + auto + pad(10000) + manual + pad(50000) + auto
	if err := os.WriteFile(filepath.Join(dir, "s.jsonl"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if m := TranscriptModel(filepath.Join(dir, "s.jsonl")); m != "claude-opus-5" {
		t.Fatalf("model = %q", m)
	}
	if _, n := CompactionSize(home, "claude-sonnet-5"); n != 0 {
		t.Fatalf("another model's compactions were counted: %d", n)
	}
	size, n := CompactionSize(home, "claude-opus-5")
	if n != 2 {
		t.Fatalf("samples = %d, want 2 automatic compactions", n)
	}
	lo, hi := int64(len(pad(30000))), int64(len(pad(50000)))
	if size < lo || size > hi+200 {
		t.Fatalf("size = %d, want the median of the automatic gaps (between %d and %d)", size, lo, hi)
	}
	if size, n := CompactionSize(t.TempDir(), "claude-opus-5"); size != 0 || n != 0 {
		t.Fatalf("no transcripts: got %d, %d", size, n)
	}
}
