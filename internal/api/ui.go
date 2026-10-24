package api

import (
	"io/fs"
	"mime"
	"path"
	"strings"

	"github.com/gofiber/fiber/v3"

	"github.com/tecncr/nwm-hri-modbus-panel/web"
)

// mountUI serves the embedded dashboard under basePath. Asset URLs in the
// bundle are relative, so the UI works behind any path prefix as long as it
// is reached with a trailing slash.
func (s *Server) mountUI(app *fiber.App, basePath string) {
	dist := web.Dist()
	index, err := fs.ReadFile(dist, "index.html")
	if err != nil {
		s.Log.Warn("dashboard bundle missing; UI disabled", "err", err)
		return
	}
	serveIndex := func(c fiber.Ctx) error {
		c.Set(fiber.HeaderCacheControl, "no-cache")
		c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)
		return c.Send(index)
	}
	if basePath != "" {
		// Non-strict routing also sends "/base/" here, so only redirect the
		// slash-less form. The relative target keeps working behind proxies
		// that add further prefixes.
		app.Get(basePath, func(c fiber.Ctx) error {
			if strings.HasSuffix(string(c.Request().URI().Path()), "/") {
				return serveIndex(c)
			}
			target := path.Base(basePath) + "/"
			if q := string(c.Request().URI().QueryString()); q != "" {
				target += "?" + q
			}
			return c.Redirect().Status(fiber.StatusMovedPermanently).To(target)
		})
	}
	app.Get(basePath+"/*", func(c fiber.Ctx) error {
		if c.Method() != fiber.MethodGet && c.Method() != fiber.MethodHead {
			return fiber.ErrMethodNotAllowed
		}
		rel := strings.TrimPrefix(path.Clean("/"+c.Params("*")), "/")
		if rel == "" || rel == "index.html" {
			return serveIndex(c)
		}
		if strings.HasPrefix(rel, "api/") {
			return fiber.NewError(fiber.StatusNotFound, "no such endpoint")
		}
		data, err := fs.ReadFile(dist, rel)
		if err != nil {
			// Unknown paths fall back to the app shell (client-side routing).
			return serveIndex(c)
		}
		if ct := mime.TypeByExtension(path.Ext(rel)); ct != "" {
			c.Set(fiber.HeaderContentType, ct)
		}
		if strings.HasPrefix(rel, "assets/") {
			// Vite fingerprints asset file names.
			c.Set(fiber.HeaderCacheControl, "public, max-age=31536000, immutable")
		} else {
			c.Set(fiber.HeaderCacheControl, "public, max-age=3600")
		}
		return c.Send(data)
	})
}
