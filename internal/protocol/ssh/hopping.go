package ssh

import (
	"net"
	"strconv"
	"time"

	"github.com/kaelwang/go-Term/internal/protocol"
	cryptossh "golang.org/x/crypto/ssh"
)

// hopNode is one stage in a connection chain.
type hopNode struct {
	host string
	port int
	user string
	cfg  *cryptossh.ClientConfig
}

// hopClientConfig builds a ClientConfig for a single jump host.
func hopClientConfig(h *protocol.HopConfig, base *protocol.Connection) *cryptossh.ClientConfig {
	auth, _ := hopAuth(h.Username, h.Password, h.PrivateKey, h.Passphrase, h.UseAgent)
	timeout := base.Timeout
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	return &cryptossh.ClientConfig{
		User:            h.Username,
		Auth:            auth,
		HostKeyCallback: makeHostKeyCallback(base),
		Timeout:         timeout,
	}
}

// proxyClientConfig builds a ClientConfig for a single ProxyConfig jump host.
func proxyClientConfig(p *protocol.ProxyConfig, base *protocol.Connection) *cryptossh.ClientConfig {
	auth, _ := hopAuth(p.Username, p.Password, p.PrivateKey, p.Passphrase, p.UseAgent)
	timeout := base.Timeout
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	return &cryptossh.ClientConfig{
		User:            p.Username,
		Auth:            auth,
		HostKeyCallback: makeHostKeyCallback(base),
		Timeout:         timeout,
	}
}

// dialProxy connects to the target through a single SSH proxy/jump host.
func dialProxy(conn *protocol.Connection, cfg *cryptossh.ClientConfig) (*cryptossh.Client, error) {
	p := conn.Proxy
	pcfg := proxyClientConfig(p, conn)
	paddr := net.JoinHostPort(p.Host, strconv.Itoa(p.Port))
	pclient, err := cryptossh.Dial("tcp", paddr, pcfg)
	if err != nil {
		return nil, err
	}
	taddr := net.JoinHostPort(conn.Host, strconv.Itoa(conn.Port))
	// Establish a TCP connection to the target *through* the proxy.
	nconn, err := pclient.Dial("tcp", taddr)
	if err != nil {
		_ = pclient.Close()
		return nil, err
	}
	// Perform a fresh SSH handshake to the target over that tunneled connection.
	cc, chans, reqs, err := cryptossh.NewClientConn(nconn, taddr, cfg)
	if err != nil {
		_ = nconn.Close()
		_ = pclient.Close()
		return nil, err
	}
	tclient := cryptossh.NewClient(cc, chans, reqs)
	// Keep the proxy client alive as a hidden dependency of the target session.
	return tclient, nil
}

// dialHops connects through an ordered chain of jump hosts, finally reaching
// the target with the target's own credentials.
func dialHops(conn *protocol.Connection, cfg *cryptossh.ClientConfig) (*cryptossh.Client, error) {
	nodes := make([]*hopNode, 0, len(conn.Hops)+1)
	for _, h := range conn.Hops {
		nodes = append(nodes, &hopNode{
			host: h.Host,
			port: h.Port,
			user: h.Username,
			cfg:  hopClientConfig(h, conn),
		})
	}
	nodes = append(nodes, &hopNode{
		host: conn.Host,
		port: conn.Port,
		user: cfg.User,
		cfg:  cfg,
	})

	first := nodes[0]
	addr := net.JoinHostPort(first.host, strconv.Itoa(first.port))
	c0, err := cryptossh.Dial("tcp", addr, first.cfg)
	if err != nil {
		return nil, err
	}
	clients := []*cryptossh.Client{c0}
	prev := c0

	for _, n := range nodes[1:] {
		naddr := net.JoinHostPort(n.host, strconv.Itoa(n.port))
		nconn, err := prev.Dial("tcp", naddr)
		if err != nil {
			return nil, err
		}
		cc, chans, reqs, err := cryptossh.NewClientConn(nconn, naddr, n.cfg)
		if err != nil {
			_ = nconn.Close()
			return nil, err
		}
		client := cryptossh.NewClient(cc, chans, reqs)
		clients = append(clients, client)
		prev = client
	}

	final := clients[len(clients)-1]
	// The earlier clients are required to keep the tunnel alive; we surface them
	// via the final session so they get closed together.
	extra := clients[:len(clients)-1]
	// Re-wrap the final client's session with the extra clients attached.
	if err := attachExtra(final, extra); err != nil {
		return nil, err
	}
	return final, nil
}

// attachExtra annotates the final client's future sessions with the extra
// (intermediate) clients so they are closed on session teardown. Because
// protocol.Conn is returned from newSession, we stash extras on a registry
// keyed by client pointer.
var extraRegistry = map[*cryptossh.Client][]*cryptossh.Client{}

func attachExtra(final *cryptossh.Client, extra []*cryptossh.Client) error {
	if len(extra) == 0 {
		return nil
	}
	extraRegistry[final] = extra
	return nil
}

// consumeExtra returns and clears any extra clients associated with c.
func consumeExtra(c *cryptossh.Client) []*cryptossh.Client {
	if e, ok := extraRegistry[c]; ok {
		delete(extraRegistry, c)
		return e
	}
	return nil
}
