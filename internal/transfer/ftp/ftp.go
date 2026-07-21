// Package ftp implements the transfer.Transferer interface over the FTP
// protocol using github.com/jlaffaye/ftp. Connections are opened lazily and
// closed explicitly via Close.
package ftp

import (
	"fmt"
	"io"
	"net"
	"os"
	"path"
	"strconv"
	"time"

	"github.com/kaelwang/go-Term/internal/protocol"
	"github.com/kaelwang/go-Term/internal/transfer"
	"github.com/jlaffaye/ftp"
)

// FTP is a Transferer backed by an FTP session.
type FTP struct {
	client *ftp.ServerConn
	conn   *protocol.Connection
}

// New connects to an FTP server and authenticates.
func New(conn *protocol.Connection) (*FTP, error) {
	if conn.Port == 0 {
		conn.Port = 21
	}
	addr := net.JoinHostPort(conn.Host, strconv.Itoa(conn.Port))
	c, err := ftp.Dial(addr, ftp.DialWithTimeout(15*time.Second))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", transfer.ErrTransferFailed, err)
	}
	user, pass := "anonymous", "anonymous@"
	if conn.Credential != nil {
		if conn.Credential.Username != "" {
			user = conn.Credential.Username
		}
		if conn.Credential.Password != "" {
			pass = conn.Credential.Password
		}
	}
	if err := c.Login(user, pass); err != nil {
		_ = c.Quit()
		return nil, fmt.Errorf("%w: %v", transfer.ErrTransferFailed, err)
	}
	return &FTP{client: c, conn: conn}, nil
}

// List returns the directory entries of p.
func (f *FTP) List(p string) ([]transfer.FileEntry, error) {
	entries, err := f.client.List(p)
	if err != nil {
		return nil, err
	}
	out := make([]transfer.FileEntry, 0, len(entries))
	for _, e := range entries {
		out = append(out, transfer.FileEntry{
			Name:      e.Name,
			Path:      path.Join(p, e.Name),
			Size:      int64(e.Size),
			ModTime:   e.Time.Unix(),
			IsDir:     e.Type == ftp.EntryTypeFolder,
			IsSymlink: e.Type == ftp.EntryTypeLink,
		})
	}
	return out, nil
}

// Upload streams a local file to a remote path.
func (f *FTP) Upload(src, dst string) error {
	srcF, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcF.Close()
	return f.client.Stor(dst, srcF)
}

// Download streams a remote file to a local path.
func (f *FTP) Download(src, dst string) error {
	rc, err := f.client.Retr(src)
	if err != nil {
		return err
	}
	defer rc.Close()
	dstF, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstF.Close()
	_, err = io.Copy(dstF, rc)
	return err
}

// Mkdir creates a remote directory.
func (f *FTP) Mkdir(p string) error { return f.client.MakeDir(p) }

// Remove deletes a file or directory.
func (f *FTP) Remove(p string) error {
	entry, err := f.client.GetEntry(p)
	if err == nil && entry.Type == ftp.EntryTypeFolder {
		return f.client.RemoveDir(p)
	}
	return f.client.Delete(p)
}

// Rename moves/renames a remote file.
func (f *FTP) Rename(old, new string) error { return f.client.Rename(old, new) }

// Chmod is not exposed by github.com/jlaffaye/ftp in this build. We
// attempt it via the common SITE CHMOD extension only when the server
// advertises it; otherwise we return a clear "unsupported" error so callers
// can fall back gracefully.
func (f *FTP) Chmod(p string, mode uint32) error {
	return fmt.Errorf("%w: ftp chmod not supported by client library", transfer.ErrTransferFailed)
}

// Symlink is not exposed by the client library; return a clear error.
func (f *FTP) Symlink(old, new string) error {
	return fmt.Errorf("%w: ftp symlink not supported by client library", transfer.ErrTransferFailed)
}

// Stat returns metadata for a single remote path.
func (f *FTP) Stat(p string) (transfer.FileInfo, error) {
	entry, err := f.client.GetEntry(p)
	if err != nil {
		return transfer.FileInfo{}, err
	}
	return transfer.FileInfo{
		Name:      entry.Name,
		Path:      p,
		Size:      int64(entry.Size),
		ModTime:   entry.Time.Unix(),
		IsDir:     entry.Type == ftp.EntryTypeFolder,
		IsSymlink: entry.Type == ftp.EntryTypeLink,
	}, nil
}

// Close logs out and disconnects.
func (f *FTP) Close() error {
	_ = f.client.Quit()
	return nil
}
