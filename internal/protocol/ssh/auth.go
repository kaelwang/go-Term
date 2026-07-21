package ssh

import (
	"errors"
	"fmt"
	"net"
	"os"

	"github.com/kaelwang/go-Term/internal/protocol"
	cryptossh "golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

// parsePrivateKey parses a PEM private key, decrypting it when a passphrase
// is supplied.
func parsePrivateKey(pem, passphrase string) (cryptossh.Signer, error) {
	if passphrase != "" {
		return cryptossh.ParsePrivateKeyWithPassphrase([]byte(pem), []byte(passphrase))
	}
	return cryptossh.ParsePrivateKey([]byte(pem))
}

// agentAuth returns a public-key auth method backed by the user's ssh-agent.
func agentAuth() (cryptossh.AuthMethod, error) {
	sock := os.Getenv("SSH_AUTH_SOCK")
	if sock == "" {
		return nil, errors.New("SSH_AUTH_SOCK not set")
	}
	conn, err := net.Dial("unix", sock)
	if err != nil {
		return nil, err
	}
	ag := agent.NewClient(conn)
	return cryptossh.PublicKeysCallback(ag.Signers), nil
}

// authMethods builds the ordered list of SSH auth methods for a credential.
// It supports password, public-key (optionally encrypted), keyboard-interactive
// (used for 2FA / OTP prompts) and ssh-agent.
func authMethods(cred *protocol.Credential, useAgent bool) ([]cryptossh.AuthMethod, error) {
	if cred == nil {
		return nil, protocol.ErrAuthFailed
	}
	var methods []cryptossh.AuthMethod

	if useAgent {
		if am, err := agentAuth(); err == nil {
			methods = append(methods, am)
		}
	}

	if cred.PrivateKey != "" {
		signer, err := parsePrivateKey(cred.PrivateKey, cred.Passphrase)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", protocol.ErrAuthFailed, err)
		}
		methods = append(methods, cryptossh.PublicKeys(signer))
	}

	if cred.Password != "" {
		methods = append(methods, cryptossh.Password(cred.Password))
		// Keyboard-interactive fallback answers prompts with the password and
		// any extra answers (e.g. 2FA tokens) provided on the credential.
		methods = append(methods, cryptossh.KeyboardInteractive(
			func(name, instruction string, questions []string, echos []bool) ([]string, error) {
				answers := make([]string, len(questions))
				for i := range questions {
					if i < len(cred.Answers) {
						answers[i] = cred.Answers[i]
					} else {
						answers[i] = cred.Password
					}
				}
				return answers, nil
			}))
	}

	if len(methods) == 0 {
		return nil, protocol.ErrAuthFailed
	}
	return methods, nil
}

// hopAuth builds auth methods from a hop/proxy credential subset.
func hopAuth(username, password, privateKey, passphrase string, useAgent bool) ([]cryptossh.AuthMethod, error) {
	cred := &protocol.Credential{
		Type:       "publickey",
		Username:   username,
		Password:   password,
		PrivateKey: privateKey,
		Passphrase: passphrase,
	}
	return authMethods(cred, useAgent)
}
