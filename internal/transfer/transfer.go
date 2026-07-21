// Package transfer defines the uniform file-transfer interface (Transferer) and
// the shared data structures used by SFTP, FTP, batch and trzsz.
package transfer

import (
	"errors"
	"sync"
)

// ErrTransferFailed is the generic transfer error sentinel.
var ErrTransferFailed = errors.New("transfer failed")

// FileEntry describes a remote file or directory returned by List.
type FileEntry struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	Size      int64  `json:"size"`
	Mode      uint32 `json:"mode"`
	ModTime   int64  `json:"mod_time"`
	IsDir     bool   `json:"is_dir"`
	IsSymlink bool   `json:"is_symlink"`
}

// FileInfo is the result of a single Stat call.
type FileInfo struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	Size      int64  `json:"size"`
	Mode      uint32 `json:"mode"`
	ModTime   int64  `json:"mod_time"`
	IsDir     bool   `json:"is_dir"`
	IsSymlink bool   `json:"is_symlink"`
}

// ToEntry converts a FileInfo to a FileEntry (same field layout).
func (f FileInfo) ToEntry() FileEntry {
	return FileEntry{
		Name:      f.Name,
		Path:      f.Path,
		Size:      f.Size,
		Mode:      f.Mode,
		ModTime:   f.ModTime,
		IsDir:     f.IsDir,
		IsSymlink: f.IsSymlink,
	}
}

// ToInfo converts a FileEntry to a FileInfo.
func (e FileEntry) ToInfo() FileInfo {
	return FileInfo{
		Name:      e.Name,
		Path:      e.Path,
		Size:      e.Size,
		Mode:      e.Mode,
		ModTime:   e.ModTime,
		IsDir:     e.IsDir,
		IsSymlink: e.IsSymlink,
	}
}

// Transferer is the uniform file-transfer interface.
type Transferer interface {
	List(path string) ([]FileEntry, error)
	Upload(src, dst string) error
	Download(src, dst string) error
	Mkdir(path string) error
	Remove(path string) error
	Rename(old, new string) error
	Chmod(path string, mode uint32) error
	Stat(path string) (FileInfo, error)
	Symlink(old, new string) error
	Close() error
}

// ProgressFunc reports transferred/total bytes.
type ProgressFunc func(done, total int64)

// TransferTask is a single tracked transfer with progress.
type TransferTask struct {
	ID          string `json:"id"`
	Kind        string `json:"kind"`
	LocalPath   string `json:"local_path"`
	RemotePath  string `json:"remote_path"`
	Total       int64  `json:"total"`
	Transferred int64  `json:"transferred"`
	Status      string `json:"status"`
	Error       string `json:"error"`

	onProgress ProgressFunc
	mu         sync.Mutex
}

// SetProgress registers a progress callback.
func (t *TransferTask) SetProgress(f ProgressFunc) { t.onProgress = f }

// Add updates transferred bytes and fires the progress callback.
func (t *TransferTask) Add(n int64) {
	t.mu.Lock()
	t.Transferred += n
	done, total := t.Transferred, t.Total
	t.mu.Unlock()
	if t.onProgress != nil {
		t.onProgress(done, total)
	}
}

// Fail marks the task failed.
func (t *TransferTask) Fail(err error) {
	t.mu.Lock()
	t.Status = "error"
	t.Error = err.Error()
	t.mu.Unlock()
}
