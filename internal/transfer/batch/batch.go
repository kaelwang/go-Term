// Package batch provides bounded-concurrency, resumable, recursive bulk
// transfers over any transfer.Transferer, plus tar.gz directory packaging.
package batch

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"

	"github.com/kaelwang/go-Term/internal/transfer"
)

// Item is a single unit of work in a batch.
type Item struct {
	// Direction is "upload" (local -> remote) or "download" (remote -> local).
	Direction string
	// Src is the local path for uploads, remote path for downloads.
	Src string
	// Dst is the remote path for uploads, local path for downloads.
	Dst string
}

// Run expands directories, applies resume, and executes all transfers with at
// most max concurrent workers. It returns the first error encountered.
func Run(t transfer.Transferer, items []Item, max int) error {
	if max <= 0 {
		max = 64
	}
	expanded := expand(items, t)

	sem := make(chan struct{}, max)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var firstErr error

	for _, it := range expanded {
		wg.Add(1)
		sem <- struct{}{}
		go func(it Item) {
			defer wg.Done()
			defer func() { <-sem }()
			if err := transferOne(t, it); err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				mu.Unlock()
			}
		}(it)
	}
	wg.Wait()
	return firstErr
}

// expand turns directory items into per-file items.
func expand(items []Item, t transfer.Transferer) []Item {
	var out []Item
	for _, it := range items {
		if it.Direction == "upload" {
			if fi, err := os.Stat(it.Src); err == nil && fi.IsDir() {
				_ = filepath.Walk(it.Src, func(p string, info os.FileInfo, err error) error {
					if err != nil || info.IsDir() {
						return nil
					}
					rel, _ := filepath.Rel(it.Src, p)
					out = append(out, Item{
						Direction: "upload",
						Src:       p,
						Dst:       path.Join(it.Dst, filepath.ToSlash(rel)),
					})
					return nil
				})
			} else {
				out = append(out, it)
			}
		} else { // download
			if fi, err := t.Stat(it.Src); err == nil && fi.IsDir {
				listRecursive(t, it.Src, it.Dst, &out)
			} else {
				out = append(out, it)
			}
		}
	}
	return out
}

func listRecursive(t transfer.Transferer, dir, dstDir string, out *[]Item) {
	entries, err := t.List(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		target := path.Join(dstDir, e.Name)
		if e.IsDir {
			listRecursive(t, e.Path, target, out)
		} else {
			*out = append(*out, Item{Direction: "download", Src: e.Path, Dst: target})
		}
	}
}

// transferOne performs one file transfer honoring resume semantics.
func transferOne(t transfer.Transferer, it Item) error {
	if it.Direction == "upload" {
		if rfi, err := t.Stat(it.Dst); err == nil && rfi.Size == localSize(it.Src) {
			return nil // already transferred
		}
		return t.Upload(it.Src, it.Dst)
	}
	if lfi, err := os.Stat(it.Dst); err == nil && lfi.Size() == remoteSize(t, it.Src) {
		return nil // already downloaded
	}
	return t.Download(it.Src, it.Dst)
}

func localSize(p string) int64 {
	if fi, err := os.Stat(p); err == nil {
		return fi.Size()
	}
	return -1
}

func remoteSize(t transfer.Transferer, p string) int64 {
	if fi, err := t.Stat(p); err == nil {
		return fi.Size
	}
	return -1
}

// ArchiveDir packs a local directory tree into a gzip-compressed tar archive.
func ArchiveDir(srcDir, outArchive string) error {
	out, err := os.Create(outArchive)
	if err != nil {
		return err
	}
	defer out.Close()
	gw := gzip.NewWriter(out)
	defer gw.Close()
	tw := tar.NewWriter(gw)
	defer tw.Close()

	return filepath.Walk(srcDir, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(srcDir, p)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		hdr, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		hdr.Name = filepath.ToSlash(rel)
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		f, err := os.Open(p)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = io.Copy(tw, f)
		return err
	})
}

// ExtractArchive expands a gzip-compressed tar archive into dstDir.
func ExtractArchive(srcArchive, dstDir string) error {
	in, err := os.Open(srcArchive)
	if err != nil {
		return err
	}
	defer in.Close()
	gr, err := gzip.NewReader(in)
	if err != nil {
		return err
	}
	defer gr.Close()
	tr := tar.NewReader(gr)

	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		return err
	}
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		target := filepath.Join(dstDir, filepath.Clean(hdr.Name))
		// Guard against path traversal.
		if !within(dstDir, target) {
			continue
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			f, err := os.Create(target)
			if err != nil {
				return err
			}
			if _, err := io.Copy(f, tr); err != nil {
				_ = f.Close()
				return err
			}
			_ = f.Close()
		}
	}
	return nil
}

func within(root, target string) bool {
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
