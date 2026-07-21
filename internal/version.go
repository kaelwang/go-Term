// Package internal wires the configuration, protocols, gateway and REST API
// into a single runnable server.
package internal

// Version is the go-Term release version returned by /api/public/config.
// It can be overridden at build time via -ldflags "-X .../internal.Version=...".
const Version = "1.0.0"
