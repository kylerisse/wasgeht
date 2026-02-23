package http

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/kylerisse/wasgeht/pkg/check"
)

func TestType(t *testing.T) {
	c, err := New([]string{"http://localhost"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.Type() != "http" {
		t.Errorf("expected type 'http', got %q", c.Type())
	}
}

func TestNew_NoURLs(t *testing.T) {
	_, err := New(nil)
	if err == nil {
		t.Error("expected error for nil URLs")
	}
	_, err = New([]string{})
	if err == nil {
		t.Error("expected error for empty URLs")
	}
}

func TestNew_Defaults(t *testing.T) {
	c, err := New([]string{"http://localhost"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.timeout != DefaultTimeout {
		t.Errorf("expected timeout %v, got %v", DefaultTimeout, c.timeout)
	}
	if !c.skipVerify {
		t.Error("expected skipVerify to default to true")
	}
	if len(c.urls) != 1 {
		t.Errorf("expected 1 URL, got %d", len(c.urls))
	}
}

func TestNew_WithTimeout(t *testing.T) {
	c, err := New([]string{"http://localhost"}, WithTimeout(5*time.Second))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.timeout != 5*time.Second {
		t.Errorf("expected timeout 5s, got %v", c.timeout)
	}
}

func TestNew_WithSkipVerifyFalse(t *testing.T) {
	c, err := New([]string{"http://localhost"}, WithSkipVerify(false))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.skipVerify {
		t.Error("expected skipVerify to be false")
	}
}

func TestDescribe_SingleURL(t *testing.T) {
	c, err := New([]string{"http://example.com"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	desc := c.Describe()
	if desc.GraphStyle != check.GraphStyleLine {
		t.Errorf("expected GraphStyle %q, got %q", check.GraphStyleLine, desc.GraphStyle)
	}
	if desc.Label != "response time" {
		t.Errorf("expected Label 'response time', got %q", desc.Label)
	}
	if len(desc.Metrics) != 1 {
		t.Fatalf("expected 1 metric, got %d", len(desc.Metrics))
	}
	m := desc.Metrics[0]
	if m.ResultKey != "http://example.com" {
		t.Errorf("expected ResultKey 'http://example.com', got %q", m.ResultKey)
	}
	if m.DSName != "url0" {
		t.Errorf("expected DSName 'url0', got %q", m.DSName)
	}
	if m.Label != "http://example.com" {
		t.Errorf("expected Label 'http://example.com', got %q", m.Label)
	}
	if m.Unit != "ms" {
		t.Errorf("expected Unit 'ms', got %q", m.Unit)
	}
	if m.Scale != 1000 {
		t.Errorf("expected Scale 1000, got %d", m.Scale)
	}
}

func TestDescribe_MultipleURLs(t *testing.T) {
	urls := []string{"http://a.com", "http://b.com", "https://c.com"}
	c, err := New(urls)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	desc := c.Describe()
	if len(desc.Metrics) != 3 {
		t.Fatalf("expected 3 metrics, got %d", len(desc.Metrics))
	}
	for i, u := range urls {
		if desc.Metrics[i].ResultKey != u {
			t.Errorf("metric %d: expected ResultKey %q, got %q", i, u, desc.Metrics[i].ResultKey)
		}
		expectedDS := fmt.Sprintf("url%d", i)
		if desc.Metrics[i].DSName != expectedDS {
			t.Errorf("metric %d: expected DSName %q, got %q", i, expectedDS, desc.Metrics[i].DSName)
		}
	}
}

func TestRun_SingleURL_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c, err := New([]string{srv.URL})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result := c.Run(context.Background())
	if !result.Success {
		t.Errorf("expected success, got failure: %v", result.Err)
	}
	if result.Metrics[srv.URL] <= 0 {
		t.Errorf("expected positive response time, got %d", result.Metrics[srv.URL])
	}
	if result.Timestamp.IsZero() {
		t.Error("expected non-zero timestamp")
	}
}

func TestRun_MultipleURLs_AllSuccess(t *testing.T) {
	srv1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv1.Close()

	srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv2.Close()

	c, err := New([]string{srv1.URL, srv2.URL})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result := c.Run(context.Background())
	if !result.Success {
		t.Errorf("expected success, got failure: %v", result.Err)
	}
	if len(result.Metrics) != 2 {
		t.Errorf("expected 2 metrics, got %d", len(result.Metrics))
	}
	if result.Metrics[srv1.URL] <= 0 {
		t.Errorf("expected positive response time for srv1")
	}
	if result.Metrics[srv2.URL] <= 0 {
		t.Errorf("expected positive response time for srv2")
	}
}

func TestRun_MultipleURLs_PartialFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	// Second URL is unreachable
	c, err := New([]string{srv.URL, "http://192.0.2.1:1"}, WithTimeout(1*time.Second))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result := c.Run(context.Background())
	// Partial failure means not all URLs responded — check fails
	if result.Success {
		t.Error("expected failure with partial failure (not all URLs responded)")
	}
	if result.Err == nil {
		t.Error("expected non-nil error")
	}
	// But we should still have the successful URL's metric
	if result.Metrics[srv.URL] <= 0 {
		t.Errorf("expected positive response time for successful URL")
	}
}

func TestRun_AllURLsFail(t *testing.T) {
	c, err := New([]string{"http://192.0.2.1:1"}, WithTimeout(1*time.Second))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result := c.Run(context.Background())
	if result.Success {
		t.Error("expected failure when all URLs fail")
	}
	if result.Err == nil {
		t.Error("expected non-nil error when all URLs fail")
	}
	if len(result.Metrics) != 0 {
		t.Errorf("expected 0 metrics when all fail, got %d", len(result.Metrics))
	}
}

func TestRun_NonOKStatus_StillSuccess(t *testing.T) {
	// Any HTTP response means the endpoint is reachable
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c, err := New([]string{srv.URL})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result := c.Run(context.Background())
	if !result.Success {
		t.Error("expected success for non-OK status (endpoint is reachable)")
	}
}

func TestRun_Redirect_NotFollowed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://example.com", http.StatusFound)
	}))
	defer srv.Close()

	c, err := New([]string{srv.URL})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result := c.Run(context.Background())
	if !result.Success {
		t.Error("expected success (redirect response is still a response)")
	}
}

func TestRun_ContextCancellation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c, err := New([]string{srv.URL})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	result := c.Run(ctx)
	if result.Success {
		t.Error("expected failure when context is cancelled")
	}
}

func TestRun_TLSInsecure(t *testing.T) {
	// httptest.NewTLSServer uses a self-signed cert
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c, err := New([]string{srv.URL})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result := c.Run(context.Background())
	if !result.Success {
		t.Errorf("expected success with insecure TLS (skip_verify=true), got failure: %v", result.Err)
	}
}

func TestRun_TLSVerify_FailsOnSelfSigned(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c, err := New([]string{srv.URL}, WithSkipVerify(false))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result := c.Run(context.Background())
	if result.Success {
		t.Error("expected failure when verifying self-signed cert")
	}
}

// --- Factory tests ---

func TestFactory_WithURLs(t *testing.T) {
	config := map[string]any{
		"urls": []any{"http://localhost:8080", "http://localhost:9090"},
	}

	chk, err := Factory(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if chk.Type() != "http" {
		t.Errorf("expected type 'http', got %q", chk.Type())
	}

	httpChk := chk.(*Check)
	if len(httpChk.urls) != 2 {
		t.Errorf("expected 2 URLs, got %d", len(httpChk.urls))
	}
}

func TestFactory_WithTimeout(t *testing.T) {
	config := map[string]any{
		"urls":    []any{"http://localhost:8080"},
		"timeout": "5s",
	}

	chk, err := Factory(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	httpChk := chk.(*Check)
	if httpChk.timeout != 5*time.Second {
		t.Errorf("expected timeout 5s, got %v", httpChk.timeout)
	}
}

func TestFactory_WithSkipVerify(t *testing.T) {
	config := map[string]any{
		"urls":        []any{"https://localhost"},
		"skip_verify": false,
	}

	chk, err := Factory(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	httpChk := chk.(*Check)
	if httpChk.skipVerify {
		t.Error("expected skipVerify to be false")
	}
}

func TestFactory_DefaultSkipVerify(t *testing.T) {
	config := map[string]any{
		"urls": []any{"https://localhost"},
	}

	chk, err := Factory(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	httpChk := chk.(*Check)
	if !httpChk.skipVerify {
		t.Error("expected skipVerify to default to true")
	}
}

func TestFactory_MissingURLs(t *testing.T) {
	config := map[string]any{}
	_, err := Factory(config)
	if err == nil {
		t.Error("expected error for missing URLs")
	}
}

func TestFactory_EmptyURLs(t *testing.T) {
	config := map[string]any{
		"urls": []any{},
	}
	_, err := Factory(config)
	if err == nil {
		t.Error("expected error for empty URLs")
	}
}

func TestFactory_InvalidURLType(t *testing.T) {
	config := map[string]any{
		"urls": []any{123},
	}
	_, err := Factory(config)
	if err == nil {
		t.Error("expected error for non-string URL")
	}
}

func TestFactory_InvalidTimeout(t *testing.T) {
	config := map[string]any{
		"urls":    []any{"http://localhost"},
		"timeout": "not-a-duration",
	}
	_, err := Factory(config)
	if err == nil {
		t.Error("expected error for invalid timeout")
	}
}

func TestFactory_WrongTimeoutType(t *testing.T) {
	config := map[string]any{
		"urls":    []any{"http://localhost"},
		"timeout": 123,
	}
	_, err := Factory(config)
	if err == nil {
		t.Error("expected error for non-string timeout")
	}
}

func TestFactory_WrongSkipVerifyType(t *testing.T) {
	config := map[string]any{
		"urls":        []any{"http://localhost"},
		"skip_verify": "yes",
	}
	_, err := Factory(config)
	if err == nil {
		t.Error("expected error for non-bool skip_verify")
	}
}

func TestFactory_WrongURLsType(t *testing.T) {
	config := map[string]any{
		"urls": "not-a-list",
	}
	_, err := Factory(config)
	if err == nil {
		t.Error("expected error for non-list urls")
	}
}

func TestFactory_StringSliceURLs(t *testing.T) {
	// Test with actual []string (not []any from JSON)
	config := map[string]any{
		"urls": []string{"http://localhost:8080"},
	}

	chk, err := Factory(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	httpChk := chk.(*Check)
	if len(httpChk.urls) != 1 {
		t.Errorf("expected 1 URL, got %d", len(httpChk.urls))
	}
}

func TestFactory_IgnoresTargetKey(t *testing.T) {
	// The worker injects "target" but HTTP check should ignore it
	config := map[string]any{
		"target": "some-host",
		"urls":   []any{"http://localhost:8080"},
	}

	chk, err := Factory(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	httpChk := chk.(*Check)
	if len(httpChk.urls) != 1 || httpChk.urls[0] != "http://localhost:8080" {
		t.Errorf("expected URL from urls config, not target")
	}
}

// --- Registry integration ---

func TestRegistryIntegration(t *testing.T) {
	reg := check.NewRegistry()
	err := reg.Register(TypeName, Factory)
	if err != nil {
		t.Fatalf("failed to register http: %v", err)
	}

	chk, err := reg.Create("http", map[string]any{
		"urls": []any{"http://localhost:8080", "https://localhost:8443"},
	})
	if err != nil {
		t.Fatalf("failed to create http check: %v", err)
	}

	if chk.Type() != "http" {
		t.Errorf("expected type 'http', got %q", chk.Type())
	}

	desc := chk.Describe()
	if len(desc.Metrics) != 2 {
		t.Fatalf("expected 2 metrics in descriptor, got %d", len(desc.Metrics))
	}
	if desc.GraphStyle != check.GraphStyleLine {
		t.Errorf("expected GraphStyle %q, got %q", check.GraphStyleLine, desc.GraphStyle)
	}
	if desc.Metrics[0].DSName != "url0" {
		t.Errorf("expected first DSName 'url0', got %q", desc.Metrics[0].DSName)
	}
	if desc.Metrics[1].DSName != "url1" {
		t.Errorf("expected second DSName 'url1', got %q", desc.Metrics[1].DSName)
	}
}

// TestCheckInterface verifies that Check satisfies the check.Check interface.
func TestCheckInterface(t *testing.T) {
	var _ check.Check = &Check{}
}
