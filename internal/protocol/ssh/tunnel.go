package ssh

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strconv"

	"github.com/kaelwang/go-Term/internal/protocol"
	cryptossh "golang.org/x/crypto/ssh"
	"go.uber.org/zap"
)

// startTunnel launches the requested port-forwarding rule(s) in the background
// and returns the listeners it opened so the session can close them on teardown
// (otherwise the listening sockets leak after the SSH connection ends).
func startTunnel(client *cryptossh.Client, cfg *protocol.TunnelConfig) ([]net.Listener, error) {
	switch cfg.Type {
	case "local":
		ln, err := net.Listen("tcp", cfg.LocalAddr)
		if err != nil {
			return nil, err
		}
		go localForward(client, ln, cfg.RemoteAddr)
		return []net.Listener{ln}, nil
	case "remote":
		lr, err := client.Listen("tcp", cfg.RemoteAddr)
		if err != nil {
			return nil, err
		}
		go remoteForward(lr, cfg.LocalAddr)
		return []net.Listener{lr}, nil
	case "dynamic":
		ln, err := net.Listen("tcp", cfg.LocalAddr)
		if err != nil {
			return nil, err
		}
		go dynamicForward(client, ln)
		return []net.Listener{ln}, nil
	default:
		return nil, fmt.Errorf("unsupported tunnel type: %s", cfg.Type)
	}
}

// localForward forwards a local listener to a remote address via the SSH client.
func localForward(client *cryptossh.Client, ln net.Listener, remote string) {
	defer ln.Close()
	for {
		c, err := ln.Accept()
		if err != nil {
			zap.L().Error("local tunnel accept failed", zap.Error(err))
			return
		}
		go func(local net.Conn) {
			defer local.Close()
			remoteConn, err := client.Dial("tcp", remote)
			if err != nil {
				zap.L().Error("local tunnel dial failed", zap.Error(err))
				return
			}
			defer remoteConn.Close()
			link(local, remoteConn)
		}(c)
	}
}

// remoteForward forwards a listener on the remote host back to a local address.
func remoteForward(lr net.Listener, localAddr string) {
	defer lr.Close()
	for {
		c, err := lr.Accept()
		if err != nil {
			zap.L().Error("remote tunnel accept failed", zap.Error(err))
			return
		}
		go func(remote net.Conn) {
			defer remote.Close()
			local, err := net.Dial("tcp", localAddr)
			if err != nil {
				zap.L().Error("remote tunnel dial failed", zap.Error(err))
				return
			}
			defer local.Close()
			link(remote, local)
		}(c)
	}
}

// dynamicForward implements a minimal SOCKS5 proxy over SSH (dynamic port).
func dynamicForward(client *cryptossh.Client, ln net.Listener) {
	defer ln.Close()
	for {
		c, err := ln.Accept()
		if err != nil {
			zap.L().Error("dynamic tunnel accept failed", zap.Error(err))
			return
		}
		go socks5(client, c)
	}
}

// link copies data bidirectionally between two connections until one closes.
func link(a, b net.Conn) {
	done := make(chan struct{}, 2)
	go func() { _, _ = io.Copy(a, b); done <- struct{}{} }()
	go func() { _, _ = io.Copy(b, a); done <- struct{}{} }()
	<-done
	_ = a.Close()
	_ = b.Close()
}

// socks5 handles a single SOCKS5 CONNECT request, tunneling it via SSH.
func socks5(client *cryptossh.Client, conn net.Conn) {
	defer conn.Close()

	// Greeting: VER NMETHODS METHODS...
	hdr := make([]byte, 2)
	if _, err := io.ReadFull(conn, hdr); err != nil {
		return
	}
	if hdr[0] != 5 {
		return
	}
	methods := make([]byte, hdr[1])
	if _, err := io.ReadFull(conn, methods); err != nil {
		return
	}
	// Reply: no authentication required.
	if _, err := conn.Write([]byte{5, 0}); err != nil {
		return
	}

	// Request: VER CMD RSV ATYP DST.ADDR DST.PORT
	req := make([]byte, 4)
	if _, err := io.ReadFull(conn, req); err != nil {
		return
	}
	if req[0] != 5 || req[1] != 1 { // only CONNECT supported
		return
	}
	var host string
	switch req[3] {
	case 1: // IPv4
		ip := make([]byte, 4)
		if _, err := io.ReadFull(conn, ip); err != nil {
			return
		}
		host = net.IP(ip).String()
	case 4: // IPv6
		ip := make([]byte, 16)
		if _, err := io.ReadFull(conn, ip); err != nil {
			return
		}
		host = net.IP(ip).String()
	case 3: // Domain name
		l := make([]byte, 1)
		if _, err := io.ReadFull(conn, l); err != nil {
			return
		}
		name := make([]byte, l[0])
		if _, err := io.ReadFull(conn, name); err != nil {
			return
		}
		host = string(name)
	default:
		return
	}
	pb := make([]byte, 2)
	if _, err := io.ReadFull(conn, pb); err != nil {
		return
	}
	port := int(binary.BigEndian.Uint16(pb))
	target := net.JoinHostPort(host, strconv.Itoa(port))

	remote, err := client.Dial("tcp", target)
	if err != nil {
		// Failure reply.
		_, _ = conn.Write([]byte{5, 1, 0, 1, 0, 0, 0, 0, 0, 0})
		return
	}
	defer remote.Close()
	// Success reply (address bytes are placeholders).
	if _, err := conn.Write([]byte{5, 0, 0, 1, 0, 0, 0, 0, 0, 0}); err != nil {
		return
	}
	link(conn, remote)
}
