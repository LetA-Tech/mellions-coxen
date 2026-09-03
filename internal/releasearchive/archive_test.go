// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package releasearchive

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBuildIsDeterministicAndVerified(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "mellions_0.1.0_test_arch")
	writeFixture(t, root)
	epoch := time.Unix(1_788_345_678, 0)
	one := filepath.Join(dir, "one.tar.gz")
	two := filepath.Join(dir, "two.tar.gz")
	if err := Build(root, one, epoch); err != nil {
		t.Fatal(err)
	}
	for _, e := range releaseEntries {
		if e.typeflag != tar.TypeDir {
			if err := os.Chtimes(filepath.Join(root, filepath.FromSlash(e.path)), time.Now(), time.Now()); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := Build(root, two, epoch); err != nil {
		t.Fatal(err)
	}
	a, _ := os.ReadFile(one)
	b, _ := os.ReadFile(two)
	if !bytes.Equal(a, b) {
		t.Error("the same inputs and epoch produced different archives")
	}
}

func TestBuildRequiresEveryPublishedFile(t *testing.T) {
	root := filepath.Join(t.TempDir(), "release")
	writeFixture(t, root)
	if err := os.Remove(filepath.Join(root, "NOTICE")); err != nil {
		t.Fatal(err)
	}
	err := Build(root, filepath.Join(t.TempDir(), "release.tar.gz"), time.Unix(1, 0))
	if err == nil || !strings.Contains(err.Error(), "NOTICE") {
		t.Fatalf("Build without NOTICE = %v, want a NOTICE error", err)
	}
}

func TestBuildAndVerifyRejectPathTraversingPackageNames(t *testing.T) {
	if err := Build("..", filepath.Join(t.TempDir(), "release.tar.gz"), time.Unix(1, 0)); err == nil {
		t.Fatal("Build accepted .. as the package root")
	}
	if err := Verify(filepath.Join(t.TempDir(), "missing.tar.gz"), "..", time.Unix(1, 0)); err == nil {
		t.Fatal("Verify accepted .. as the package name")
	}
}

func TestHeaderVerificationRejectsExtensionAndLinkMetadata(t *testing.T) {
	epoch := time.Unix(1_788_345_678, 0).UTC()
	gz := gzip.Header{ModTime: epoch, OS: 255}
	if err := verifyGzipHeader(gz, epoch); err != nil {
		t.Fatalf("canonical gzip header: %v", err)
	}
	gz.Extra = []byte("host metadata")
	if err := verifyGzipHeader(gz, epoch); err == nil {
		t.Fatal("gzip header verifier accepted extra metadata")
	}

	h := &tar.Header{
		Name: "release/", Mode: 0o755, Typeflag: tar.TypeDir,
		ModTime: epoch, Uid: 0, Gid: 0, Uname: "root", Gname: "root", Format: tar.FormatUSTAR,
	}
	if err := verifyTarHeader(h, releaseEntries[0], "release", epoch); err != nil {
		t.Fatalf("canonical tar header: %v", err)
	}
	h.Linkname = "../host-file"
	if err := verifyTarHeader(h, releaseEntries[0], "release", epoch); err == nil {
		t.Fatal("tar header verifier accepted link metadata")
	}
}

func TestVerifyRejectsIncompleteManifest(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "release")
	writeFixture(t, root)
	epoch := time.Unix(1_788_345_678, 0)
	path := filepath.Join(dir, "incomplete.tar.gz")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	withoutNotice := append([]entry(nil), releaseEntries[:2]...)
	withoutNotice = append(withoutNotice, releaseEntries[3:]...)
	if err := writeArchive(f, root, filepath.Base(root), epoch, withoutNotice); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if err := Verify(path, filepath.Base(root), epoch); err == nil {
		t.Fatal("Verify accepted an archive without NOTICE")
	}
}

func writeFixture(t *testing.T, root string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, "config"), 0o755); err != nil {
		t.Fatal(err)
	}
	for path, body := range map[string]string{
		"LICENSE": "license\n", "NOTICE": "notice\n", "README.md": "readme\n",
		"config/mellions.example.json": "{}\n", "mellions": "binary\n",
	} {
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(path)), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}
