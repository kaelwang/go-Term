package batch

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/kaelwang/go-Term/internal/transfer"
)

// fakeXfer is a configurable in-memory Transferer used to exercise batch
// concurrency, resume and traversal logic without any real backend.
type fakeXfer struct {
	mu            sync.Mutex
	statSize      int64
	statErr       error
	isDir         bool
	listRet       []transfer.FileEntry
	listErr       error
	uploadFn      func(src, dst string) error
	downloadFn    func(src, dst string) error
	uploads       int
	downloads    int
	sleep        time.Duration
	curConcurrent int
	maxConcurrent int
}

func (f *fakeXfer) List(path string) ([]transfer.FileEntry, error) { return f.listRet, f.listErr }
func (f *fakeXfer) Mkdir(path string) error                       { return nil }
func (f *fakeXfer) Remove(path string) error                     { return nil }
func (f *fakeXfer) Rename(old, nw string) error                  { return nil }
func (f *fakeXfer) Chmod(path string, mode uint32) error         { return nil }
func (f *fakeXfer) Symlink(old, nw string) error                 { return nil }
func (f *fakeXfer) Close() error                                 { return nil }

func (f *fakeXfer) Stat(path string) (transfer.FileInfo, error) {
	if f.statErr != nil {
		return transfer.FileInfo{}, f.statErr
	}
	return transfer.FileInfo{Size: f.statSize, IsDir: f.isDir}, nil
}

func (f *fakeXfer) Upload(src, dst string) error {
	f.mu.Lock()
	f.curConcurrent++
	if f.curConcurrent > f.maxConcurrent {
		f.maxConcurrent = f.curConcurrent
	}
	f.mu.Unlock()
	defer func() {
		f.mu.Lock()
		f.curConcurrent--
		f.uploads++
		f.mu.Unlock()
	}()
	if f.sleep > 0 {
		time.Sleep(f.sleep)
	}
	if f.uploadFn != nil {
		return f.uploadFn(src, dst)
	}
	return nil
}

func (f *fakeXfer) Download(src, dst string) error {
	f.mu.Lock()
	f.curConcurrent++
	if f.curConcurrent > f.maxConcurrent {
		f.maxConcurrent = f.curConcurrent
	}
	f.mu.Unlock()
	defer func() {
		f.mu.Lock()
		f.curConcurrent--
		f.downloads++
		f.mu.Unlock()
	}()
	if f.sleep > 0 {
		time.Sleep(f.sleep)
	}
	if f.downloadFn != nil {
		return f.downloadFn(src, dst)
	}
	return nil
}

// TestRunBoundedConcurrency verifies the semaphore caps the number of
// concurrent workers at exactly `max` (and at the 64 default when 0).
func TestRunBoundedConcurrency(t *testing.T) {
	t.Run("explicit-max-2", func(t *testing.T) {
		fx := &fakeXfer{sleep: 10 * time.Millisecond}
		items := make([]Item, 30)
		for i := range items {
			items[i] = Item{Direction: "upload", Src: fmt.Sprintf("missing-%d", i), Dst: "remote"}
		}
		if err := Run(fx, items, 2); err != nil {
			t.Fatalf("Run error: %v", err)
		}
		if fx.uploads != 30 {
			t.Errorf("uploads = %d want 30", fx.uploads)
		}
		if fx.maxConcurrent > 2 {
			t.Errorf("concurrency exceeded max=2: observed %d", fx.maxConcurrent)
		}
	})

	t.Run("default-max-64", func(t *testing.T) {
		fx := &fakeXfer{sleep: 5 * time.Millisecond}
		items := make([]Item, 120)
		for i := range items {
			items[i] = Item{Direction: "upload", Src: fmt.Sprintf("missing-%d", i), Dst: "remote"}
		}
		if err := Run(fx, items, 0); err != nil {
			t.Fatalf("Run error: %v", err)
		}
		if fx.uploads != 120 {
			t.Errorf("uploads = %d want 120", fx.uploads)
		}
		if fx.maxConcurrent > 64 {
			t.Errorf("concurrency exceeded default max=64: observed %d", fx.maxConcurrent)
		}
	})
}

// TestTransferOneResumeUpload verifies the resume rule: when the remote size
// already equals the local file size, no upload occurs.
func TestTransferOneResumeUpload(t *testing.T) {
	// Case 1: sizes match -> resume (skip upload).
	f, _ := os.CreateTemp(t.TempDir(), "src")
	f.Write(make([]byte, 100))
	f.Close()

	fx := &fakeXfer{statSize: 100, statErr: nil, uploadFn: func(_, _ string) error {
		t.Fatal("upload must be skipped when remote size matches local size")
		return nil
	}}
	if err := transferOne(fx, Item{Direction: "upload", Src: f.Name(), Dst: "remote/file"}); err != nil {
		t.Fatalf("transferOne error: %v", err)
	}
	if fx.uploads != 0 {
		t.Errorf("expected 0 uploads on resume, got %d", fx.uploads)
	}

	// Case 2: sizes differ -> upload happens.
	fx2 := &fakeXfer{statSize: 50}
	if err := transferOne(fx2, Item{Direction: "upload", Src: f.Name(), Dst: "remote/file"}); err != nil {
		t.Fatalf("transferOne error: %v", err)
	}
	if fx2.uploads != 1 {
		t.Errorf("expected 1 upload when sizes differ, got %d", fx2.uploads)
	}

	// Case 3: Stat error -> upload happens (can't confirm resume).
	fx3 := &fakeXfer{statErr: os.ErrNotExist}
	if err := transferOne(fx3, Item{Direction: "upload", Src: f.Name(), Dst: "remote/file"}); err != nil {
		t.Fatalf("transferOne error: %v", err)
	}
	if fx3.uploads != 1 {
		t.Errorf("expected 1 upload when Stat errors, got %d", fx3.uploads)
	}
}

// TestTransferOneResumeDownload verifies the download resume rule.
func TestTransferOneResumeDownload(t *testing.T) {
	dst := filepath.Join(t.TempDir(), "downloaded.bin")
	os.WriteFile(dst, make([]byte, 100), 0644)

	// Remote size matches local -> skip download.
	fx := &fakeXfer{statSize: 100, downloadFn: func(_, _ string) error {
		t.Fatal("download must be skipped when local size matches remote size")
		return nil
	}}
	if err := transferOne(fx, Item{Direction: "download", Src: "remote/x", Dst: dst}); err != nil {
		t.Fatalf("transferOne error: %v", err)
	}
	if fx.downloads != 0 {
		t.Errorf("expected 0 downloads on resume, got %d", fx.downloads)
	}

	// Sizes differ -> download happens.
	fx2 := &fakeXfer{statSize: 40}
	if err := transferOne(fx2, Item{Direction: "download", Src: "remote/x", Dst: dst}); err != nil {
		t.Fatalf("transferOne error: %v", err)
	}
	if fx2.downloads != 1 {
		t.Errorf("expected 1 download when sizes differ, got %d", fx2.downloads)
	}
}

// TestExpandUploadWalk verifies directory expansion walks recursively and
// preserves relative destination paths joined under Dst.
func TestExpandUploadWalk(t *testing.T) {
	src := t.TempDir()
	os.MkdirAll(filepath.Join(src, "sub"), 0755)
	os.WriteFile(filepath.Join(src, "a.txt"), []byte("a"), 0644)
	os.WriteFile(filepath.Join(src, "sub", "b.txt"), []byte("b"), 0644)

	got := expand([]Item{{Direction: "upload", Src: src, Dst: "remote"}}, &fakeXfer{})
	if len(got) != 2 {
		t.Fatalf("expected 2 expanded items, got %d: %+v", len(got), got)
	}
	dsts := map[string]bool{}
	for _, it := range got {
		dsts[it.Dst] = true
		if it.Direction != "upload" {
			t.Errorf("expanded item direction = %q want upload", it.Direction)
		}
	}
	if !dsts["remote/a.txt"] || !dsts["remote/sub/b.txt"] {
		t.Errorf("unexpected expanded destinations: %v", dsts)
	}
}

// TestExpandDownloadRecursive verifies the remote directory recursion builds
// one download item per remote file entry.
func TestExpandDownloadRecursive(t *testing.T) {
	fx := &fakeXfer{
		isDir: true,
		listRet: []transfer.FileEntry{
			{Name: "f.txt", Path: "/d/f.txt", IsDir: false},
			{Name: "g.txt", Path: "/d/g.txt", IsDir: false},
		},
	}
	got := expand([]Item{{Direction: "download", Src: "/d", Dst: "local"}}, fx)
	if len(got) != 2 {
		t.Fatalf("expected 2 download items, got %d: %+v", len(got), got)
	}
	for _, it := range got {
		if it.Direction != "download" {
			t.Errorf("item direction = %q want download", it.Direction)
		}
	}
}

// TestArchiveRoundtrip verifies ArchiveDir produces a gzip tar that
// ExtractArchive can restore, and that the archive is standard gzip/tar and
// preserves file contents and relative structure.
func TestArchiveRoundtrip(t *testing.T) {
	src := t.TempDir()
	os.MkdirAll(filepath.Join(src, "nested"), 0755)
	os.WriteFile(filepath.Join(src, "a.bin"), []byte("hello"), 0644)
	os.WriteFile(filepath.Join(src, "nested", "b.bin"), []byte("world!!"), 0644)

	arc := filepath.Join(t.TempDir(), "out.tar.gz")
	if err := ArchiveDir(src, arc); err != nil {
		t.Fatalf("ArchiveDir error: %v", err)
	}

	// The archive must be a valid gzip stream (magic 0x1f 0x8b).
	raw, err := os.ReadFile(arc)
	if err != nil {
		t.Fatalf("read archive: %v", err)
	}
	if len(raw) < 2 || raw[0] != 0x1f || raw[1] != 0x8b {
		t.Fatal("archive is not gzip-compressed (missing gzip magic)")
	}

	// It must be readable by the standard library tar reader (POSIX tar).
	gr, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("gzip reader: %v", err)
	}
	tr := tar.NewReader(gr)
	names := map[string]bool{}
	for {
		hdr, err := tr.Next()
		if err != nil {
			break
		}
		names[hdr.Name] = true
	}
	if !names["a.bin"] || !names["nested/b.bin"] {
		t.Errorf("archive entries missing: %v", names)
	}

	// Extract and verify contents match.
	dst := t.TempDir()
	if err := ExtractArchive(arc, dst); err != nil {
		t.Fatalf("ExtractArchive error: %v", err)
	}
	check := func(rel, want string) {
		got, err := os.ReadFile(filepath.Join(dst, rel))
		if err != nil {
			t.Errorf("read extracted %s: %v", rel, err)
			return
		}
		if string(got) != want {
			t.Errorf("extracted %s = %q want %q", rel, got, want)
		}
	}
	check("a.bin", "hello")
	check("nested/b.bin", "world!!")
}

// TestWithin guards the path-traversal protection used by ExtractArchive.
func TestWithin(t *testing.T) {
	if !within("/tmp/x", "/tmp/x/y") {
		t.Error("/tmp/x/y should be within /tmp/x")
	}
	if within("/tmp/x", "/tmp/x/../z") {
		t.Error("/tmp/x/../z should NOT be within /tmp/x")
	}
	if within("/tmp/x", "/etc/passwd") {
		t.Error("/etc/passwd should NOT be within /tmp/x")
	}
}
