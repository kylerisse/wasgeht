package server

import (
	"embed"
	"io/fs"
	"net/http"
	"time"
)

//go:embed static/*
var staticFiles embed.FS

//go:embed templates/*
var templateFiles embed.FS

// startAPI registers all HTTP routes and starts the API server in a goroutine.
func (s *Server) startAPI() {
	mux := http.NewServeMux()

	mux.HandleFunc("/api", func(w http.ResponseWriter, r *http.Request) {
		s.handleAPI(w, r)
	})

	mux.HandleFunc("/api/hosts/{hostname}", func(w http.ResponseWriter, r *http.Request) {
		s.handleHostAPI(w, r)
	})

	mux.HandleFunc("/api/summary", func(w http.ResponseWriter, r *http.Request) {
		s.handleSummaryAPI(w, r)
	})

	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		s.handlePrometheus(w, r)
	})

	content, err := fs.Sub(staticFiles, "static")
	if err != nil {
		s.logger.Fatalf("Failed to create sub filesystem: %v", err)
	}

	// Serve generated graphs from the graphDir
	imgFS := http.FileServer(http.Dir(s.graphDir))
	mux.Handle("/imgs/", noCacheMiddleware(imgFS))

	mux.Handle("/host-detail", hostDetailHandler(templateFiles))

	// Serve static content
	htmlFS := http.FileServer(http.FS(content))
	mux.Handle("/", noCacheMiddleware(http.StripPrefix("/", htmlFS)))

	go func() {
		s.logger.Infof("Starting API server on port %v...", s.listenPort)
		srv := &http.Server{
			Addr:              ":" + s.listenPort,
			Handler:           mux,
			ReadHeaderTimeout: 5 * time.Second,
			ReadTimeout:       10 * time.Second,
			WriteTimeout:      30 * time.Second,
			IdleTimeout:       60 * time.Second,
		}
		if err := srv.ListenAndServe(); err != nil {
			s.logger.Fatalf("Failed to start API server: %v", err)
		}
	}()
}
