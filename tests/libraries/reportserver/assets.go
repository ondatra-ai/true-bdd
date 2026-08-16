package reportserver

import (
	"embed"
	"io/fs"
	"net/http"
)

// webAssets is the single-page UI.
//
// Embedded rather than served from disk so the binary runs from anywhere
// and there is no build step: no npm, no bundler, no node_modules inside
// a Go-only test tree.
//
//go:embed web
var webAssets embed.FS

// assetHandler serves the embedded UI at the root.
//
// Explicitly uncacheable. Embedded files carry a zero modification time,
// so http.FileServer sends no Last-Modified and browsers fall back to
// heuristic caching — which means a rebuilt binary keeps serving the old
// page until someone thinks to hard-reload. For a local dev tool that is
// pure confusion, and there is nothing to gain from caching a few KB
// off loopback.
func assetHandler() http.Handler {
	sub, err := fs.Sub(webAssets, "web")
	if err != nil {
		// Unreachable: the embed directive fixes the tree at compile time.
		panic(err)
	}

	files := http.FileServer(http.FS(sub))

	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Cache-Control", "no-store, must-revalidate")
		files.ServeHTTP(writer, request)
	})
}
