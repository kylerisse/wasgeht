package server

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kylerisse/wasgeht/pkg/check"
)

func TestGetOrCreateStatus_CreatesNew(t *testing.T) {
	s := &Server{
		statuses: make(map[string]map[string]*check.Status),
	}

	status := s.getOrCreateStatus("host1", "ping")
	if status == nil {
		t.Fatal("expected non-nil status")
	}
}

func TestGetOrCreateStatus_ReturnsSame(t *testing.T) {
	s := &Server{
		statuses: make(map[string]map[string]*check.Status),
	}

	s1 := s.getOrCreateStatus("host1", "ping")
	s2 := s.getOrCreateStatus("host1", "ping")
	if s1 != s2 {
		t.Error("expected same status pointer on second call")
	}
}

func TestGetOrCreateStatus_SeparateChecks(t *testing.T) {
	s := &Server{
		statuses: make(map[string]map[string]*check.Status),
	}

	ping := s.getOrCreateStatus("host1", "ping")
	http := s.getOrCreateStatus("host1", "http")
	if ping == http {
		t.Error("expected different status for different check types")
	}
}

func TestHostStatuses_ReturnsSnapshots(t *testing.T) {
	s := &Server{
		statuses: make(map[string]map[string]*check.Status),
	}

	status := s.getOrCreateStatus("host1", "ping")
	status.SetResult(check.Result{
		Success: true,
		Metrics: map[string]*int64{"latency_us": p64(5000)},
	})
	status.SetLastUpdate(1700000000)

	snaps := s.hostStatuses("host1")
	if len(snaps) != 1 {
		t.Fatalf("expected 1 snapshot, got %d", len(snaps))
	}

	snap, ok := snaps["ping"]
	if !ok {
		t.Fatal("expected ping snapshot")
	}
	if !snap.Alive {
		t.Error("expected alive")
	}
	if p := snap.Metrics["latency_us"]; p == nil || *p != 5000 {
		t.Errorf("expected latency_us=5000, got %v", snap.Metrics["latency_us"])
	}
	if snap.LastUpdate != 1700000000 {
		t.Errorf("expected lastupdate=1700000000, got %d", snap.LastUpdate)
	}
}

func TestHostStatuses_UnknownHost(t *testing.T) {
	s := &Server{
		statuses: make(map[string]map[string]*check.Status),
	}

	snaps := s.hostStatuses("nonexistent")
	if snaps != nil {
		t.Errorf("expected nil for unknown host, got %v", snaps)
	}
}

func TestHostStatuses_SnapshotIndependence(t *testing.T) {
	s := &Server{
		statuses: make(map[string]map[string]*check.Status),
	}

	status := s.getOrCreateStatus("host1", "ping")
	status.SetResult(check.Result{
		Success: true,
		Metrics: map[string]*int64{"latency_us": p64(1000)},
	})

	snaps := s.hostStatuses("host1")

	// Mutate the status after taking snapshots
	status.SetResult(check.Result{Success: false})

	// Snapshot should still reflect old state
	if !snaps["ping"].Alive {
		t.Error("snapshot should be independent of subsequent mutations")
	}
}

func TestLoadHosts_ValidFile(t *testing.T) {
	content := `{
		"router": {},
		"google": {
			"checks": {
				"ping": {"timeout": "5s"}
			}
		}
	}`

	dir := t.TempDir()
	path := filepath.Join(dir, "hosts.json")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	hosts, err := loadHosts(path)
	if err != nil {
		t.Fatalf("loadHosts failed: %v", err)
	}

	if len(hosts) != 2 {
		t.Fatalf("expected 2 hosts, got %d", len(hosts))
	}

	google, ok := hosts["google"]
	if !ok {
		t.Fatal("expected google host")
	}
	if google.Name != "google" {
		t.Errorf("expected name 'google', got %q", google.Name)
	}
	if _, ok := google.Checks["ping"]; !ok {
		t.Error("expected google to keep ping check")
	}
}

func TestLoadHosts_MissingFile(t *testing.T) {
	_, err := loadHosts("/nonexistent/path/hosts.json")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestLoadHosts_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hosts.json")
	if err := os.WriteFile(path, []byte("not json"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	_, err := loadHosts(path)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestLoadHosts_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hosts.json")
	if err := os.WriteFile(path, []byte("{}"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	hosts, err := loadHosts(path)
	if err != nil {
		t.Fatalf("loadHosts failed: %v", err)
	}
	if len(hosts) != 0 {
		t.Errorf("expected 0 hosts, got %d", len(hosts))
	}
}

func TestLoadHosts_BareHostIsInert(t *testing.T) {
	content := `{
		"bare": {},
		"custom": {
			"checks": {
				"http": {"path": "/health"}
			}
		}
	}`

	dir := t.TempDir()
	path := filepath.Join(dir, "hosts.json")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	hosts, err := loadHosts(path)
	if err != nil {
		t.Fatalf("loadHosts failed: %v", err)
	}

	if len(hosts["bare"].Checks) != 0 {
		t.Error("bare host should have no checks")
	}
	if _, ok := hosts["custom"].Checks["http"]; !ok {
		t.Error("custom host should keep http check")
	}
}

func TestLoadHosts_SetsName(t *testing.T) {
	content := `{"myhost": {}}`

	dir := t.TempDir()
	path := filepath.Join(dir, "hosts.json")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	hosts, err := loadHosts(path)
	if err != nil {
		t.Fatalf("loadHosts failed: %v", err)
	}

	if hosts["myhost"].Name != "myhost" {
		t.Errorf("expected name 'myhost', got %q", hosts["myhost"].Name)
	}
}
