package security

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// HostKeyResult describes the outcome of a host key verification.
type HostKeyResult struct {
	// Known reports whether the host has an entry in known_hosts.
	Known bool
	// Trusted reports whether the presented key matches the stored entry.
	Trusted bool
}

// expandUserPath expands a leading "~" to the user's home directory.
func expandUserPath(p string) string {
	if p == "~" || strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			if p == "~" {
				return home
			}
			return filepath.Join(home, p[2:])
		}
	}
	return p
}

// CheckHostKey verifies key against the known_hosts file at path. The host
// argument is matched exactly as stored (typically "host" or "[host]:port").
// A missing file is treated as "host unknown, not trusted".
func CheckHostKey(path, host string, key ssh.PublicKey) (HostKeyResult, error) {
	path = expandUserPath(path)
	if path == "" {
		return HostKeyResult{}, fmt.Errorf("empty known_hosts path")
	}
	cb, err := knownhosts.New(path)
	if err != nil {
		if os.IsNotExist(err) {
			return HostKeyResult{Known: false, Trusted: false}, nil
		}
		return HostKeyResult{}, err
	}
	// knownhosts matches purely by the host string we pass.
	khErr := cb(host, &net.TCPAddr{}, key)
	if khErr == nil {
		return HostKeyResult{Known: true, Trusted: true}, nil
	}
	if kErr, ok := khErr.(*knownhosts.KeyError); ok {
		if len(kErr.Want) == 0 {
			// Host present in file but no matching key line -> unknown host.
			return HostKeyResult{Known: false, Trusted: false}, nil
		}
		// Host known, key differs.
		return HostKeyResult{Known: true, Trusted: false}, nil
	}
	return HostKeyResult{Known: true, Trusted: false}, khErr
}

// AddHostKey appends a trusted host key to the known_hosts file, creating the
// file (and parent directories) if necessary.
func AddHostKey(path, host string, key ssh.PublicKey) error {
	path = expandUserPath(path)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	line := knownhosts.Line([]string{host}, key)
	if _, err := f.WriteString(line + "\n"); err != nil {
		return err
	}
	return nil
}

// Fingerprint returns the SHA256 fingerprint (ssh-keygen -lf style) of a key.
func Fingerprint(key ssh.PublicKey) string {
	sum := sha256.Sum256(key.Marshal())
	b64 := base64.StdEncoding.EncodeToString(sum[:])
	// ssh-keygen uses URL-safe base64 without padding.
	b64 = strings.TrimRight(b64, "=")
	b64 = strings.ReplaceAll(b64, "+", "-")
	b64 = strings.ReplaceAll(b64, "/", "_")
	return "SHA256:" + b64
}
