// Package http provides directory-level transfer helpers built on top of any
// transfer.Transferer: download a remote directory as a tar.gz archive and
// upload a local tar.gz archive into a remote directory.
package http

import (
	"os"

	"github.com/kaelwang/go-Term/internal/transfer"
	"github.com/kaelwang/go-Term/internal/transfer/batch"
)

// DownloadDirectory downloads remoteDir recursively into a temporary location
// and packs it into a local tar.gz at localArchive.
func DownloadDirectory(t transfer.Transferer, remoteDir, localArchive string) error {
	tmp, err := os.MkdirTemp("", "webssh-dl-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	items := []batch.Item{{Direction: "download", Src: remoteDir, Dst: tmp}}
	if err := batch.Run(t, items, 16); err != nil {
		return err
	}
	return batch.ArchiveDir(tmp, localArchive)
}

// UploadDirectory extracts a local tar.gz archive and bulk-uploads its
// contents into remoteDir.
func UploadDirectory(t transfer.Transferer, localArchive, remoteDir string) error {
	tmp, err := os.MkdirTemp("", "webssh-ul-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	if err := batch.ExtractArchive(localArchive, tmp); err != nil {
		return err
	}
	items := []batch.Item{{Direction: "upload", Src: tmp, Dst: remoteDir}}
	return batch.Run(t, items, 16)
}
