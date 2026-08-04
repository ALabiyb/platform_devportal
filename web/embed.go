// ---------------------------------------------------------------------------
// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com
// ---------------------------------------------------------------------------

// Package web embeds the compiled React SPA into the Go binary.
//
// The dist/ directory is produced by running `npm run build` inside web/.
// Until that command is run, only the placeholder web/dist/index.html exists
// and the binary serves that — telling developers how to build the frontend.
//
// In production CI the build order is always:
//
//	cd web && npm ci && npm run build
//	cd ..  && go build ./cmd/devportal
//
// so the binary always contains the full compiled SPA.
package web

import "embed"

// FS is the embedded filesystem containing the compiled React SPA.
// Paths inside FS are prefixed with "dist/" — use fs.Sub(FS, "dist") to
// get a root-relative FS suitable for http.FileServer.
//
//go:embed all:dist
var FS embed.FS
