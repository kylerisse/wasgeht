package rrd

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/kylerisse/wasgeht/pkg/check"
	"github.com/sirupsen/logrus"
)

// requireRRDTool skips the test if rrdtool is not on PATH.
func requireRRDTool(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("rrdtool"); err != nil {
		t.Skip("skipping: rrdtool not found on PATH")
	}
}

func testLogger() *logrus.Logger {
	l := logrus.New()
	l.SetOutput(os.Stderr)
	l.SetLevel(logrus.DebugLevel)
	return l
}

// singleMetric is the standard ping-like metric for tests.
var singleMetric = []check.MetricDef{
	{ResultKey: "latency_us", DSName: "latency", Label: "latency", Unit: "ms", Scale: 1000},
}

// multiMetrics simulates a wifi_stations-like check with two stored data sources.
var multiMetrics = []check.MetricDef{
	{ResultKey: "clients_2ghz", DSName: "clients2g", Label: "2.4 GHz clients", Unit: "clients", Scale: 0},
	{ResultKey: "clients_5ghz", DSName: "clients5g", Label: "5 GHz clients", Unit: "clients", Scale: 0},
}

func TestNewRRD_CreatesFile(t *testing.T) {
	requireRRDTool(t)

	rrdDir := t.TempDir()
	graphDir := t.TempDir()
	logger := testLogger()

	r, err := NewRRD("testhost", rrdDir, graphDir, "ping", singleMetric, logger)
	if err != nil {
		t.Fatalf("NewRRD failed: %v", err)
	}
	defer r.file.Close()

	// Check the RRD file was created in the right place
	rrdPath := filepath.Join(rrdDir, "testhost", "ping.rrd")
	if _, err := os.Stat(rrdPath); os.IsNotExist(err) {
		t.Errorf("expected RRD file at %s", rrdPath)
	}
}

func TestNewRRD_CreatesGraphDir(t *testing.T) {
	requireRRDTool(t)

	rrdDir := t.TempDir()
	graphDir := t.TempDir()
	logger := testLogger()

	r, err := NewRRD("testhost", rrdDir, graphDir, "ping", singleMetric, logger)
	if err != nil {
		t.Fatalf("NewRRD failed: %v", err)
	}
	defer r.file.Close()

	// Check the graph directory was created
	imgDir := filepath.Join(graphDir, "imgs", "testhost")
	if _, err := os.Stat(imgDir); os.IsNotExist(err) {
		t.Errorf("expected graph directory at %s", imgDir)
	}
}

func TestNewRRD_CreatesGraphFiles(t *testing.T) {
	requireRRDTool(t)

	rrdDir := t.TempDir()
	graphDir := t.TempDir()
	logger := testLogger()

	r, err := NewRRD("testhost", rrdDir, graphDir, "ping", singleMetric, logger)
	if err != nil {
		t.Fatalf("NewRRD failed: %v", err)
	}
	defer r.file.Close()

	// Should have created graphs for all time lengths
	expectedTimeLengths := []string{"15m", "1h", "4h", "8h", "1d", "4d", "1w", "31d", "93d", "1y", "2y", "5y"}
	for _, tl := range expectedTimeLengths {
		graphPath := filepath.Join(graphDir, "imgs", "testhost", "testhost_ping_"+tl+".png")
		if _, err := os.Stat(graphPath); os.IsNotExist(err) {
			t.Errorf("expected graph file at %s", graphPath)
		}
	}

	if len(r.graphs) != len(expectedTimeLengths) {
		t.Errorf("expected %d graphs, got %d", len(expectedTimeLengths), len(r.graphs))
	}
}

func TestNewRRD_IdempotentCreation(t *testing.T) {
	requireRRDTool(t)

	rrdDir := t.TempDir()
	graphDir := t.TempDir()
	logger := testLogger()

	r1, err := NewRRD("testhost", rrdDir, graphDir, "ping", singleMetric, logger)
	if err != nil {
		t.Fatalf("first NewRRD failed: %v", err)
	}
	r1.file.Close()

	// Creating again should not fail — should reuse existing file
	r2, err := NewRRD("testhost", rrdDir, graphDir, "ping", singleMetric, logger)
	if err != nil {
		t.Fatalf("second NewRRD failed: %v", err)
	}
	r2.file.Close()
}

func TestNewRRD_InvalidRRDDir(t *testing.T) {
	requireRRDTool(t)

	graphDir := t.TempDir()
	logger := testLogger()

	_, err := NewRRD("testhost", "/nonexistent/path", graphDir, "ping", singleMetric, logger)
	if err == nil {
		t.Error("expected error for nonexistent rrdDir")
	}
}

func TestNewRRD_EmptyMetrics(t *testing.T) {
	requireRRDTool(t)

	rrdDir := t.TempDir()
	graphDir := t.TempDir()
	logger := testLogger()

	_, err := NewRRD("testhost", rrdDir, graphDir, "ping", []check.MetricDef{}, logger)
	if err == nil {
		t.Error("expected error for empty metrics")
	}
}

func TestNewRRD_NilMetrics(t *testing.T) {
	requireRRDTool(t)

	rrdDir := t.TempDir()
	graphDir := t.TempDir()
	logger := testLogger()

	_, err := NewRRD("testhost", rrdDir, graphDir, "ping", nil, logger)
	if err == nil {
		t.Error("expected error for nil metrics")
	}
}

func TestSafeUpdate_Success(t *testing.T) {
	requireRRDTool(t)

	rrdDir := t.TempDir()
	graphDir := t.TempDir()
	logger := testLogger()

	r, err := NewRRD("testhost", rrdDir, graphDir, "ping", singleMetric, logger)
	if err != nil {
		t.Fatalf("NewRRD failed: %v", err)
	}
	defer r.file.Close()

	// Update with a value
	ts := time.Now()
	lastUpdate, err := r.SafeUpdate(ts, []int64{42000})
	if err != nil {
		t.Fatalf("SafeUpdate failed: %v", err)
	}
	if lastUpdate != ts.Unix() {
		t.Errorf("expected lastUpdate=%d, got %d", ts.Unix(), lastUpdate)
	}
}

func TestSafeUpdate_RejectsDuplicate(t *testing.T) {
	requireRRDTool(t)

	rrdDir := t.TempDir()
	graphDir := t.TempDir()
	logger := testLogger()

	r, err := NewRRD("testhost", rrdDir, graphDir, "ping", singleMetric, logger)
	if err != nil {
		t.Fatalf("NewRRD failed: %v", err)
	}
	defer r.file.Close()

	ts := time.Now()
	_, err = r.SafeUpdate(ts, []int64{42000})
	if err != nil {
		t.Fatalf("first SafeUpdate failed: %v", err)
	}

	// Same timestamp should be rejected
	_, err = r.SafeUpdate(ts, []int64{43000})
	if err == nil {
		t.Error("expected error when updating with same timestamp")
	}
}

func TestSafeUpdate_AcceptsNewer(t *testing.T) {
	requireRRDTool(t)

	rrdDir := t.TempDir()
	graphDir := t.TempDir()
	logger := testLogger()

	r, err := NewRRD("testhost", rrdDir, graphDir, "ping", singleMetric, logger)
	if err != nil {
		t.Fatalf("NewRRD failed: %v", err)
	}
	defer r.file.Close()

	ts1 := time.Now()
	_, err = r.SafeUpdate(ts1, []int64{42000})
	if err != nil {
		t.Fatalf("first SafeUpdate failed: %v", err)
	}

	// One minute later should succeed
	ts2 := ts1.Add(time.Minute)
	lastUpdate, err := r.SafeUpdate(ts2, []int64{43000})
	if err != nil {
		t.Fatalf("second SafeUpdate failed: %v", err)
	}
	if lastUpdate != ts2.Unix() {
		t.Errorf("expected lastUpdate=%d, got %d", ts2.Unix(), lastUpdate)
	}
}

func TestSafeUpdate_EmptyValues(t *testing.T) {
	requireRRDTool(t)

	rrdDir := t.TempDir()
	graphDir := t.TempDir()
	logger := testLogger()

	r, err := NewRRD("testhost", rrdDir, graphDir, "ping", singleMetric, logger)
	if err != nil {
		t.Fatalf("NewRRD failed: %v", err)
	}
	defer r.file.Close()

	// Empty values should still succeed (just redraws graphs, no rrdtool update)
	ts := time.Now()
	_, err = r.SafeUpdate(ts, []int64{})
	if err != nil {
		t.Fatalf("SafeUpdate with empty values failed: %v", err)
	}
}

func TestGetLastUpdate_NewFile(t *testing.T) {
	requireRRDTool(t)

	rrdDir := t.TempDir()
	graphDir := t.TempDir()
	logger := testLogger()

	r, err := NewRRD("testhost", rrdDir, graphDir, "ping", singleMetric, logger)
	if err != nil {
		t.Fatalf("NewRRD failed: %v", err)
	}
	defer r.file.Close()

	lastUpdate, err := r.getLastUpdate()
	if err != nil {
		t.Fatalf("getLastUpdate failed: %v", err)
	}

	// A brand new RRD should have lastUpdate of 0 (no valid data yet)
	if lastUpdate != 0 {
		t.Errorf("expected lastUpdate=0 for new RRD, got %d", lastUpdate)
	}
}

func TestGetLastUpdate_AfterUpdate(t *testing.T) {
	requireRRDTool(t)

	rrdDir := t.TempDir()
	graphDir := t.TempDir()
	logger := testLogger()

	r, err := NewRRD("testhost", rrdDir, graphDir, "ping", singleMetric, logger)
	if err != nil {
		t.Fatalf("NewRRD failed: %v", err)
	}
	defer r.file.Close()

	ts := time.Now()
	_, err = r.SafeUpdate(ts, []int64{42000})
	if err != nil {
		t.Fatalf("SafeUpdate failed: %v", err)
	}

	lastUpdate, err := r.getLastUpdate()
	if err != nil {
		t.Fatalf("getLastUpdate failed: %v", err)
	}

	if lastUpdate != ts.Unix() {
		t.Errorf("expected lastUpdate=%d, got %d", ts.Unix(), lastUpdate)
	}
}

func TestNewRRD_NoScaling(t *testing.T) {
	requireRRDTool(t)

	rrdDir := t.TempDir()
	graphDir := t.TempDir()
	logger := testLogger()

	noScaleMetric := []check.MetricDef{
		{ResultKey: "rtt_us", DSName: "rtt", Label: "rtt", Unit: "us", Scale: 0},
	}

	r, err := NewRRD("testhost", rrdDir, graphDir, "ping", noScaleMetric, logger)
	if err != nil {
		t.Fatalf("NewRRD failed: %v", err)
	}
	defer r.file.Close()

	// Verify graphs were created
	if len(r.graphs) == 0 {
		t.Error("expected graphs to be initialized")
	}
}

func TestNewRRD_DifferentCheckTypes(t *testing.T) {
	requireRRDTool(t)

	rrdDir := t.TempDir()
	graphDir := t.TempDir()
	logger := testLogger()

	r1, err := NewRRD("testhost", rrdDir, graphDir, "ping", singleMetric, logger)
	if err != nil {
		t.Fatalf("NewRRD ping failed: %v", err)
	}
	defer r1.file.Close()

	httpMetric := []check.MetricDef{
		{ResultKey: "response_ms", DSName: "response", Label: "response time", Unit: "ms", Scale: 0},
	}
	r2, err := NewRRD("testhost", rrdDir, graphDir, "http", httpMetric, logger)
	if err != nil {
		t.Fatalf("NewRRD http failed: %v", err)
	}
	defer r2.file.Close()

	// Both should exist in the same host directory
	pingPath := filepath.Join(rrdDir, "testhost", "ping.rrd")
	httpPath := filepath.Join(rrdDir, "testhost", "http.rrd")

	if _, err := os.Stat(pingPath); os.IsNotExist(err) {
		t.Errorf("expected ping.rrd at %s", pingPath)
	}
	if _, err := os.Stat(httpPath); os.IsNotExist(err) {
		t.Errorf("expected http.rrd at %s", httpPath)
	}
}

// --- Multi-DS tests ---

func TestNewRRD_MultiDS_CreatesFile(t *testing.T) {
	requireRRDTool(t)

	rrdDir := t.TempDir()
	graphDir := t.TempDir()
	logger := testLogger()

	r, err := NewRRD("ap1", rrdDir, graphDir, "wifi_stations", multiMetrics, logger)
	if err != nil {
		t.Fatalf("NewRRD multi-DS failed: %v", err)
	}
	defer r.file.Close()

	rrdPath := filepath.Join(rrdDir, "ap1", "wifi_stations.rrd")
	if _, err := os.Stat(rrdPath); os.IsNotExist(err) {
		t.Errorf("expected RRD file at %s", rrdPath)
	}
}

func TestNewRRD_MultiDS_CreatesGraphs(t *testing.T) {
	requireRRDTool(t)

	rrdDir := t.TempDir()
	graphDir := t.TempDir()
	logger := testLogger()

	r, err := NewRRD("ap1", rrdDir, graphDir, "wifi_stations", multiMetrics, logger)
	if err != nil {
		t.Fatalf("NewRRD multi-DS failed: %v", err)
	}
	defer r.file.Close()

	if len(r.graphs) == 0 {
		t.Error("expected graphs to be initialized for multi-DS RRD")
	}

	// Check a graph file exists
	graphPath := filepath.Join(graphDir, "imgs", "ap1", "ap1_wifi_stations_4h.png")
	if _, err := os.Stat(graphPath); os.IsNotExist(err) {
		t.Errorf("expected graph file at %s", graphPath)
	}
}

func TestSafeUpdate_MultiDS_Success(t *testing.T) {
	requireRRDTool(t)

	rrdDir := t.TempDir()
	graphDir := t.TempDir()
	logger := testLogger()

	r, err := NewRRD("ap1", rrdDir, graphDir, "wifi_stations", multiMetrics, logger)
	if err != nil {
		t.Fatalf("NewRRD multi-DS failed: %v", err)
	}
	defer r.file.Close()

	ts := time.Now()
	lastUpdate, err := r.SafeUpdate(ts, []int64{3, 7})
	if err != nil {
		t.Fatalf("SafeUpdate multi-DS failed: %v", err)
	}
	if lastUpdate != ts.Unix() {
		t.Errorf("expected lastUpdate=%d, got %d", ts.Unix(), lastUpdate)
	}
}

func TestSafeUpdate_MultiDS_AcceptsNewer(t *testing.T) {
	requireRRDTool(t)

	rrdDir := t.TempDir()
	graphDir := t.TempDir()
	logger := testLogger()

	r, err := NewRRD("ap1", rrdDir, graphDir, "wifi_stations", multiMetrics, logger)
	if err != nil {
		t.Fatalf("NewRRD multi-DS failed: %v", err)
	}
	defer r.file.Close()

	ts1 := time.Now()
	_, err = r.SafeUpdate(ts1, []int64{3, 7})
	if err != nil {
		t.Fatalf("first SafeUpdate failed: %v", err)
	}

	ts2 := ts1.Add(time.Minute)
	lastUpdate, err := r.SafeUpdate(ts2, []int64{5, 12})
	if err != nil {
		t.Fatalf("second SafeUpdate failed: %v", err)
	}
	if lastUpdate != ts2.Unix() {
		t.Errorf("expected lastUpdate=%d, got %d", ts2.Unix(), lastUpdate)
	}
}

func TestGetLastUpdate_MultiDS_AfterUpdate(t *testing.T) {
	requireRRDTool(t)

	rrdDir := t.TempDir()
	graphDir := t.TempDir()
	logger := testLogger()

	r, err := NewRRD("ap1", rrdDir, graphDir, "wifi_stations", multiMetrics, logger)
	if err != nil {
		t.Fatalf("NewRRD multi-DS failed: %v", err)
	}
	defer r.file.Close()

	ts := time.Now()
	_, err = r.SafeUpdate(ts, []int64{3, 7})
	if err != nil {
		t.Fatalf("SafeUpdate failed: %v", err)
	}

	lastUpdate, err := r.getLastUpdate()
	if err != nil {
		t.Fatalf("getLastUpdate failed: %v", err)
	}

	if lastUpdate != ts.Unix() {
		t.Errorf("expected lastUpdate=%d, got %d", ts.Unix(), lastUpdate)
	}
}

func TestNewRRD_MultiDS_IdempotentCreation(t *testing.T) {
	requireRRDTool(t)

	rrdDir := t.TempDir()
	graphDir := t.TempDir()
	logger := testLogger()

	r1, err := NewRRD("ap1", rrdDir, graphDir, "wifi_stations", multiMetrics, logger)
	if err != nil {
		t.Fatalf("first NewRRD multi-DS failed: %v", err)
	}
	r1.file.Close()

	r2, err := NewRRD("ap1", rrdDir, graphDir, "wifi_stations", multiMetrics, logger)
	if err != nil {
		t.Fatalf("second NewRRD multi-DS failed: %v", err)
	}
	r2.file.Close()
}

func TestNewRRD_StoresMetrics(t *testing.T) {
	requireRRDTool(t)

	rrdDir := t.TempDir()
	graphDir := t.TempDir()
	logger := testLogger()

	r, err := NewRRD("testhost", rrdDir, graphDir, "wifi_stations", multiMetrics, logger)
	if err != nil {
		t.Fatalf("NewRRD failed: %v", err)
	}
	defer r.file.Close()

	if len(r.metrics) != 2 {
		t.Fatalf("expected 2 metrics, got %d", len(r.metrics))
	}
	if r.metrics[0].DSName != "clients2g" {
		t.Errorf("expected first DS 'clients2g', got %q", r.metrics[0].DSName)
	}
	if r.metrics[1].DSName != "clients5g" {
		t.Errorf("expected second DS 'clients5g', got %q", r.metrics[1].DSName)
	}
}
