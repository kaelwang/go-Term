//go:build embedstatic

package static

import (
	"io/fs"
	"mime"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

// Register installs a catch-all SPA fallback route on the gin engine. It serves
// the embedded frontend build from distFS. Exact asset paths are served
// directly; unknown routes without a file extension fall back to index.html so
// client-side routing works. The /api and /ws routes are matched earlier, so
// they are never swallowed by this handler.
//
// Files are read directly from the embedded FS and written with an explicit
// content type. This deliberately avoids gin.Context.FileFromFS /
// http.FileServer, which 301-redirect (Location: ./) SPA routes when serving
// from an embed.FS because the path handed to http.FileServer ends up empty.
func Register(r *gin.Engine) {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		return
	}
	r.NoRoute(func(c *gin.Context) {
		// Path relative to the embedded dist root (no leading slash).
		reqPath := strings.TrimPrefix(c.Request.URL.Path, "/")
		isRoute := filepath.Ext(reqPath) == ""
		if reqPath == "" {
			reqPath = "index.html"
			isRoute = true
		}

		if isRoute {
			// Extension-less request (client-side route): try the literal
			// asset, then fall back to the SPA entry point.
			if data, rerr := fs.ReadFile(sub, reqPath); rerr == nil {
				serve(c, reqPath, data)
				return
			}
			if data, ferr := fs.ReadFile(sub, "index.html"); ferr == nil {
				c.Data(http.StatusOK, "text/html; charset=utf-8", data)
				return
			}
			c.Status(http.StatusNotFound)
			return
		}

		// Has a file extension: serve it, or 404 if it does not exist.
		data, err := fs.ReadFile(sub, reqPath)
		if err != nil {
			c.Status(http.StatusNotFound)
			return
		}
		serve(c, reqPath, data)
	})
}

// serve writes raw file bytes with a content type derived from the extension.
func serve(c *gin.Context, name string, data []byte) {
	contentType := mime.TypeByExtension(filepath.Ext(name))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	c.Data(http.StatusOK, contentType, data)
}
