package host

import (
	"encoding/json"
	"testing"
)

func TestJSON_WithChecks(t *testing.T) {
	input := `{
		"checks": {
			"ping": {"timeout": "5s"},
			"http": {"path": "/health"}
		}
	}`

	var h Host
	if err := json.Unmarshal([]byte(input), &h); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if len(h.Checks) != 2 {
		t.Fatalf("expected 2 checks, got %d", len(h.Checks))
	}
	if h.Checks["ping"]["timeout"] != "5s" {
		t.Errorf("expected ping timeout '5s', got %v", h.Checks["ping"]["timeout"])
	}
	if h.Checks["http"]["path"] != "/health" {
		t.Errorf("expected http path '/health', got %v", h.Checks["http"]["path"])
	}
}

func TestJSON_WithoutChecks(t *testing.T) {
	input := `{}`

	var h Host
	if err := json.Unmarshal([]byte(input), &h); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if h.Checks != nil {
		t.Error("expected nil checks when not in JSON")
	}
}

func TestJSON_NilChecksIsInert(t *testing.T) {
	h := Host{Name: "bare"}
	if len(h.Checks) != 0 {
		t.Error("host with no checks should be inert")
	}
}

func TestJSON_WithTags(t *testing.T) {
	input := `{
		"tags": {"category": "ap", "building": "expo"},
		"checks": {}
	}`

	var h Host
	if err := json.Unmarshal([]byte(input), &h); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if h.Tags["category"] != "ap" {
		t.Errorf("expected category 'ap', got %q", h.Tags["category"])
	}
	if h.Tags["building"] != "expo" {
		t.Errorf("expected building 'expo', got %q", h.Tags["building"])
	}
}

func TestJSON_WithoutTags(t *testing.T) {
	input := `{"checks": {}}`

	var h Host
	if err := json.Unmarshal([]byte(input), &h); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if h.Tags != nil {
		t.Error("expected nil tags when not in JSON")
	}

	out, err := json.Marshal(h)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	if string(out) == "" {
		t.Fatal("expected non-empty output")
	}
	var roundtrip map[string]any
	if err := json.Unmarshal(out, &roundtrip); err != nil {
		t.Fatalf("roundtrip unmarshal failed: %v", err)
	}
	if _, ok := roundtrip["tags"]; ok {
		t.Error("tags key should be omitted from JSON when nil")
	}
}

func TestJSON_MultiHost(t *testing.T) {
	input := `{
		"router": {},
		"google": {
			"checks": {
				"ping": {"timeout": "5s"}
			}
		}
	}`

	var hosts map[string]Host
	if err := json.Unmarshal([]byte(input), &hosts); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if len(hosts["router"].Checks) != 0 {
		t.Error("router should have no checks")
	}
	if _, ok := hosts["google"].Checks["ping"]; !ok {
		t.Error("google should have ping check")
	}
}
