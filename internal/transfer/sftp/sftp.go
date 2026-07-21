// Package sftp implements the transfer.Transferer interface over an SSH
// connection using the github.com/pkg/sftp client.
package sftp

import (
	"fmt"
	"io"
	"os"
	"path"

	"github.com/kaelwang/go-Term/internal/protocol"
	sshpkg "github.com/kaelwang/go-Term/internal/protocol/ssh"
	"github.com/kaelwang/go-Term/internal/transfer"
	"github.com/pkg/sftp"
	cryptossh "golang.org/x/crypto/ssh"
)

// SFTP is a Transferer backed by an SFTP session.
type SFTP struct {
	client    *sftp.Client
	sshClient *cryptossh.Client
	extra     []*cryptossh.Client
}

// New dials SSH and opens an SFTP client for the given connection.
func New(conn *protocol.Connection) (*SFTP, error) {
	client, extra, err := sshpkg.DialRaw(conn)
	if err != nil {
		return nil, err
	}
	sftpClient, err := sftp.NewClient(client)
	if err != nil {
		_ = client.Close()
		for _, ec := range extra {
			_ = ec.Close()
		}
		return nil, fmt.Errorf("%w: %v", transfer.ErrTransferFailed, err)
	}
	return &SFTP{client: sftpClient, sshClient: client, extra: extra}, nil
}

func toEntry(fi os.FileInfo, p string) transfer.FileEntry {
	return transfer.FileEntry{
		Name:      fi.Name(),
		Path:      p,
		Size:      fi.Size(),
		Mode:      uint32(fi.Mode()),
		ModTime:   fi.ModTime().Unix(),
		IsDir:     fi.IsDir(),
		IsSymlink: fi.Mode()&os.ModeSymlink != 0,
	}
}

// List returns the directory entries of p.
func (s *SFTP) List(p string) ([]transfer.FileEntry, error) {
	entries, err := s.client.ReadDir(p)
	if err != nil {
		return nil, err
	}
	out := make([]transfer.FileEntry, 0, len(entries))
	for _, e := range entries {
		out = append(out, toEntry(e, path.Join(p, e.Name())))
	}
	return out, nil
}

// Upload copies a local file to a remote path.
func (s *SFTP) Upload(src, dst string) error {
	srcF, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcF.Close()
	fi, err := srcF.Stat()
	if err != nil {
		return err
	}
	dstF, err := s.client.Create(dst)
	if err != nil {
		return err
	}
	defer dstF.Close()
	buf := make([]byte, 32*1024)
	if _, err := io.CopyBuffer(dstF, srcF, buf); err != nil {
		return err
	}
	_ = s.client.Chmod(dst, fi.Mode())
	return nil
}

// Download copies a remote file to a local path.
func (s *SFTP) Download(src, dst string) error {
	srcF, err := s.client.Open(src)
	if err != nil {
		return err
	}
	defer srcF.Close()
	dstF, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstF.Close()
	buf := make([]byte, 32*1024)
	_, err = io.CopyBuffer(dstF, srcF, buf)
	return err
}

// Mkdir creates a directory (and parents).
func (s *SFTP) Mkdir(p string) error { return s.client.MkdirAll(p) }

// Remove deletes a file or directory.
func (s *SFTP) Remove(p string) error { return s.client.Remove(p) }

// Rename moves/renames a remote file.
func (s *SFTP) Rename(old, new string) error { return s.client.Rename(old, new) }

// Chmod changes the mode of a remote file.
func (s *SFTP) Chmod(p string, mode uint32) error {
	return s.client.Chmod(p, os.FileMode(mode))
}

// Symlink creates a symbolic link new -> old.
func (s *SFTP) Symlink(old, new string) error { return s.client.Symlink(old, new) }

// Stat returns metadata for a single remote path.
func (s *SFTP) Stat(p string) (transfer.FileInfo, error) {
	fi, err := s.client.Stat(p)
	if err != nil {
		return transfer.FileInfo{}, err
	}
	return toEntry(fi, p).ToInfo(), nil
}

// Close releases the SFTP session and SSH client.
func (s *SFTP) Close() error {
	_ = s.client.Close()
	_ = s.sshClient.Close()
	for _, ec := range s.extra {
		_ = ec.Close()
	}
	return nil
}
