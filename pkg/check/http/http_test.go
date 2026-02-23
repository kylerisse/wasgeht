package http

import (
	"testing"
	"time"

	"github.com/kylerisse/wasgeht/pkg/check"
)

func TestNew_Valid(t *testing.T) {
	chk, err := New([]string{"http://localhost:8080"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if chk.Type() != "http" {
		t.Errorf("expected type 'http', got %q", chk.Type())
	}
}

func TestNew_EmptyURLs(t *testing.T) {
	_, err := New([]string{})
	if err == nil {
		t.Error("expected error for empty URLs")
	}
}

func TestNew_WithTimeout(t *testing.T) {
	chk, err := New([]string{"http://localhost"}, WithTimeout(5*time.Second))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if chk.timeout != 5*time.Second {
		t.Errorf("expected timeout 5s, got %v", chk.timeout)
	}
}

func TestNew_WithSkipVerify(t *testing.T) {
	chk, err := New([]string{"https://localhost"}, WithSkipVerify(false))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if chk.skipVerify {
		t.Error("expected skipVerify false")
	}
}

func TestDescribe_SingleURL(t *testing.T) {
	chk, err := New([]string{"http://localhost:8080"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	desc := chk.Describe()
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
	chk, err := New([]string{"http://a.com", "http://b.com"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	desc := chk.Describe()
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
		"urls": []any{"http://localhost:8080"},
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
}

func TestFactory_WithTimeout(t *testing.T) {
	config := map[string]any{
		"urls":    []any{"http://localhost"},
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
