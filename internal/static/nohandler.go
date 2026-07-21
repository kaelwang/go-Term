//go:build !embedstatic

package static

import "github.com/gin-gonic/gin"

// Register is a no-op in the dev build (compiled without the `embedstatic`
// tag). During development the frontend is served separately by the Vite dev
// server, so no static handler is installed on the gin engine.
func Register(r *gin.Engine) {}
