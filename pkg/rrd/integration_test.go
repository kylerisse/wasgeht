package rrd

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

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

func TestNewRRD_CreatesFile(t *testing.T) {
	requireRRDTool(t)

	rrdDir := t.TempDir()
	graphDir := t.TempDir()
	logger := testLogger()

	r, err := NewRRD("testhost", rrdDir, graphDir, "ping", "latency", "latency", "ms", 1000, logger)
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

	r, err := NewRRD("testhost", rrdDir, graphDir, "ping", "latency", "latency", "ms", 1000, logger)
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

	r, err := NewRRD("testhost", rrdDir, graphDir, "ping", "latency", "latency", "ms", 1000, logger)
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

	r1, err := NewRRD("testhost", rrdDir, graphDir, "ping", "latency", "latency", "ms", 1000, logger)
	if err != nil {
		t.Fatalf("first NewRRD failed: %v", err)
	}
	r1.file.Close()

	// Creating again should not fail — should reuse existing file
	r2, err := NewRRD("testhost", rrdDir, graphDir, "ping", "latency", "latency", "ms", 1000, logger)
	if err != nil {
		t.Fatalf("second NewRRD failed: %v", err)
	}
	r2.file.Close()
}

func TestNewRRD_InvalidRRDDir(t *testing.T) {
	requireRRDTool(t)

	graphDir := t.TempDir()
	logger := testLogger()

	_, err := NewRRD("testhost", "/nonexistent/path", graphDir, "ping", "latency", "latency", "ms", 1000, logger)
	if err == nil {
		t.Error("expected error for nonexistent rrdDir")
	}
}

func TestSafeUpdate_Success(t *testing.T) {
	requireRRDTool(t)

	rrdDir := t.TempDir()
	graphDir := t.TempDir()
	logger := testLogger()

	r, err := NewRRD("testhost", rrdDir, graphDir, "ping", "latency", "latency", "ms", 1000, logger)
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

	r, err := NewRRD("testhost", rrdDir, graphDir, "ping", "latency", "latency", "ms", 1000, logger)
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

	r, err := NewRRD("testhost", rrdDir, graphDir, "ping", "latency", "latency", "ms", 1000, logger)
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

	r, err := NewRRD("testhost", rrdDir, graphDir, "ping", "latency", "latency", "ms", 1000, logger)
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

	r, err := NewRRD("testhost", rrdDir, graphDir, "ping", "latency", "latency", "ms", 1000, logger)
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

	r, err := NewRRD("testhost", rrdDir, graphDir, "ping", "latency", "latency", "ms", 1000, logger)
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

	// scale=0 means no scaling
	r, err := NewRRD("testhost", rrdDir, graphDir, "ping", "rtt", "rtt", "us", 0, logger)
	if err != nil {
		t.Fatalf("NewRRD failed: %v", err)
	}
	defer r.file.Close()

	// Verify graphs were created (they use displayVarName with no scaling)
	if len(r.graphs) == 0 {
		t.Error("expected graphs to be initialized")
	}
}

func TestNewRRD_DifferentCheckTypes(t *testing.T) {
	requireRRDTool(t)

	rrdDir := t.TempDir()
	graphDir := t.TempDir()
	logger := testLogger()

	r1, err := NewRRD("testhost", rrdDir, graphDir, "ping", "latency", "latency", "ms", 1000, logger)
	if err != nil {
		t.Fatalf("NewRRD ping failed: %v", err)
	}
	defer r1.file.Close()

	r2, err := NewRRD("testhost", rrdDir, graphDir, "http", "response", "response time", "ms", 0, logger)
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
