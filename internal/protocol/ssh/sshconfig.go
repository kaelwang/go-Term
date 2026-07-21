package ssh

import (
	"bufio"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// SSHConfigEntry is one parsed "Host" block from ~/.ssh/config.
type SSHConfigEntry struct {
	Host                  string
	HostName              string
	User                  string
	Port                  int
	IdentityFile          string
	ProxyJump             string
	StrictHostKeyChecking string
}

// ParseSSHConfig reads a simplified OpenSSH client config file. It returns a map
// keyed by the Host pattern. Unsupported directives are ignored.
func ParseSSHConfig(path string) map[string]*SSHConfigEntry {
	if path == "" {
		if home, err := os.UserHomeDir(); err == nil {
			path = filepath.Join(home, ".ssh", "config")
		}
	}
	entries := map[string]*SSHConfigEntry{}
	f, err := os.Open(path)
	if err != nil {
		return entries
	}
	defer f.Close()

	var cur *SSHConfigEntry
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		key := strings.ToLower(parts[0])
		val := strings.Join(parts[1:], " ")
		switch key {
		case "host":
			cur = &SSHConfigEntry{Host: val}
			entries[val] = cur
		case "hostname":
			if cur != nil {
				cur.HostName = val
			}
		case "user":
			if cur != nil {
				cur.User = val
			}
		case "port":
			if cur != nil {
				cur.Port, _ = strconv.Atoi(val)
			}
		case "identityfile":
			if cur != nil {
				cur.IdentityFile = expandTilde(val)
			}
		case "proxyjump":
			if cur != nil {
				cur.ProxyJump = val
			}
		case "stricthostkeychecking":
			if cur != nil {
				cur.StrictHostKeyChecking = val
			}
		}
	}
	return entries
}

// ListSSHConfigHosts returns every Host alias declared in ~/.ssh/config,
// including wildcard patterns. The result is sorted for stable display.
func ListSSHConfigHosts() []string {
	entries := ParseSSHConfig("")
	hosts := make([]string, 0, len(entries))
	for host := range entries {
		hosts = append(hosts, host)
	}
	sort.Strings(hosts)
	return hosts
}

// expandTilde expands a leading "~" to the user's home directory.
func expandTilde(p string) string {
	if strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, p[2:])
		}
	}
	return p
}

// ResolveSSHConfig finds the first entry whose Host pattern matches host.
func ResolveSSHConfig(host string) *SSHConfigEntry {
	entries := ParseSSHConfig("")
	for pat, e := range entries {
		if matchGlob(pat, host) {
			return e
		}
	}
	return nil
}

// matchGlob implements a simple wildcard matcher supporting '*' and '?'.
func matchGlob(pattern, s string) bool {
	if pattern == s {
		return true
	}
	if pattern == "*" {
		return true
	}
	// Convert the pattern to a prefix/suffix/substring match when possible.
	if !strings.Contains(pattern, "*") && !strings.Contains(pattern, "?") {
		return pattern == s
	}
	return globMatch(strings.Split(pattern, ""), strings.Split(s, ""))
}

func globMatch(p, s []string) bool {
	i, j := 0, 0
	star := -1
	mark := 0
	for j < len(s) {
		if i < len(p) && (p[i] == "?" || p[i] == s[j]) {
			i++
			j++
		} else if i < len(p) && p[i] == "*" {
			star = i
			mark = j
			i++
		} else if star != -1 {
			i = star + 1
			mark++
			j = mark
		} else {
			return false
		}
	}
	for i < len(p) && p[i] == "*" {
		i++
	}
	return i == len(p)
}
