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

func TestNew_Valid(t *testing.T) {
	chk, err := New([]string{"http://localhost:8080"}, "test label")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if chk.Type() != "http" {
		t.Errorf("expected type 'http', got %q", chk.Type())
	}
	if chk.label != "test label" {
		t.Errorf("expected label 'test label', got %q", chk.label)
	}
}

func TestNew_EmptyURLs(t *testing.T) {
	_, err := New([]string{}, "test")
	if err == nil {
		t.Error("expected error for empty URLs")
	}
}

func TestNew_EmptyLabel(t *testing.T) {
	_, err := New([]string{"http://localhost"}, "")
	if err == nil {
		t.Error("expected error for empty label")
	}
}

func TestNew_WithTimeout(t *testing.T) {
	chk, err := New([]string{"http://localhost"}, "test", WithTimeout(5*time.Second))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if chk.timeout != 5*time.Second {
		t.Errorf("expected timeout 5s, got %v", chk.timeout)
	}
}

func TestNew_WithSkipVerify(t *testing.T) {
	chk, err := New([]string{"https://localhost"}, "test", WithSkipVerify(false))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if chk.skipVerify {
		t.Error("expected skipVerify false")
	}
}

func TestDescribe_SingleURL(t *testing.T) {
	chk, err := New([]string{"http://localhost:8080"}, "my http check")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	desc := chk.Describe()
	if desc.Label != "my http check" {
		t.Errorf("expected Descriptor.Label 'my http check', got %q", desc.Label)
	}
	if len(desc.Metrics) != 1 {
		t.Fatalf("expected 1 metric, got %d", len(desc.Metrics))
	}
	if desc.Metrics[0].DSName != "url0" {
		t.Errorf("expected DSName 'url0', got %q", desc.Metrics[0].DSName)
	}
	if desc.Metrics[0].ResultKey != "http://localhost:8080" {
		t.Errorf("expected ResultKey to be URL, got %q", desc.Metrics[0].ResultKey)
	}
}

func TestDescribe_MultipleURLs(t *testing.T) {
	chk, err := New([]string{"http://a.com", "http://b.com"}, "ab check")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	desc := chk.Describe()
	if desc.Label != "ab check" {
		t.Errorf("expected Descriptor.Label 'ab check', got %q", desc.Label)
	}
	if len(desc.Metrics) != 2 {
		t.Fatalf("expected 2 metrics, got %d", len(desc.Metrics))
	}
	if desc.Metrics[0].DSName != "url0" {
		t.Errorf("expected first DSName 'url0', got %q", desc.Metrics[0].DSName)
	}
	if desc.Metrics[1].DSName != "url1" {
		t.Errorf("expected second DSName 'url1', got %q", desc.Metrics[1].DSName)
	}
}

func TestFactory_MinimalConfig(t *testing.T) {
	config := map[string]any{
		"urls":  []any{"http://localhost:8080"},
		"label": "test check",
	}

	chk, err := Factory(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	httpChk := chk.(*Check)
	if httpChk.timeout != DefaultTimeout {
		t.Errorf("expected default timeout, got %v", httpChk.timeout)
	}
	if !httpChk.skipVerify {
		t.Error("expected skipVerify to default to true")
	}
	if httpChk.label != "test check" {
		t.Errorf("expected label 'test check', got %q", httpChk.label)
	}
}

func TestFactory_MissingLabel(t *testing.T) {
	config := map[string]any{
		"urls": []any{"http://localhost:8080"},
	}
	_, err := Factory(config)
	if err == nil {
		t.Error("expected error for missing label")
	}
}

func TestFactory_EmptyLabel(t *testing.T) {
	config := map[string]any{
		"urls":  []any{"http://localhost:8080"},
		"label": "",
	}
	_, err := Factory(config)
	if err == nil {
		t.Error("expected error for empty label")
	}
}

func TestFactory_WrongLabelType(t *testing.T) {
	config := map[string]any{
		"urls":  []any{"http://localhost:8080"},
		"label": 42,
	}
	_, err := Factory(config)
	if err == nil {
		t.Error("expected error for non-string label")
	}
}

func TestFactory_WithTimeout(t *testing.T) {
	config := map[string]any{
		"urls":    []any{"http://localhost"},
		"label":   "test",
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
		"label":       "test",
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
		"urls":  []any{"https://localhost"},
		"label": "test",
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
	config := map[string]any{"label": "test"}
	_, err := Factory(config)
	if err == nil {
		t.Error("expected error for missing URLs")
	}
}

func TestFactory_EmptyURLs(t *testing.T) {
	config := map[string]any{
		"urls":  []any{},
		"label": "test",
	}
	_, err := Factory(config)
	if err == nil {
		t.Error("expected error for empty URLs")
	}
}

func TestFactory_InvalidURLType(t *testing.T) {
	config := map[string]any{
		"urls":  []any{123},
		"label": "test",
	}
	_, err := Factory(config)
	if err == nil {
		t.Error("expected error for non-string URL")
	}
}

func TestFactory_InvalidTimeout(t *testing.T) {
	config := map[string]any{
		"urls":    []any{"http://localhost"},
		"label":   "test",
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
		"label":   "test",
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
		"label":       "test",
		"skip_verify": "yes",
	}
	_, err := Factory(config)
	if err == nil {
		t.Error("expected error for non-bool skip_verify")
	}
}

func TestFactory_WrongURLsType(t *testing.T) {
	config := map[string]any{
		"urls":  "not-a-list",
		"label": "test",
	}
	_, err := Factory(config)
	if err == nil {
		t.Error("expected error for non-list urls")
	}
}

func TestFactory_StringSliceURLs(t *testing.T) {
	config := map[string]any{
		"urls":  []string{"http://localhost:8080"},
		"label": "test",
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
	config := map[string]any{
		"target": "some-host",
		"urls":   []any{"http://localhost:8080"},
		"label":  "test",
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

func TestRegistryIntegration(t *testing.T) {
	reg := check.NewRegistry()
	err := reg.Register(TypeName, Factory)
	if err != nil {
		t.Fatalf("failed to register http: %v", err)
	}

	chk, err := reg.Create("http", map[string]any{
		"urls":  []any{"http://localhost:8080", "https://localhost:8443"},
		"label": "integration test",
	})
	if err != nil {
		t.Fatalf("failed to create http check: %v", err)
	}

	if chk.Type() != "http" {
		t.Errorf("expected type 'http', got %q", chk.Type())
	}

	desc := chk.Describe()
	if desc.Label != "integration test" {
		t.Errorf("expected Descriptor.Label 'integration test', got %q", desc.Label)
	}
	if len(desc.Metrics) != 2 {
		t.Fatalf("expected 2 metrics in descriptor, got %d", len(desc.Metrics))
	}
	if desc.Metrics[0].DSName != "url0" {
		t.Errorf("expected first DSName 'url0', got %q", desc.Metrics[0].DSName)
	}
	if desc.Metrics[1].DSName != "url1" {
		t.Errorf("expected second DSName 'url1', got %q", desc.Metrics[1].DSName)
	}
}

func TestCheckInterface(t *testing.T) {
	var _ check.Check = &Check{}
}

func TestRun_SingleURL_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c, err := New([]string{srv.URL}, "test server")
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

	c, err := New([]string{srv1.URL, srv2.URL}, "two servers")
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
}

func TestDescribe_SingleURL_MetricFields(t *testing.T) {
	url := "http://example.com"
	c, err := New([]string{url}, "example")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	desc := c.Describe()
	if len(desc.Metrics) != 1 {
		t.Fatalf("expected 1 metric, got %d", len(desc.Metrics))
	}
	m := desc.Metrics[0]
	if m.DSName != "url0" {
		t.Errorf("expected DSName 'url0', got %q", m.DSName)
	}
	if m.Label != url {
		t.Errorf("expected Label %q, got %q", url, m.Label)
	}
	if m.Unit != "ms" {
		t.Errorf("expected Unit 'ms', got %q", m.Unit)
	}
	if m.Scale != 1000 {
		t.Errorf("expected Scale 1000, got %d", m.Scale)
	}
}

func TestDescribe_MultipleURLs_AllFields(t *testing.T) {
	urls := []string{"http://a.com", "http://b.com", "https://c.com"}
	c, err := New(urls, "multi")
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
