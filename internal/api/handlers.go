package api

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/kaelwang/go-Term/internal/config"
	"github.com/kaelwang/go-Term/internal/gateway"
	"github.com/kaelwang/go-Term/internal/protocol"
	"github.com/kaelwang/go-Term/internal/protocol/ssh"
	"github.com/kaelwang/go-Term/internal/security"
	"github.com/kaelwang/go-Term/internal/transfer"
	transferhttp "github.com/kaelwang/go-Term/internal/transfer/http"
	"github.com/kaelwang/go-Term/internal/transfer/ftp"
	"github.com/kaelwang/go-Term/internal/transfer/sftp"
	cryptossh "golang.org/x/crypto/ssh"
)

// Error codes returned in the unified REST envelope. Kept in sync with
// the frontend ErrorCode (frontend/src/types.ts).
const (
	CodeOK               = 0
	CodeAuthFail         = 1001
	CodePermissionDenied = 1002
	CodeConnFail         = 2001
	CodeHostKey          = 2003
	CodeUnsupported      = 2004
	CodeTransferFail     = 3001
	CodeBadParam         = 4001
)

// apiRegistry is the session registry shared with the WebSocket gateway.
var apiRegistry *gateway.SessionRegistry

// SetRegistry wires the session registry into the API package.
func SetRegistry(r *gateway.SessionRegistry) { apiRegistry = r }

// respond writes the unified REST envelope.
func respond(c *gin.Context, code int, message string, data interface{}) {
	c.JSON(http.StatusOK, gin.H{"code": code, "message": message, "data": data})
}

// buildTransferer constructs a transfer.Transferer for the given kind.
func buildTransferer(kind string, conn *protocol.Connection) (transfer.Transferer, error) {
	switch kind {
	case "ftp":
		return ftp.New(conn)
	case "sftp", "":
		return sftp.New(conn)
	default:
		return nil, fmt.Errorf("unsupported transfer protocol: %s", kind)
	}
}

// apiVersion is reported by /api/public/config. It is set from the internal
// package Version constant when the router is constructed in NewRouter.
var apiVersion = "dev"

// TestTerminalHandler validates connectivity without keeping a session.
func TestTerminalHandler(c *gin.Context) {
	var conn protocol.Connection
	if err := c.ShouldBindJSON(&conn); err != nil {
		respond(c, CodeBadParam,"bad params", nil)
		return
	}
	// 兜底 known_hosts_path：前端通常不传，需回退到全局默认（~/.ssh/known_hosts），
	// 否则 conn.KnownHostsPath 为空，SSH 握手会因 "empty known_hosts path" 失败。
	// 对齐 gateway.applyConnDefaults 与 resolveFileConnCredential 中的处理。
	if conn.KnownHostsPath == "" && config.Global != nil && config.Global.KnownHostsPath != "" {
		conn.KnownHostsPath = config.Global.KnownHostsPath
	}
	proto, err := protocol.GetProtocol(conn.Protocol)
	if err != nil {
		respond(c, CodeUnsupported,"unsupported protocol", nil)
		return
	}
	remote, err := proto.Dial(&conn)
	if err != nil {
		respond(c, CodeConnFail,err.Error(), nil)
		return
	}
	_ = remote.Close()
	respond(c, 0, "ok", gin.H{"connected": true})
}

// LocalShellEnabledHandler reports whether the local terminal is available.
func LocalShellEnabledHandler(c *gin.Context) {
	respond(c, 0, "ok", gin.H{"enabled": !config.Global.DisableLocalTerm})
}

// SSHConfigHostsHandler returns the list of Host aliases declared in the
// server's ~/.ssh/config so the frontend can offer them as a dropdown (F2).
func SSHConfigHostsHandler(c *gin.Context) {
	respond(c, 0, "ok", gin.H{"hosts": ssh.ListSSHConfigHosts()})
}

// TransferUploadHandler accepts a file via multipart upload and stores it in
// the configured UploadDir, returning the server-side temp path that the
// WebSocket transfer layer will later send to the remote (F1 / A2).
func TransferUploadHandler(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		respond(c, CodeBadParam, "bad params", nil)
		return
	}
	dir := config.Global.UploadDir
	_ = os.MkdirAll(dir, 0o755)
	// Store the upload inside a unique per-transfer subdirectory (named with a
	// nanosecond timestamp) using the file's ORIGINAL basename. This keeps
	// concurrent uploads from colliding while ensuring `tsz` — which transmits
	// the basename of the path it is given — sends the original filename to the
	// remote instead of a "tx-<ts>-" prefixed one. (Earlier builds prefixed the
	// filename itself, so the remote received "tx-<ts>-<name>".)
	sub := filepath.Join(dir, fmt.Sprintf("tx-%d", time.Now().UnixNano()))
	if err := os.MkdirAll(sub, 0o755); err != nil {
		respond(c, CodeTransferFail, err.Error(), nil)
		return
	}
	dst := filepath.Join(sub, filepath.Base(file.Filename))
	if err := c.SaveUploadedFile(file, dst); err != nil {
		respond(c, CodeTransferFail, err.Error(), nil)
		return
	}
	respond(c, 0, "ok", gin.H{"path": dst})
}

// TransferFileHandler streams a previously received file back to the browser
// from the configured DownloadDir. The path is confined to DownloadDir to
// prevent path traversal (F1 / A2).
func TransferFileHandler(c *gin.Context) {
	path := c.Query("path")
	if path == "" {
		respond(c, CodeBadParam, "missing path", nil)
		return
	}
	clean := filepath.Clean(path)
	root := filepath.Clean(config.Global.DownloadDir)
	if !strings.HasPrefix(clean, root) {
		respond(c, CodePermissionDenied, "forbidden path", nil)
		return
	}
	if _, err := os.Stat(clean); err != nil {
		respond(c, CodeBadParam, "not found", nil)
		return
	}
	c.File(clean)
}

// TransferBinsHandler reports whether the external trz/tsz binaries are
// available on the server PATH (honoring the GOTERM_*_BIN overrides). The
// frontend uses this to grey-out unsupported transfer protocols (F1 / A1).
func TransferBinsHandler(c *gin.Context) {
	respond(c, 0, "ok", gin.H{
		"trz": binAvailable("trz", "GOTERM_TRZ_BIN"),
		"tsz": binAvailable("tsz", "GOTERM_TSZ_BIN"),
	})
}

// binAvailable reports whether an external transfer binary exists, honoring a
// dedicated env override (A1).
func binAvailable(name, envVar string) bool {
	if v := os.Getenv(envVar); v != "" {
		_, err := os.Stat(v)
		return err == nil
	}
	_, err := exec.LookPath(name)
	return err == nil
}

// DownloadHandler streams a remote directory as a tar.gz archive.
func DownloadHandler(c *gin.Context) {
	var req struct {
		Connection *protocol.Connection `json:"connection"`
		RemotePath string               `json:"remote_path"`
		Transfer   string               `json:"transfer"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Connection == nil {
		respond(c, CodeBadParam,"bad params", nil)
		return
	}
	resolveFileConnCredential(c, req.Connection)
	t, err := buildTransferer(req.Transfer, req.Connection)
	if err != nil {
		respond(c, CodeTransferFail,err.Error(), nil)
		return
	}
	defer t.Close()

	archive := filepath.Join(config.Global.DownloadDir,
		fmt.Sprintf("dl-%d.tar.gz", time.Now().UnixNano()))
	if err := transferhttp.DownloadDirectory(t, req.RemotePath, archive); err != nil {
		respond(c, CodeTransferFail,err.Error(), nil)
		return
	}
	c.File(archive)
}

// UploadHandler accepts a multipart archive and uploads it to the remote.
func UploadHandler(c *gin.Context) {
	var meta struct {
		Connection *protocol.Connection `json:"connection"`
		RemotePath string               `json:"remote_path"`
		Transfer   string               `json:"transfer"`
	}
	if m := c.PostForm("meta"); m != "" {
		_ = json.Unmarshal([]byte(m), &meta)
	}
	file, err := c.FormFile("file")
	if err != nil || meta.Connection == nil {
		respond(c, CodeBadParam,"bad params", nil)
		return
	}
	archive := filepath.Join(config.Global.UploadDir,
		fmt.Sprintf("ul-%d.tar.gz", time.Now().UnixNano()))
	if err := c.SaveUploadedFile(file, archive); err != nil {
		respond(c, CodeTransferFail,err.Error(), nil)
		return
	}
	resolveFileConnCredential(c, meta.Connection)
	t, err := buildTransferer(meta.Transfer, meta.Connection)
	if err != nil {
		respond(c, CodeTransferFail,err.Error(), nil)
		return
	}
	defer t.Close()
	if err := transferhttp.UploadDirectory(t, archive, meta.RemotePath); err != nil {
		respond(c, CodeTransferFail,err.Error(), nil)
		return
	}
	respond(c, 0, "ok", gin.H{"remote_path": meta.RemotePath})
}

// HostKeyHandler lists known_hosts or adds a trusted key.
func HostKeyHandler(c *gin.Context) {
	if c.Request.Method == http.MethodPost {
		var req struct {
			Host string `json:"host"`
			Key  string `json:"key"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			respond(c, CodeBadParam,"bad params", nil)
			return
		}
		pk, _, _, _, err := cryptossh.ParseAuthorizedKey([]byte(req.Key))
		if err != nil {
			respond(c, CodeBadParam,"bad key", nil)
			return
		}
		if err := security.AddHostKey(config.Global.KnownHostsPath, req.Host, pk); err != nil {
			respond(c, CodeTransferFail,err.Error(), nil)
			return
		}
		respond(c, 0, "ok", nil)
		return
	}
	respond(c, 0, "ok", listKnownHosts(config.Global.KnownHostsPath))
}

// ---- file manager endpoints ----

// FileListHandler lists a remote directory.
func FileListHandler(c *gin.Context) {
	var req struct {
		Connection *protocol.Connection `json:"connection"`
		Path       string               `json:"path"`
		Transfer   string               `json:"transfer"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Connection == nil {
		respond(c, CodeBadParam,"bad params", nil)
		return
	}
	resolveFileConnCredential(c, req.Connection)
	t, err := buildTransferer(req.Transfer, req.Connection)
	if err != nil {
		respond(c, CodeTransferFail,err.Error(), nil)
		return
	}
	defer t.Close()
	entries, err := t.List(req.Path)
	if err != nil {
		respond(c, CodeTransferFail,err.Error(), nil)
		return
	}
	respond(c, 0, "ok", entries)
}

// FileDownloadHandler streams a single remote file.
func FileDownloadHandler(c *gin.Context) {
	var req struct {
		Connection *protocol.Connection `json:"connection"`
		Path       string               `json:"path"`
		Transfer   string               `json:"transfer"`
	}
	if c.Query("connection") != "" {
		var conn protocol.Connection
		if err := json.Unmarshal([]byte(c.Query("connection")), &conn); err == nil {
			req.Connection = &conn
			req.Path = c.Query("path")
			req.Transfer = c.Query("transfer")
		}
	} else if err := c.ShouldBindJSON(&req); err != nil || req.Connection == nil {
		respond(c, CodeBadParam,"bad params", nil)
		return
	}
	resolveFileConnCredential(c, req.Connection)
	t, err := buildTransferer(req.Transfer, req.Connection)
	if err != nil {
		respond(c, CodeTransferFail,err.Error(), nil)
		return
	}
	defer t.Close()
	tmp := filepath.Join(config.Global.DownloadDir, fmt.Sprintf("f-%d", time.Now().UnixNano()))
	if err := t.Download(req.Path, tmp); err != nil {
		respond(c, CodeTransferFail,err.Error(), nil)
		return
	}
	defer os.Remove(tmp)
	c.File(tmp)
}

// FileUploadHandler uploads a single file to a remote directory.
func FileUploadHandler(c *gin.Context) {
	var meta struct {
		Connection *protocol.Connection `json:"connection"`
		Path       string               `json:"path"`
		Transfer   string               `json:"transfer"`
	}
	if m := c.PostForm("meta"); m != "" {
		_ = json.Unmarshal([]byte(m), &meta)
	}
	file, err := c.FormFile("file")
	if err != nil || meta.Connection == nil {
		respond(c, CodeBadParam,"bad params", nil)
		return
	}
	tmp := filepath.Join(config.Global.UploadDir, fmt.Sprintf("ul-%d", time.Now().UnixNano()))
	if err := c.SaveUploadedFile(file, tmp); err != nil {
		respond(c, CodeTransferFail,err.Error(), nil)
		return
	}
	defer os.Remove(tmp)
	resolveFileConnCredential(c, meta.Connection)
	t, err := buildTransferer(meta.Transfer, meta.Connection)
	if err != nil {
		respond(c, CodeTransferFail,err.Error(), nil)
		return
	}
	defer t.Close()
	dst := filepath.Join(meta.Path, file.Filename)
	if err := t.Upload(tmp, dst); err != nil {
		respond(c, CodeTransferFail,err.Error(), nil)
		return
	}
	respond(c, 0, "ok", gin.H{"path": dst})
}

// FileMkdirHandler creates a remote directory.
func FileMkdirHandler(c *gin.Context) {
	var req struct {
		Connection *protocol.Connection `json:"connection"`
		Path       string               `json:"path"`
		Transfer   string               `json:"transfer"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Connection == nil {
		respond(c, CodeBadParam,"bad params", nil)
		return
	}
	resolveFileConnCredential(c, req.Connection)
	t, err := buildTransferer(req.Transfer, req.Connection)
	if err != nil {
		respond(c, CodeTransferFail,err.Error(), nil)
		return
	}
	defer t.Close()
	if err := t.Mkdir(req.Path); err != nil {
		respond(c, CodeTransferFail,err.Error(), nil)
		return
	}
	respond(c, 0, "ok", nil)
}

// FileRenameHandler renames a remote file.
func FileRenameHandler(c *gin.Context) {
	var req struct {
		Connection *protocol.Connection `json:"connection"`
		Old        string               `json:"old"`
		New        string               `json:"new"`
		Transfer   string               `json:"transfer"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Connection == nil {
		respond(c, CodeBadParam,"bad params", nil)
		return
	}
	resolveFileConnCredential(c, req.Connection)
	t, err := buildTransferer(req.Transfer, req.Connection)
	if err != nil {
		respond(c, CodeTransferFail,err.Error(), nil)
		return
	}
	defer t.Close()
	if err := t.Rename(req.Old, req.New); err != nil {
		respond(c, CodeTransferFail,err.Error(), nil)
		return
	}
	respond(c, 0, "ok", nil)
}

// FileRemoveHandler deletes a remote file or directory.
func FileRemoveHandler(c *gin.Context) {
	var req struct {
		Connection *protocol.Connection `json:"connection"`
		Path       string               `json:"path"`
		Transfer   string               `json:"transfer"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Connection == nil {
		respond(c, CodeBadParam,"bad params", nil)
		return
	}
	resolveFileConnCredential(c, req.Connection)
	t, err := buildTransferer(req.Transfer, req.Connection)
	if err != nil {
		respond(c, CodeTransferFail,err.Error(), nil)
		return
	}
	defer t.Close()
	if err := t.Remove(req.Path); err != nil {
		respond(c, CodeTransferFail,err.Error(), nil)
		return
	}
	respond(c, 0, "ok", nil)
}

// ---- helpers ----

// resolveFileConnCredential mirrors the WebSocket Connect handler's vault
// credential resolution (gateway/ws.go L103-112). When the connection
// references a saved credential by id and auth is enabled, it decrypts the
// real credential so that file manager SFTP/FTP connections can authenticate.
// Without this, file operations always fail with "authentication failed"
// because the frontend only sends credential_id (not the actual secret).
func resolveFileConnCredential(c *gin.Context, conn *protocol.Connection) {
	if conn == nil || conn.CredentialID == "" || !config.Global.AuthEnabled {
		return
	}
	u := currentUser(c)
	if u == "" {
		return
	}
	if gateway.CredentialResolver == nil {
		return
	}
	cred, err := gateway.CredentialResolver(u, conn.CredentialID)
	if err != nil || cred == nil {
		return
	}
	conn.Credential = cred
	// 兜底 known_hosts_path（对齐 gateway.applyConnDefaults）
	if conn.KnownHostsPath == "" && config.Global.KnownHostsPath != "" {
		conn.KnownHostsPath = config.Global.KnownHostsPath
	}
}

func listKnownHosts(path string) []gin.H {
	path = expandPath(path)
	out := []gin.H{}
	f, err := os.Open(path)
	if err != nil {
		return out
	}
	defer f.Close()
	scanner := bufio.NewReader(f)
	for {
		line, err := scanner.ReadString('\n')
		if len(line) == 0 {
			if err != nil {
				break
			}
		}
		if err != nil && len(line) == 0 {
			break
		}
		trimmed := trimNewline(line)
		if len(trimmed) == 0 || trimmed[0] == '#' {
			if err != nil {
				break
			}
			continue
		}
		key, _, _, _, perr := cryptossh.ParseAuthorizedKey([]byte(trimmed))
		if perr != nil {
			if err != nil {
				break
			}
			continue
		}
		host := trimmed
		if idx := indexSpace(trimmed); idx >= 0 {
			host = trimmed[:idx]
		}
		out = append(out, gin.H{
			"host":       host,
			"fingerprint": security.Fingerprint(key),
		})
		if err != nil {
			break
		}
	}
	return out
}

func trimNewline(s string) string {
	if len(s) > 0 && s[len(s)-1] == '\n' {
		s = s[:len(s)-1]
	}
	if len(s) > 0 && s[len(s)-1] == '\r' {
		s = s[:len(s)-1]
	}
	return s
}

func expandPath(p string) string {
	if len(p) >= 2 && p[0] == '~' && (p[1] == '/' || p[1] == '\\') {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, p[2:])
		}
	}
	return p
}

func indexSpace(s string) int {
	for i := 0; i < len(s); i++ {
		if s[i] == ' ' {
			return i
		}
	}
	return -1
}
