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

// lineMetrics simulates an http check with per-URL data sources.
var lineMetrics = []check.MetricDef{
	{ResultKey: "http://a.com", DSName: "url0", Label: "http://a.com", Unit: "ms", Scale: 1000},
	{ResultKey: "http://b.com", DSName: "url1", Label: "http://b.com", Unit: "ms", Scale: 1000},
}

func TestNewRRD_CreatesFile(t *testing.T) {
	requireRRDTool(t)

	rrdDir := t.TempDir()
	graphDir := t.TempDir()
	logger := testLogger()

	r, err := NewRRD("testhost", rrdDir, graphDir, "ping", singleMetric, "", "", logger)
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

	r, err := NewRRD("testhost", rrdDir, graphDir, "ping", singleMetric, "", "", logger)
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

	r, err := NewRRD("testhost", rrdDir, graphDir, "ping", singleMetric, "", "", logger)
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

func TestNewRRD_Idempotent(t *testing.T) {
	requireRRDTool(t)

	rrdDir := t.TempDir()
	graphDir := t.TempDir()
	logger := testLogger()

	r1, err := NewRRD("testhost", rrdDir, graphDir, "ping", singleMetric, "", "", logger)
	if err != nil {
		t.Fatalf("first NewRRD failed: %v", err)
	}
	r1.file.Close()

	r2, err := NewRRD("testhost", rrdDir, graphDir, "ping", singleMetric, "", "", logger)
	if err != nil {
		t.Fatalf("second NewRRD failed: %v", err)
	}
	r2.file.Close()
}

func TestNewRRD_BadRrdDir(t *testing.T) {
	logger := testLogger()
	_, err := NewRRD("testhost", "/nonexistent/path", "/tmp", "ping", singleMetric, "", "", logger)
	if err == nil {
		t.Error("expected error for nonexistent rrdDir")
	}
}

func TestNewRRD_EmptyMetrics(t *testing.T) {
	logger := testLogger()
	_, err := NewRRD("testhost", t.TempDir(), t.TempDir(), "ping", []check.MetricDef{}, "", "", logger)
	if err == nil {
		t.Error("expected error for empty metrics")
	}
}

func TestNewRRD_PerHostSubdirectory(t *testing.T) {
	requireRRDTool(t)

	rrdDir := t.TempDir()
	graphDir := t.TempDir()
	logger := testLogger()

	r1, err := NewRRD("host-a", rrdDir, graphDir, "ping", singleMetric, "", "", logger)
	if err != nil {
		t.Fatalf("NewRRD for host-a failed: %v", err)
	}
	defer r1.file.Close()

	r2, err := NewRRD("host-b", rrdDir, graphDir, "ping", singleMetric, "", "", logger)
	if err != nil {
		t.Fatalf("NewRRD for host-b failed: %v", err)
	}
	defer r2.file.Close()

	pathA := filepath.Join(rrdDir, "host-a", "ping.rrd")
	pathB := filepath.Join(rrdDir, "host-b", "ping.rrd")

	if _, err := os.Stat(pathA); os.IsNotExist(err) {
		t.Errorf("expected RRD at %s", pathA)
	}
	if _, err := os.Stat(pathB); os.IsNotExist(err) {
		t.Errorf("expected RRD at %s", pathB)
	}
}

func TestNewRRD_MultipleCheckTypes(t *testing.T) {
	requireRRDTool(t)

	rrdDir := t.TempDir()
	graphDir := t.TempDir()
	logger := testLogger()

	httpMetrics := []check.MetricDef{
		{ResultKey: "response_ms", DSName: "response", Label: "response time", Unit: "ms", Scale: 0},
	}

	r1, err := NewRRD("testhost", rrdDir, graphDir, "ping", singleMetric, "", "", logger)
	if err != nil {
		t.Fatalf("NewRRD for ping failed: %v", err)
	}
	defer r1.file.Close()

	r2, err := NewRRD("testhost", rrdDir, graphDir, "http", httpMetrics, "", "", logger)
	if err != nil {
		t.Fatalf("NewRRD for http failed: %v", err)
	}
	defer r2.file.Close()

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

	r, err := NewRRD("ap1", rrdDir, graphDir, "wifi_stations", multiMetrics, "", "", logger)
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

	r, err := NewRRD("ap1", rrdDir, graphDir, "wifi_stations", multiMetrics, "", "", logger)
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

	r, err := NewRRD("ap1", rrdDir, graphDir, "wifi_stations", multiMetrics, "", "", logger)
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

	r, err := NewRRD("ap1", rrdDir, graphDir, "wifi_stations", multiMetrics, "", "", logger)
	if err != nil {
		t.Fatalf("NewRRD multi-DS failed: %v", err)
	}
	defer r.file.Close()

	ts1 := time.Now()
	_, err = r.SafeUpdate(ts1, []int64{3, 7})
	if err != nil {
		t.Fatalf("first SafeUpdate failed: %v", err)
	}

	ts2 := ts1.Add(61 * time.Second)
	lastUpdate, err := r.SafeUpdate(ts2, []int64{5, 10})
	if err != nil {
		t.Fatalf("second SafeUpdate failed: %v", err)
	}
	if lastUpdate != ts2.Unix() {
		t.Errorf("expected lastUpdate=%d, got %d", ts2.Unix(), lastUpdate)
	}
}

// --- Line style graph tests ---

func TestNewRRD_LineStyle_CreatesGraphs(t *testing.T) {
	requireRRDTool(t)

	rrdDir := t.TempDir()
	graphDir := t.TempDir()
	logger := testLogger()

	r, err := NewRRD("qube", rrdDir, graphDir, "http", lineMetrics, check.GraphStyleLine, "response time", logger)
	if err != nil {
		t.Fatalf("NewRRD line-style failed: %v", err)
	}
	defer r.file.Close()

	if len(r.graphs) == 0 {
		t.Error("expected graphs to be initialized for line-style RRD")
	}

	// Check a graph file exists
	graphPath := filepath.Join(graphDir, "imgs", "qube", "qube_http_4h.png")
	if _, err := os.Stat(graphPath); os.IsNotExist(err) {
		t.Errorf("expected graph file at %s", graphPath)
	}

	// Verify graphStyle was threaded through
	if r.graphStyle != check.GraphStyleLine {
		t.Errorf("expected graphStyle %q, got %q", check.GraphStyleLine, r.graphStyle)
	}
}

func TestSafeUpdate_LineStyle_Success(t *testing.T) {
	requireRRDTool(t)

	rrdDir := t.TempDir()
	graphDir := t.TempDir()
	logger := testLogger()

	r, err := NewRRD("qube", rrdDir, graphDir, "http", lineMetrics, check.GraphStyleLine, "response time", logger)
	if err != nil {
		t.Fatalf("NewRRD line-style failed: %v", err)
	}
	defer r.file.Close()

	ts := time.Now()
	lastUpdate, err := r.SafeUpdate(ts, []int64{15000, 22000})
	if err != nil {
		t.Fatalf("SafeUpdate line-style failed: %v", err)
	}
	if lastUpdate != ts.Unix() {
		t.Errorf("expected lastUpdate=%d, got %d", ts.Unix(), lastUpdate)
	}
}

// --- SafeUpdate single DS tests ---

func TestSafeUpdate_SingleDS(t *testing.T) {
	requireRRDTool(t)

	rrdDir := t.TempDir()
	graphDir := t.TempDir()
	logger := testLogger()

	r, err := NewRRD("testhost", rrdDir, graphDir, "ping", singleMetric, "", "", logger)
	if err != nil {
		t.Fatalf("NewRRD failed: %v", err)
	}
	defer r.file.Close()

	ts := time.Now()
	lastUpdate, err := r.SafeUpdate(ts, []int64{12340})
	if err != nil {
		t.Fatalf("SafeUpdate failed: %v", err)
	}
	if lastUpdate != ts.Unix() {
		t.Errorf("expected lastUpdate=%d, got %d", ts.Unix(), lastUpdate)
	}
}

func TestSafeUpdate_RejectsSameTimestamp(t *testing.T) {
	requireRRDTool(t)

	rrdDir := t.TempDir()
	graphDir := t.TempDir()
	logger := testLogger()

	r, err := NewRRD("testhost", rrdDir, graphDir, "ping", singleMetric, "", "", logger)
	if err != nil {
		t.Fatalf("NewRRD failed: %v", err)
	}
	defer r.file.Close()

	ts := time.Now()
	_, err = r.SafeUpdate(ts, []int64{12340})
	if err != nil {
		t.Fatalf("first SafeUpdate failed: %v", err)
	}

	_, err = r.SafeUpdate(ts, []int64{56780})
	if err == nil {
		t.Error("expected error for same timestamp update")
	}
}

func TestSafeUpdate_EmptyValues(t *testing.T) {
	requireRRDTool(t)

	rrdDir := t.TempDir()
	graphDir := t.TempDir()
	logger := testLogger()

	r, err := NewRRD("testhost", rrdDir, graphDir, "ping", singleMetric, "", "", logger)
	if err != nil {
		t.Fatalf("NewRRD failed: %v", err)
	}
	defer r.file.Close()

	ts := time.Now()
	// Empty values should not error (just triggers graph redraw)
	_, err = r.SafeUpdate(ts, []int64{})
	if err != nil {
		t.Fatalf("SafeUpdate with empty values failed: %v", err)
	}
}
