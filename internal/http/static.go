package httpserver

import (
	"io/fs"
	"mime"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	embedweb "github.com/kkjayakumar/AegIsOps/embed"
)

func staticHandler() http.Handler {
	sub, err := fs.Sub(embedweb.FS, "out")
	if err != nil {
		return http.NotFoundHandler()
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.NotFound(w, r)
			return
		}

		name := resolveStaticPath(sub, strings.TrimPrefix(r.URL.Path, "/"))
		data, err := fs.ReadFile(sub, name)
		if err != nil {
			http.NotFound(w, r)
			return
		}

		if ct := mime.TypeByExtension(filepath.Ext(name)); ct != "" {
			w.Header().Set("Content-Type", ct)
		}
		if r.Method == http.MethodHead {
			w.Header().Set("Content-Length", strconv.Itoa(len(data)))
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	})
}

func resolveStaticPath(sub fs.FS, path string) string {
	if path == "" || path == "index.html" {
		return "index.html"
	}
	if strings.Contains(filepath.Base(path), ".") {
		return path
	}
	if _, err := fs.Stat(sub, path+".html"); err == nil {
		return path + ".html"
	}
	return "index.html"
}

func (s *Server) rootHandler() http.Handler {
	static := staticHandler()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") || r.URL.Path == "/healthz" || r.URL.Path == "/readyz" {
			s.mux.ServeHTTP(w, r)
			return
		}
		static.ServeHTTP(w, r)
	})
}
