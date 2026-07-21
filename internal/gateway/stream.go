package gateway

import (
	"encoding/base64"
	"encoding/json"
	"time"

	"go.uber.org/zap"
)

// WritePump copies output from the remote Conn into WebSocket data messages.
// During normal operation Conn.Read/Writes are safe concurrently (SSH stdin
// and stdout are independent channels). During a file transfer the pump
// pauses (checked via the transferring atomic) so the transfer goroutine
// (RunExclusive) can take exclusive ownership of Conn without a mutex fight.
func WritePump(s *Session) {
	buf := make([]byte, 32*1024)

	for {
		if s.transferring.Load() {
			time.Sleep(50 * time.Millisecond)
			continue
		}
		n, err := s.Conn.Read(buf)
		if n > 0 {
			chunk := buf[:n]
			s.WriteOutput(chunk)
		}
		if err != nil {
			_ = s.ws.WriteJSON(Message{Type: "close", Session: s.ID})
			s.Close()
			return
		}
	}
}

// ReadPump reads WebSocket messages and forwards them to the remote Conn.
func ReadPump(s *Session) {
	zap.L().Info("ws: ReadPump started", zap.String("session", s.ID))
	for {
		_, data, err := s.ws.ReadMessage()
		if err != nil {
			zap.L().Warn("ws: ReadPump read error",
				zap.String("session", s.ID),
				zap.Error(err))
			s.Close()
			return
		}
		zap.L().Info("ws: ReadPump got message",
			zap.String("session", s.ID),
			zap.Int("len", len(data)))
		var msg Message
		if err := json.Unmarshal(data, &msg); err != nil {
			zap.L().Warn("ws: bad message", zap.Error(err))
			continue
		}
		switch msg.Type {
		case "input", "data":
			// Skip user input while a transfer owns the Conn.
			if s.transferring.Load() {
				continue
			}
			var p struct {
				Data string `json:"data"`
			}
			if err := json.Unmarshal(msg.Payload, &p); err != nil {
				continue
			}
			raw, e := base64.StdEncoding.DecodeString(p.Data)
			if e != nil {
				zap.L().Warn("ws: bad base64 in input",
					zap.String("session", s.ID),
					zap.Error(e))
				continue
			}
			if len(raw) == 0 {
				zap.L().Warn("ws: empty input payload",
					zap.String("session", s.ID))
				continue
			}
			zap.L().Info("ws: input received",
				zap.String("session", s.ID),
				zap.Int("len", len(raw)))
			_, _ = s.Conn.Write(raw)
		case "resize":
			if s.transferring.Load() {
				continue
			}
			var p struct {
				Cols int `json:"cols"`
				Rows int `json:"rows"`
			}
			if err := json.Unmarshal(msg.Payload, &p); err == nil {
				_ = s.Conn.Resize(p.Cols, p.Rows)
			}
		case "hostkey_accept":
			// Interactive host-key acceptance: currently the gateway
			// auto-accepts on first use; this is a no-op hook point.
		case "transfer":
			var p transferPayload
			if err := json.Unmarshal(msg.Payload, &p); err != nil {
				zap.L().Warn("ws: bad transfer payload", zap.Error(err))
				continue
			}
			// Run the transfer on a dedicated goroutine; it takes exclusive
			// ownership of the Conn for its whole duration (F1 / A3).
			go s.RunTransfer(p.Protocol, p.Direction, p.File)
		case "close":
			s.Close()
			return
		}
	}
}
