// ---------------------------------------------------------------------------
// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com
// ---------------------------------------------------------------------------

// spa.go serves the embedded React SPA for all non-API routes.
// Known static assets (JS, CSS, images) are served directly from the embedded FS.
// All other paths return index.html so React Router handles client-side navigation.

package handler

import (
	"io/fs"
	"net/http"
	"strings"
)

// spaHandler returns an http.HandlerFunc that serves the React SPA from fsys.
//
// Routing logic:
//   - If the requested path exists in the FS → serve it (JS, CSS, images, fonts).
//   - If the path does not exist → serve dist/index.html (SPA client-side route).
//
// API routes (/api/v1/..., /auth/..., /healthz) are registered first in
// chi's router and are matched before this wildcard handler, so they are
// never intercepted here.
func spaHandler(fsys fs.FS) http.HandlerFunc {
	// Get a sub-FS rooted at "dist" so HTTP paths map directly to file paths.
	dist, err := fs.Sub(fsys, "dist")
	if err != nil {
		// Should never happen — embed.go guarantees "dist" exists.
		panic("spaHandler: cannot sub into dist: " + err.Error())
	}
	fileServer := http.FileServer(http.FS(dist))

	return func(w http.ResponseWriter, r *http.Request) {
		// Clean the path: strip leading slash to get a relative FS path.
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}

		// Attempt to open the file from the embedded FS.
		f, err := dist.Open(path)
		if err != nil {
			// File not found — fall back to serving root so React Router handles routing.
			// Cannot use "/index.html" here: http.FileServer redirects any path ending
			// in "/index.html" to "./" which creates an infinite redirect loop.
			r2 := r.Clone(r.Context())
			r2.URL.Path = "/"
			fileServer.ServeHTTP(w, r2)
			return
		}
		f.Close()

		// File exists — serve it normally (handles caching headers, ranges, etc.).
		fileServer.ServeHTTP(w, r)
	}
}
