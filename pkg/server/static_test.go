package server

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kylerisse/wasgeht/pkg/check"
	"github.com/kylerisse/wasgeht/pkg/host"
	"golang.org/x/time/rate"
)

// newTestHandler builds the full HTTP handler stack matching startAPI,
// without starting a real server.
func newTestHandler(s *Server) http.Handler {
	mux := http.NewServeMux()

	rl := newRateLimitMiddleware(rate.NewLimiter(rate.Limit(200), 500))

	mux.Handle("/api", http.HandlerFunc(s.handleAPI))
	mux.Handle("/api/hosts/{hostname}", http.HandlerFunc(s.handleHostAPI))
	mux.Handle("/api/summary", http.HandlerFunc(s.handleSummaryAPI))
	mux.Handle("/metrics", http.HandlerFunc(s.handlePrometheus))

	content, err := fs.Sub(staticFiles, "static")
	if err != nil {
		panic("failed to create sub filesystem: " + err.Error())
	}

	mux.Handle("/host-detail", http.HandlerFunc(s.handleHostDetail))

	htmlFS := http.FileServer(http.FS(content))
	mux.Handle("/", http.StripPrefix("/", htmlFS))

	return requireGET(rl(noCacheMiddleware(securityHeadersMiddleware(mux))))
}

func TestStaticServing_VendoredCSS(t *testing.T) {
	s := &Server{
		hosts:    make(map[string]*host.Host),
		statuses: make(map[string]map[string]*check.Status),
	}
	handler := newTestHandler(s)

	req := httptest.NewRequest("GET", "/vendor/pico-2.1.1.classless.min.css", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Result().StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Result().StatusCode)
	}
	ct := w.Result().Header.Get("Content-Type")
	if !strings.Contains(ct, "text/css") {
		t.Errorf("expected text/css content type, got %q", ct)
	}
}

func TestStaticServing_VendoredJS(t *testing.T) {
	s := &Server{
		hosts:    make(map[string]*host.Host),
		statuses: make(map[string]map[string]*check.Status),
	}
	handler := newTestHandler(s)

	req := httptest.NewRequest("GET", "/vendor/alpine-csp-3.15.8.min.js", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Result().StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Result().StatusCode)
	}
}

func TestStaticServing_AppCSS(t *testing.T) {
	s := &Server{
		hosts:    make(map[string]*host.Host),
		statuses: make(map[string]map[string]*check.Status),
	}
	handler := newTestHandler(s)

	req := httptest.NewRequest("GET", "/app.css", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Result().StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Result().StatusCode)
	}
}

func TestStaticServing_AppJS(t *testing.T) {
	s := &Server{
		hosts:    make(map[string]*host.Host),
		statuses: make(map[string]map[string]*check.Status),
	}
	handler := newTestHandler(s)

	req := httptest.NewRequest("GET", "/app.js", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Result().StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Result().StatusCode)
	}
	body := w.Body.String()
	if !strings.Contains(body, "Alpine.data") {
		t.Error("expected app.js to contain Alpine.data")
	}
}

func TestStaticServing_IndexPage(t *testing.T) {
	s := &Server{
		hosts:    make(map[string]*host.Host),
		statuses: make(map[string]map[string]*check.Status),
	}
	handler := newTestHandler(s)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Result().StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Result().StatusCode)
	}
	body := w.Body.String()
	if !strings.Contains(body, `x-data="dashboard"`) {
		t.Error("expected index.html to contain x-data=\"dashboard\"")
	}
	if !strings.Contains(body, "alpine-csp") {
		t.Error("expected index.html to reference alpine-csp")
	}
	if !strings.Contains(body, "app.css") {
		t.Error("expected index.html to reference app.css")
	}
}

func TestStaticServing_GridViewPage(t *testing.T) {
	s := &Server{
		hosts:    make(map[string]*host.Host),
		statuses: make(map[string]map[string]*check.Status),
	}
	handler := newTestHandler(s)

	req := httptest.NewRequest("GET", "/grid-view.html", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Result().StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Result().StatusCode)
	}
	body := w.Body.String()
	if !strings.Contains(body, `x-data="gridview"`) {
		t.Error("expected grid-view.html to contain x-data=\"gridview\"")
	}
}

func TestStaticServing_HostDetailPage(t *testing.T) {
	s := &Server{
		hosts: map[string]*host.Host{
			"testhost": {Name: "testhost"},
		},
		statuses: make(map[string]map[string]*check.Status),
	}
	handler := newTestHandler(s)

	req := httptest.NewRequest("GET", "/host-detail?hostname=testhost", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Result().StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Result().StatusCode)
	}
	body := w.Body.String()
	if !strings.Contains(body, `x-data="hostdetail"`) {
		t.Error("expected host-detail to contain x-data=\"hostdetail\"")
	}
	if !strings.Contains(body, "testhost") {
		t.Error("expected host-detail to contain hostname")
	}
	if !strings.Contains(body, "alpine-csp") {
		t.Error("expected host-detail to reference alpine-csp")
	}
}

func TestStaticServing_OldFilesReturn404(t *testing.T) {
	s := &Server{
		hosts:    make(map[string]*host.Host),
		statuses: make(map[string]map[string]*check.Status),
	}
	handler := newTestHandler(s)

	oldFiles := []string{
		"/styles.css",
		"/scripts.js",
		"/flamegraph.html",
		"/flamegraph.css",
		"/flamegraph.js",
		"/host-detail.css",
		"/host-detail.js",
	}

	for _, path := range oldFiles {
		req := httptest.NewRequest("GET", path, nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Result().StatusCode != http.StatusNotFound {
			t.Errorf("%s: expected 404, got %d", path, w.Result().StatusCode)
		}
	}
}
