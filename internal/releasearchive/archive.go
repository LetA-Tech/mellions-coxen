// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

// Package releasearchive creates the platform archives published with a release.
package releasearchive

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type entry struct {
	path     string
	mode     int64
	typeflag byte
}

var releaseEntries = []entry{
	{path: "", mode: 0o755, typeflag: tar.TypeDir},
	{path: "LICENSE", mode: 0o644, typeflag: tar.TypeReg},
	{path: "NOTICE", mode: 0o644, typeflag: tar.TypeReg},
	{path: "README.md", mode: 0o644, typeflag: tar.TypeReg},
	{path: "config", mode: 0o755, typeflag: tar.TypeDir},
	{path: "config/mellions.example.json", mode: 0o644, typeflag: tar.TypeReg},
	{path: "mellions", mode: 0o755, typeflag: tar.TypeReg},
}

// Build writes one archive with a fixed manifest and host-independent metadata.
func Build(root, output string, epoch time.Time) error {
	root = filepath.Clean(root)
	base := filepath.Base(root)
	if err := validatePackageName(base); err != nil {
		return fmt.Errorf("invalid package root %q: %w", root, err)
	}
	if err := checkSources(root); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(output), ".release-*.tar.gz")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	ok := false
	defer func() {
		if !ok {
			_ = os.Remove(tmpName)
		}
	}()

	err = writeArchive(tmp, root, base, epoch, releaseEntries)
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if err := os.Chmod(tmpName, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmpName, output); err != nil {
		return err
	}
	ok = true
	if err := Verify(output, base, epoch); err != nil {
		_ = os.Remove(output)
		return fmt.Errorf("verify written archive: %w", err)
	}
	return nil
}

func checkSources(root string) error {
	for _, e := range releaseEntries {
		if e.typeflag == tar.TypeDir {
			continue
		}
		path := filepath.Join(root, filepath.FromSlash(e.path))
		info, err := os.Lstat(path)
		if err != nil {
			return fmt.Errorf("required release file %s: %w", e.path, err)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("required release file %s is not a regular file", e.path)
		}
	}
	return nil
}

func writeArchive(dst io.Writer, root, base string, epoch time.Time, entries []entry) error {
	epoch = epoch.UTC().Truncate(time.Second)
	gz, err := gzip.NewWriterLevel(dst, gzip.BestCompression)
	if err != nil {
		return err
	}
	gz.Header.ModTime = epoch
	gz.Header.OS = 255
	tw := tar.NewWriter(gz)
	for _, e := range entries {
		name := base + "/"
		if e.path != "" {
			name += e.path
			if e.typeflag == tar.TypeDir {
				name += "/"
			}
		}
		h := &tar.Header{
			Name: name, Mode: e.mode, ModTime: epoch,
			Typeflag: e.typeflag, Uid: 0, Gid: 0, Uname: "root", Gname: "root",
			Format: tar.FormatUSTAR,
		}
		if e.typeflag == tar.TypeReg {
			path := filepath.Join(root, filepath.FromSlash(e.path))
			f, err := os.Open(path)
			if err != nil {
				return closeWriters(tw, gz, fmt.Errorf("open %s: %w", e.path, err))
			}
			info, err := f.Stat()
			if err != nil {
				_ = f.Close()
				return closeWriters(tw, gz, err)
			}
			h.Size = info.Size()
			if err := tw.WriteHeader(h); err != nil {
				_ = f.Close()
				return closeWriters(tw, gz, err)
			}
			_, copyErr := io.Copy(tw, f)
			closeErr := f.Close()
			if copyErr != nil {
				return closeWriters(tw, gz, copyErr)
			}
			if closeErr != nil {
				return closeWriters(tw, gz, closeErr)
			}
			continue
		}
		if err := tw.WriteHeader(h); err != nil {
			return closeWriters(tw, gz, err)
		}
	}
	return closeWriters(tw, gz, nil)
}

func closeWriters(tw *tar.Writer, gz *gzip.Writer, prior error) error {
	if err := tw.Close(); prior == nil {
		prior = err
	}
	if err := gz.Close(); prior == nil {
		prior = err
	}
	return prior
}

// Verify checks the exact manifest and rejects host-derived header metadata.
func Verify(path, base string, epoch time.Time) error {
	if err := validatePackageName(base); err != nil {
		return fmt.Errorf("invalid package name %q: %w", base, err)
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()
	epoch = epoch.UTC().Truncate(time.Second)
	if err := verifyGzipHeader(gz.Header, epoch); err != nil {
		return err
	}

	tr := tar.NewReader(gz)
	for _, want := range releaseEntries {
		h, err := tr.Next()
		if err != nil {
			return fmt.Errorf("archive ended before %s: %w", want.path, err)
		}
		if err := verifyTarHeader(h, want, base, epoch); err != nil {
			return err
		}
		if _, err := io.Copy(io.Discard, tr); err != nil {
			return err
		}
	}
	if _, err := tr.Next(); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("archive contains an unexpected extra entry")
		}
		return err
	}
	return nil
}

func validatePackageName(name string) error {
	if name == "" || name == "." || name == ".." || filepath.IsAbs(name) ||
		strings.ContainsAny(name, "/\\\x00") {
		return errors.New("name must be one safe path component")
	}
	return nil
}

func verifyGzipHeader(h gzip.Header, epoch time.Time) error {
	if !h.ModTime.Equal(epoch) || h.Name != "" || h.Comment != "" || len(h.Extra) != 0 || h.OS != 255 {
		return errors.New("gzip header carries unexpected metadata")
	}
	return nil
}

func verifyTarHeader(h *tar.Header, want entry, base string, epoch time.Time) error {
	name := base + "/"
	if want.path != "" {
		name += want.path
		if want.typeflag == tar.TypeDir {
			name += "/"
		}
	}
	if h.Name != name || h.Typeflag != want.typeflag || h.Mode != want.mode {
		return fmt.Errorf("entry %q has unexpected name, type, or mode", h.Name)
	}
	if h.Uid != 0 || h.Gid != 0 || h.Uname != "root" || h.Gname != "root" {
		return fmt.Errorf("entry %q carries host ownership", h.Name)
	}
	if !h.ModTime.Equal(epoch) || !h.AccessTime.IsZero() || !h.ChangeTime.IsZero() {
		return fmt.Errorf("entry %q carries host timestamps", h.Name)
	}
	if h.Linkname != "" || len(h.PAXRecords) != 0 || len(h.Xattrs) != 0 || h.Devmajor != 0 || h.Devminor != 0 {
		return fmt.Errorf("entry %q carries unexpected link, extension, or device metadata", h.Name)
	}
	if want.typeflag == tar.TypeDir && h.Size != 0 {
		return fmt.Errorf("directory entry %q carries a payload", h.Name)
	}
	if h.Format != tar.FormatUSTAR {
		return fmt.Errorf("entry %q is not ustar", h.Name)
	}
	return nil
}
