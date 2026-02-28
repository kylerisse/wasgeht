package server

import (
	"embed"
	"io/fs"
	"net/http"
	"time"

	"golang.org/x/time/rate"
)

//go:embed all:static
var staticFiles embed.FS

//go:embed templates/*
var templateFiles embed.FS

// startAPI registers all HTTP routes and starts the API server in a goroutine.
func (s *Server) startAPI() {
	mux := http.NewServeMux()

	rl := newRateLimitMiddleware(rate.NewLimiter(rate.Limit(200), 500))

	mux.Handle("/api", http.HandlerFunc(s.handleAPI))
	mux.Handle("/api/hosts/{hostname}", http.HandlerFunc(s.handleHostAPI))
	mux.Handle("/api/summary", http.HandlerFunc(s.handleSummaryAPI))
	mux.Handle("/metrics", http.HandlerFunc(s.handlePrometheus))

	content, err := fs.Sub(staticFiles, "static")
	if err != nil {
		s.logger.Fatalf("Failed to create sub filesystem: %v", err)
	}

	// Serve generated graphs from the graphDir
	imgFS := http.FileServer(http.Dir(s.graphDir))
	mux.Handle("/imgs/", imgFS)

	mux.Handle("/host-detail", http.HandlerFunc(s.handleHostDetail))

	// Serve static content
	htmlFS := http.FileServer(http.FS(content))
	mux.Handle("/", http.StripPrefix("/", htmlFS))

	handler := requireGET(rl(noCacheMiddleware(securityHeadersMiddleware(mux))))
	s.httpServer = &http.Server{
		Addr:              ":" + s.listenPort,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		s.logger.Infof("Starting API server on port %v...", s.listenPort)
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.logger.Fatalf("Failed to start API server: %v", err)
		}
	}()
}
