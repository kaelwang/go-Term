//go:build embedstatic

// Package static embeds the compiled frontend SPA into the production binary
// (only built with the `embedstatic` build tag). In dev builds the `!embedstatic`
// variant provides a no-op Register so the frontend is served by Vite instead.
package static

import "embed"

// distFS holds the frontend build output copied from frontend/dist at build time.
//
//go:embed all:dist
var distFS embed.FS
