package gateway

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/kaelwang/go-Term/internal/config"
	"github.com/kaelwang/go-Term/internal/transfer/trzsz"
	"go.uber.org/zap"
)

// transferPayload is the parsed body of a WS "transfer" message.
type transferPayload struct {
	Protocol  string `json:"protocol"`
	Direction string `json:"direction"`
	File      string `json:"file"`
}

// RunTransfer dispatches a file transfer request with exclusive ownership of
// the session Conn (see RunExclusive). Progress is reported to the client via
// the "transfer_status" WebSocket envelope (F1 / A4). It never blocks the
// pump because it is invoked from a dedicated goroutine in ReadPump.
func (s *Session) RunTransfer(protocol, direction, file string) {
	s.RunExclusive(func() {
		s.emitTransferStatus(protocol, direction, "running", "", "")
		outPath, err := s.runTransfer(protocol, direction, file)
		if err != nil {
			zap.L().Error("transfer failed",
				zap.String("protocol", protocol),
				zap.String("direction", direction),
				zap.Error(err))
			s.emitTransferStatus(protocol, direction, "error", err.Error(), "")
			return
		}
		s.emitTransferStatus(protocol, direction, "done", "", outPath)
	})
}

// runTransfer branches on the transfer direction.
func (s *Session) runTransfer(protocol, direction, file string) (string, error) {
	switch direction {
	case "send":
		return s.runTransferSend(protocol, file)
	case "recv":
		return s.runTransferRecv(protocol, file)
	default:
		return "", fmt.Errorf("unknown transfer direction: %s", direction)
	}
}

// runTransferSend streams a previously uploaded file to the remote. For
// trzsz the external tool reads the file directly from the server temp path
// (F1 / A1 / A2).
func (s *Session) runTransferSend(protocol, file string) (string, error) {
	switch protocol {
	case "trzsz":
		if err := trzsz.Send(s.Conn, file); err != nil {
			return "", err
		}
		return file, nil
	default:
		return "", fmt.Errorf("unsupported transfer protocol: %s", protocol)
	}
}

// runTransferRecv receives one or more files from the remote. trzsz runs the
// external tool inside a dedicated subdirectory and the resulting file path is
// reported back (F1 / A2).
func (s *Session) runTransferRecv(protocol, file string) (string, error) {
	// Guard against a nil global config so we return a clear error instead of
	// panicking on DownloadDir dereference (B6 / T-NIL).
	if config.Global == nil {
		return "", fmt.Errorf("config not loaded")
	}
	dir := config.Global.DownloadDir
	if dir == "" {
		return "", fmt.Errorf("download dir not configured")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	switch protocol {
	case "trzsz":
		return runRecvToDir(dir, func(d string) error { return trzsz.Recv(s.Conn, d) })
	default:
		return "", fmt.Errorf("unsupported transfer protocol: %s", protocol)
	}
}

// runRecvToDir runs an external recv tool inside a dedicated per-transfer
// subdirectory so the resulting file(s) can be identified unambiguously, then
// returns the path of the first regular file written (F1 / A2).
func runRecvToDir(base string, run func(dir string) error) (string, error) {
	sub := filepath.Join(base, fmt.Sprintf("recv-%d", time.Now().UnixNano()))
	if err := os.MkdirAll(sub, 0o755); err != nil {
		return "", err
	}
	if err := run(sub); err != nil {
		return "", err
	}
	return firstFileInDir(sub), nil
}

// firstFileInDir returns the path of the first regular file found in dir, or
// dir itself when it contains no regular files.
func firstFileInDir(dir string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return dir
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		return filepath.Join(dir, e.Name())
	}
	return dir
}

// emitTransferStatus sends a "transfer_status" envelope to the client.
func (s *Session) emitTransferStatus(protocol, direction, status, errText, path string) {
	payload, err := json.Marshal(map[string]string{
		"protocol":  protocol,
		"direction": direction,
		"status":    status,
		"error":     errText,
		"path":      path,
	})
	if err != nil {
		return
	}
	s.WriteMessage(Message{
		Type:    "transfer_status",
		Session: s.ID,
		Payload: payload,
	})
}
