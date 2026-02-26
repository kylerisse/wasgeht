package server

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed static/*
var staticFiles embed.FS

//go:embed templates/*
var templateFiles embed.FS

// startAPI registers all HTTP routes and starts the API server in a goroutine.
func (s *Server) startAPI() {
	http.HandleFunc("/api", func(w http.ResponseWriter, r *http.Request) {
		s.handleAPI(w, r)
	})

	http.HandleFunc("/api/hosts/{hostname}", func(w http.ResponseWriter, r *http.Request) {
		s.handleHostAPI(w, r)
	})

	http.HandleFunc("/api/summary", func(w http.ResponseWriter, r *http.Request) {
		s.handleSummaryAPI(w, r)
	})

	http.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		s.handlePrometheus(w, r)
	})

	content, err := fs.Sub(staticFiles, "static")
	if err != nil {
		s.logger.Fatalf("Failed to create sub filesystem: %v", err)
	}

	// Serve generated graphs from the graphDir
	imgFS := http.FileServer(http.Dir(s.graphDir))
	http.Handle("/imgs/", noCacheMiddleware(imgFS))

	http.Handle("/host-detail", hostDetailHandler(templateFiles))

	// Serve static content
	htmlFS := http.FileServer(http.FS(content))
	http.Handle("/", noCacheMiddleware(http.StripPrefix("/", htmlFS)))

	go func() {
		s.logger.Infof("Starting API server on port %v...", s.listenPort)
		if err := http.ListenAndServe(":"+s.listenPort, nil); err != nil {
			s.logger.Fatalf("Failed to start API server: %v", err)
		}
	}()
}
