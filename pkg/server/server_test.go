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
	if status.Alive() {
		t.Error("new status should not be alive")
	}
}

func TestGetOrCreateStatus_ReturnsSame(t *testing.T) {
	s := &Server{
		statuses: make(map[string]map[string]*check.Status),
	}

	s1 := s.getOrCreateStatus("host1", "ping")
	s2 := s.getOrCreateStatus("host1", "ping")

	if s1 != s2 {
		t.Error("expected same status instance on repeated calls")
	}
}

func TestGetOrCreateStatus_SeparateCheckTypes(t *testing.T) {
	s := &Server{
		statuses: make(map[string]map[string]*check.Status),
	}

	pingStatus := s.getOrCreateStatus("host1", "ping")
	httpStatus := s.getOrCreateStatus("host1", "http")

	if pingStatus == httpStatus {
		t.Error("expected different status instances for different check types")
	}
}

func TestGetOrCreateStatus_SeparateHosts(t *testing.T) {
	s := &Server{
		statuses: make(map[string]map[string]*check.Status),
	}

	h1 := s.getOrCreateStatus("host1", "ping")
	h2 := s.getOrCreateStatus("host2", "ping")

	if h1 == h2 {
		t.Error("expected different status instances for different hosts")
	}
}

func TestHostStatuses_ReturnsSnapshots(t *testing.T) {
	s := &Server{
		statuses: make(map[string]map[string]*check.Status),
	}

	status := s.getOrCreateStatus("host1", "ping")
	status.SetResult(check.Result{
		Success: true,
		Metrics: map[string]int64{"latency_us": 5000},
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
	if snap.Metrics["latency_us"] != 5000 {
		t.Errorf("expected latency_us=5000, got %d", snap.Metrics["latency_us"])
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
		Metrics: map[string]int64{"latency_us": 1000},
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
			"address": "8.8.8.8",
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
	if google.Address != "8.8.8.8" {
		t.Errorf("expected address 8.8.8.8, got %q", google.Address)
	}
	if google.Name != "google" {
		t.Errorf("expected name 'google', got %q", google.Name)
	}

	router, ok := hosts["router"]
	if !ok {
		t.Fatal("expected router host")
	}
	// Router has no explicit checks, so ApplyDefaults should give it ping
	if _, ok := router.Checks["ping"]; !ok {
		t.Error("expected default ping check on router")
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

func TestLoadHosts_AppliesDefaults(t *testing.T) {
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

	// bare host should get default ping
	if _, ok := hosts["bare"].Checks["ping"]; !ok {
		t.Error("bare host should have default ping check")
	}

	// custom host should keep its explicit checks and not get ping injected
	if _, ok := hosts["custom"].Checks["http"]; !ok {
		t.Error("custom host should keep http check")
	}
	if _, ok := hosts["custom"].Checks["ping"]; ok {
		t.Error("custom host should not get ping injected")
	}
}

func TestLoadHosts_SetsName(t *testing.T) {
	content := `{"myhost": {"address": "1.2.3.4"}}`

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
