package reportserver

import (
	"embed"
	"io/fs"
	"net/http"
)

// webAssets is the single-page UI, embedded so the binary needs no build step.
//
//go:embed web
var webAssets embed.FS

// assetHandler serves the embedded UI at the root, explicitly uncacheable:
// embedded files carry a zero mod time, so browsers fall back to heuristic
// caching and a rebuilt binary keeps serving the old page otherwise.
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
