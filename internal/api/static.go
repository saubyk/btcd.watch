package api

import (
	"io/fs"
	"net/http"
	"os"
	"strings"

	"btcdwatch.com/web"
)

// StaticHandler serves the SPA: the embedded web/dist by default, or an
// on-disk directory when staticDir overrides it. Any path that is not a
// real file falls back to index.html so shareable URLs like /?q=<txid>
// work on cold load.
//
// When the SPA has not been built yet (embed contains only the .keep
// placeholder), a plain-text notice is served instead of broken pages.
func StaticHandler(staticDir string) (http.Handler, error) {
	var fsys fs.FS
	if staticDir != "" {
		fsys = os.DirFS(staticDir)
	} else {
		sub, err := fs.Sub(web.Dist, "dist")
		if err != nil {
			return nil, err
		}
		fsys = sub
	}

	if _, err := fs.Stat(fsys, "index.html"); err != nil {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "btcdwatch: frontend not built into this binary "+
				"(run `make build`, or pass --static-dir). The API is "+
				"available under /api/.", http.StatusNotFound)
		}), nil
	}

	fileServer := http.FileServerFS(fsys)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(r.URL.Path, "/")
		if p != "" && p != "index.html" {
			if _, err := fs.Stat(fsys, p); err == nil {
				// Vite content-hashes everything under assets/, so those
				// URLs never change meaning — cache them forever.
				if strings.HasPrefix(p, "assets/") {
					w.Header().Set("Cache-Control",
						"public, max-age=31536000, immutable")
				}
				fileServer.ServeHTTP(w, r)
				return
			}
		}
		// SPA fallback: unknown paths render the app shell. The shell must
		// revalidate on every load or deploys wouldn't propagate.
		w.Header().Set("Cache-Control", "no-cache")
		http.ServeFileFS(w, r, fsys, "index.html")
	}), nil
}
