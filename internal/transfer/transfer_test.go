package transfer

import (
	"errors"
	"sync"
	"testing"
)

// TestFileInfoToEntryRoundtrip verifies the conversion between FileInfo and
// FileEntry preserves every field (same layout by contract).
func TestFileInfoToEntryRoundtrip(t *testing.T) {
	fi := FileInfo{
		Name:      "a.txt",
		Path:      "/x/a.txt",
		Size:      1234,
		Mode:      0644,
		ModTime:   1700000000,
		IsDir:     false,
		IsSymlink: true,
	}
	e := fi.ToEntry()
	if e.Name != fi.Name || e.Path != fi.Path || e.Size != fi.Size ||
		e.Mode != fi.Mode || e.ModTime != fi.ModTime || e.IsDir != fi.IsDir ||
		e.IsSymlink != fi.IsSymlink {
		t.Fatalf("ToEntry lost fields: %+v -> %+v", fi, e)
	}

	back := e.ToInfo()
	if back != fi {
		t.Fatalf("ToInfo roundtrip mismatch: %+v", back)
	}
}

// TestTransferTaskAdd verifies Add accumulates transferred bytes and fires the
// progress callback with correct done/total.
func TestTransferTaskAdd(t *testing.T) {
	task := &TransferTask{ID: "t1", Total: 100, Status: "running"}
	var mu sync.Mutex
	var calls [][2]int64
	task.SetProgress(func(done, total int64) {
		mu.Lock()
		calls = append(calls, [2]int64{done, total})
		mu.Unlock()
	})

	task.Add(40)
	task.Add(60)

	if task.Transferred != 100 {
		t.Errorf("Transferred = %d want 100", task.Transferred)
	}
	if len(calls) != 2 {
		t.Fatalf("progress callback called %d times, want 2", len(calls))
	}
	if calls[1][0] != 100 || calls[1][1] != 100 {
		t.Errorf("final progress = %v want [100 100]", calls[1])
	}
}

// TestTransferTaskFail verifies Fail marks the task errored with a message.
func TestTransferTaskFail(t *testing.T) {
	task := &TransferTask{ID: "t2", Status: "running"}
	task.Fail(errors.New("boom"))
	if task.Status != "error" {
		t.Errorf("Status = %q want error", task.Status)
	}
	if task.Error != "boom" {
		t.Errorf("Error = %q want boom", task.Error)
	}
}

// TestErrTransferFailedSentinel verifies the shared error sentinel exists.
func TestErrTransferFailedSentinel(t *testing.T) {
	if ErrTransferFailed == nil {
		t.Fatal("ErrTransferFailed should be a non-nil sentinel")
	}
	if ErrTransferFailed.Error() != "transfer failed" {
		t.Errorf("unexpected sentinel message: %q", ErrTransferFailed.Error())
	}
}
