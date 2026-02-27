package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/kylerisse/wasgeht/pkg/check"
	"github.com/kylerisse/wasgeht/pkg/host"
)

func TestHandleAPI_BasicResponse(t *testing.T) {
	s := &Server{
		hosts: map[string]*host.Host{
			"google": {Name: "google"},
		},
		statuses: make(map[string]map[string]*check.Status),
	}

	status := s.getOrCreateStatus("google", "ping")
	status.SetResult(check.Result{
		Success: true,
		Metrics: map[string]int64{"latency_us": 12345},
	})
	status.SetLastUpdate(1700000000)

	req := httptest.NewRequest("GET", "/api", nil)
	w := httptest.NewRecorder()

	s.handleAPI(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("expected application/json, got %q", contentType)
	}

	var body APIResponse
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	google, ok := body.Hosts["google"]
	if !ok {
		t.Fatal("expected google in response")
	}

	pingCheck, ok := google.Checks["ping"]
	if !ok {
		t.Fatal("expected ping check in response")
	}
	if !pingCheck.Alive {
		t.Error("expected ping to be alive")
	}
	if pingCheck.Metrics["latency_us"] != 12345 {
		t.Errorf("expected latency_us=12345, got %d", pingCheck.Metrics["latency_us"])
	}
	if pingCheck.LastUpdate != 1700000000 {
		t.Errorf("expected lastupdate=1700000000, got %d", pingCheck.LastUpdate)
	}
}

func TestHandleAPI_IncludesHostStatus(t *testing.T) {
	s := &Server{
		hosts: map[string]*host.Host{
			"uphost":   {Name: "uphost"},
			"downhost": {Name: "downhost"},
			"newhost":  {Name: "newhost"},
		},
		statuses: make(map[string]map[string]*check.Status),
	}

	req := httptest.NewRequest("GET", "/api", nil)
	w := httptest.NewRecorder()

	s.handleAPI(w, req)

	var body APIResponse
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	for name, host := range body.Hosts {
		if host.Status != HostStatusUnconfigured {
			t.Errorf("host %q: expected status unconfigured, got %q", name, host.Status)
		}
	}
}

func TestHandleAPI_EmptyHosts(t *testing.T) {
	s := &Server{
		hosts:    make(map[string]*host.Host),
		statuses: make(map[string]map[string]*check.Status),
	}

	req := httptest.NewRequest("GET", "/api", nil)
	w := httptest.NewRecorder()

	s.handleAPI(w, req)

	var body APIResponse
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(body.Hosts) != 0 {
		t.Errorf("expected empty hosts, got %d", len(body.Hosts))
	}
}

func TestHandleAPI_Envelope(t *testing.T) {
	s := &Server{
		hosts: map[string]*host.Host{
			"router": {Name: "router"},
		},
		statuses: make(map[string]map[string]*check.Status),
	}

	before := time.Now().Unix()

	req := httptest.NewRequest("GET", "/api", nil)
	w := httptest.NewRecorder()
	s.handleAPI(w, req)

	after := time.Now().Unix()

	var body APIResponse
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if body.GeneratedAt < before || body.GeneratedAt > after {
		t.Errorf("generated_at %d not between %d and %d", body.GeneratedAt, before, after)
	}

	if _, ok := body.Hosts["router"]; !ok {
		t.Error("expected router in hosts")
	}
}

func TestHandleSummaryAPI_Empty(t *testing.T) {
	s := &Server{
		hosts:    make(map[string]*host.Host),
		statuses: make(map[string]map[string]*check.Status),
	}

	req := httptest.NewRequest("GET", "/api/summary", nil)
	w := httptest.NewRecorder()
	s.handleSummaryAPI(w, req)

	if w.Result().StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Result().StatusCode)
	}

	var body SummaryResponse
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if body.Total != 0 {
		t.Errorf("expected total=0, got %d", body.Total)
	}
	if len(body.ByStatus) != 6 {
		t.Errorf("expected 6 status entries, got %d", len(body.ByStatus))
	}
}

func TestHandleSummaryAPI_Counts(t *testing.T) {
	s := &Server{
		hosts: map[string]*host.Host{
			"up1":   {Name: "up1"},
			"up2":   {Name: "up2"},
			"down1": {Name: "down1"},
			"bare":  {Name: "bare"},
		},
		statuses: make(map[string]map[string]*check.Status),
	}

	for _, name := range []string{"up1", "up2"} {
		st := s.getOrCreateStatus(name, "ping")
		st.SetResult(check.Result{Success: true})
		st.SetLastUpdate(time.Now().Unix())
	}
	downSt := s.getOrCreateStatus("down1", "ping")
	downSt.SetResult(check.Result{Success: false})
	downSt.SetLastUpdate(time.Now().Unix())
	// bare has no checks → unconfigured

	req := httptest.NewRequest("GET", "/api/summary", nil)
	w := httptest.NewRecorder()
	s.handleSummaryAPI(w, req)

	var body SummaryResponse
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if body.Total != 4 {
		t.Errorf("expected total=4, got %d", body.Total)
	}
	if body.ByStatus[HostStatusUp] != 2 {
		t.Errorf("expected up=2, got %d", body.ByStatus[HostStatusUp])
	}
	if body.ByStatus[HostStatusDown] != 1 {
		t.Errorf("expected down=1, got %d", body.ByStatus[HostStatusDown])
	}
	if body.ByStatus[HostStatusUnconfigured] != 1 {
		t.Errorf("expected unconfigured=1, got %d", body.ByStatus[HostStatusUnconfigured])
	}
}

func TestHandleSummaryAPI_TagFilter(t *testing.T) {
	s := &Server{
		hosts: map[string]*host.Host{
			"ap1":    {Name: "ap1", Tags: map[string]string{"category": "ap"}},
			"ap2":    {Name: "ap2", Tags: map[string]string{"category": "ap"}},
			"router": {Name: "router", Tags: map[string]string{"category": "router"}},
		},
		statuses: make(map[string]map[string]*check.Status),
	}

	req := httptest.NewRequest("GET", "/api/summary?tag=category:ap", nil)
	w := httptest.NewRecorder()
	s.handleSummaryAPI(w, req)

	var body SummaryResponse
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if body.Total != 2 {
		t.Errorf("expected total=2, got %d", body.Total)
	}
}

func TestHandleSummaryAPI_StatusFilter(t *testing.T) {
	s := &Server{
		hosts: map[string]*host.Host{
			"up1":   {Name: "up1"},
			"down1": {Name: "down1"},
		},
		statuses: make(map[string]map[string]*check.Status),
	}

	upSt := s.getOrCreateStatus("up1", "ping")
	upSt.SetResult(check.Result{Success: true})
	upSt.SetLastUpdate(time.Now().Unix())

	downSt := s.getOrCreateStatus("down1", "ping")
	downSt.SetResult(check.Result{Success: false})
	downSt.SetLastUpdate(time.Now().Unix())

	req := httptest.NewRequest("GET", "/api/summary?status=down", nil)
	w := httptest.NewRecorder()
	s.handleSummaryAPI(w, req)

	var body SummaryResponse
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if body.Total != 1 {
		t.Errorf("expected total=1, got %d", body.Total)
	}
	if body.ByStatus[HostStatusDown] != 1 {
		t.Errorf("expected down=1, got %d", body.ByStatus[HostStatusDown])
	}
	if body.ByStatus[HostStatusUp] != 0 {
		t.Errorf("expected up=0, got %d", body.ByStatus[HostStatusUp])
	}
}

func TestHandleSummaryAPI_InvalidFilter(t *testing.T) {
	s := &Server{
		hosts:    make(map[string]*host.Host),
		statuses: make(map[string]map[string]*check.Status),
	}

	req := httptest.NewRequest("GET", "/api/summary?status=bogus", nil)
	w := httptest.NewRecorder()
	s.handleSummaryAPI(w, req)

	if w.Result().StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Result().StatusCode)
	}
}

func TestHandleAPI_NoTagFilter_ReturnsAll(t *testing.T) {
	s := &Server{
		hosts: map[string]*host.Host{
			"ap1":    {Name: "ap1", Tags: map[string]string{"category": "ap"}},
			"router": {Name: "router", Tags: map[string]string{"category": "router"}},
		},
		statuses: make(map[string]map[string]*check.Status),
	}

	req := httptest.NewRequest("GET", "/api", nil)
	w := httptest.NewRecorder()
	s.handleAPI(w, req)

	var body APIResponse
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if len(body.Hosts) != 2 {
		t.Errorf("expected 2 hosts, got %d", len(body.Hosts))
	}
}

func TestHandleAPI_TagFilter_SingleMatch(t *testing.T) {
	s := &Server{
		hosts: map[string]*host.Host{
			"ap1":    {Name: "ap1", Tags: map[string]string{"category": "ap"}},
			"ap2":    {Name: "ap2", Tags: map[string]string{"category": "ap"}},
			"router": {Name: "router", Tags: map[string]string{"category": "router"}},
		},
		statuses: make(map[string]map[string]*check.Status),
	}

	req := httptest.NewRequest("GET", "/api?tag=category:ap", nil)
	w := httptest.NewRecorder()
	s.handleAPI(w, req)

	var body APIResponse
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if len(body.Hosts) != 2 {
		t.Errorf("expected 2 hosts, got %d", len(body.Hosts))
	}
	if _, ok := body.Hosts["router"]; ok {
		t.Error("router should be excluded")
	}
}

func TestHandleAPI_TagFilter_MultipleAnded(t *testing.T) {
	s := &Server{
		hosts: map[string]*host.Host{
			"ap1": {Name: "ap1", Tags: map[string]string{"category": "ap", "building": "expo"}},
			"ap2": {Name: "ap2", Tags: map[string]string{"category": "ap", "building": "conference"}},
		},
		statuses: make(map[string]map[string]*check.Status),
	}

	req := httptest.NewRequest("GET", "/api?tag=category:ap&tag=building:expo", nil)
	w := httptest.NewRecorder()
	s.handleAPI(w, req)

	var body APIResponse
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if len(body.Hosts) != 1 {
		t.Errorf("expected 1 host, got %d", len(body.Hosts))
	}
	if _, ok := body.Hosts["ap1"]; !ok {
		t.Error("expected ap1 in results")
	}
}

func TestHandleAPI_TagFilter_ExcludesUntaggedHosts(t *testing.T) {
	s := &Server{
		hosts: map[string]*host.Host{
			"ap1":    {Name: "ap1", Tags: map[string]string{"category": "ap"}},
			"nohost": {Name: "nohost"},
		},
		statuses: make(map[string]map[string]*check.Status),
	}

	req := httptest.NewRequest("GET", "/api?tag=category:ap", nil)
	w := httptest.NewRecorder()
	s.handleAPI(w, req)

	var body APIResponse
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if len(body.Hosts) != 1 {
		t.Errorf("expected 1 host, got %d", len(body.Hosts))
	}
	if _, ok := body.Hosts["nohost"]; ok {
		t.Error("untagged host should be excluded")
	}
}

func TestHandleAPI_TagFilter_MalformedReturns400(t *testing.T) {
	s := &Server{
		hosts:    make(map[string]*host.Host),
		statuses: make(map[string]map[string]*check.Status),
	}

	for _, bad := range []string{"nocodon", ":missingkey", "missingval:"} {
		req := httptest.NewRequest("GET", "/api?tag="+bad, nil)
		w := httptest.NewRecorder()
		s.handleAPI(w, req)
		if w.Result().StatusCode != http.StatusBadRequest {
			t.Errorf("tag=%q: expected 400, got %d", bad, w.Result().StatusCode)
		}
	}
}

func TestHandleAPI_StatusFilter_SingleStatus(t *testing.T) {
	s := &Server{
		hosts: map[string]*host.Host{
			"up1":   {Name: "up1"},
			"down1": {Name: "down1"},
			"bare":  {Name: "bare"},
		},
		statuses: make(map[string]map[string]*check.Status),
	}

	upStatus := s.getOrCreateStatus("up1", "ping")
	upStatus.SetResult(check.Result{Success: true})
	upStatus.SetLastUpdate(time.Now().Unix())

	downStatus := s.getOrCreateStatus("down1", "ping")
	downStatus.SetResult(check.Result{Success: false})
	downStatus.SetLastUpdate(time.Now().Unix())

	req := httptest.NewRequest("GET", "/api?status=up", nil)
	w := httptest.NewRecorder()
	s.handleAPI(w, req)

	var body APIResponse
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if len(body.Hosts) != 1 {
		t.Errorf("expected 1 host, got %d", len(body.Hosts))
	}
	if _, ok := body.Hosts["up1"]; !ok {
		t.Error("expected up1 in results")
	}
}

func TestHandleAPI_StatusFilter_MultipleOrMatchd(t *testing.T) {
	s := &Server{
		hosts: map[string]*host.Host{
			"up1":   {Name: "up1"},
			"down1": {Name: "down1"},
			"bare":  {Name: "bare"},
		},
		statuses: make(map[string]map[string]*check.Status),
	}

	upStatus := s.getOrCreateStatus("up1", "ping")
	upStatus.SetResult(check.Result{Success: true})
	upStatus.SetLastUpdate(time.Now().Unix())

	downStatus := s.getOrCreateStatus("down1", "ping")
	downStatus.SetResult(check.Result{Success: false})
	downStatus.SetLastUpdate(time.Now().Unix())

	req := httptest.NewRequest("GET", "/api?status=up&status=down", nil)
	w := httptest.NewRecorder()
	s.handleAPI(w, req)

	var body APIResponse
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if len(body.Hosts) != 2 {
		t.Errorf("expected 2 hosts, got %d", len(body.Hosts))
	}
	if _, ok := body.Hosts["bare"]; ok {
		t.Error("bare (unknown) should be excluded")
	}
}

func TestHandleAPI_StatusAndTagFilters_Combined(t *testing.T) {
	s := &Server{
		hosts: map[string]*host.Host{
			"ap1":    {Name: "ap1", Tags: map[string]string{"category": "ap"}},
			"ap2":    {Name: "ap2", Tags: map[string]string{"category": "ap"}},
			"router": {Name: "router", Tags: map[string]string{"category": "router"}},
		},
		statuses: make(map[string]map[string]*check.Status),
	}

	for _, name := range []string{"ap1", "router"} {
		st := s.getOrCreateStatus(name, "ping")
		st.SetResult(check.Result{Success: true})
		st.SetLastUpdate(time.Now().Unix())
	}
	// ap2 has no status set — will be unknown

	req := httptest.NewRequest("GET", "/api?tag=category:ap&status=up", nil)
	w := httptest.NewRecorder()
	s.handleAPI(w, req)

	var body APIResponse
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if len(body.Hosts) != 1 {
		t.Errorf("expected 1 host, got %d", len(body.Hosts))
	}
	if _, ok := body.Hosts["ap1"]; !ok {
		t.Error("expected ap1 in results")
	}
}

func TestHandleAPI_StatusFilter_InvalidReturns400(t *testing.T) {
	s := &Server{
		hosts:    make(map[string]*host.Host),
		statuses: make(map[string]map[string]*check.Status),
	}

	req := httptest.NewRequest("GET", "/api?status=sideways", nil)
	w := httptest.NewRecorder()
	s.handleAPI(w, req)

	if w.Result().StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Result().StatusCode)
	}
}

func TestHandleAPI_HostnameFilter_SingleMatch(t *testing.T) {
	s := &Server{
		hosts: map[string]*host.Host{
			"router": {Name: "router"},
			"ap1":    {Name: "ap1"},
			"ap2":    {Name: "ap2"},
		},
		statuses: make(map[string]map[string]*check.Status),
	}

	req := httptest.NewRequest("GET", "/api?hostname=ap1", nil)
	w := httptest.NewRecorder()
	s.handleAPI(w, req)

	var body APIResponse
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if len(body.Hosts) != 1 {
		t.Errorf("expected 1 host, got %d", len(body.Hosts))
	}
	if _, ok := body.Hosts["ap1"]; !ok {
		t.Error("expected ap1 in results")
	}
}

func TestHandleAPI_HostnameFilter_MultipleOrMatched(t *testing.T) {
	s := &Server{
		hosts: map[string]*host.Host{
			"router": {Name: "router"},
			"ap1":    {Name: "ap1"},
			"ap2":    {Name: "ap2"},
		},
		statuses: make(map[string]map[string]*check.Status),
	}

	req := httptest.NewRequest("GET", "/api?hostname=ap1&hostname=ap2", nil)
	w := httptest.NewRecorder()
	s.handleAPI(w, req)

	var body APIResponse
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if len(body.Hosts) != 2 {
		t.Errorf("expected 2 hosts, got %d", len(body.Hosts))
	}
	if _, ok := body.Hosts["router"]; ok {
		t.Error("router should be excluded")
	}
}

func TestHandleAPI_HostnameFilter_NoMatch(t *testing.T) {
	s := &Server{
		hosts: map[string]*host.Host{
			"router": {Name: "router"},
		},
		statuses: make(map[string]map[string]*check.Status),
	}

	req := httptest.NewRequest("GET", "/api?hostname=nonexistent", nil)
	w := httptest.NewRecorder()
	s.handleAPI(w, req)

	if w.Result().StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Result().StatusCode)
	}

	var body APIResponse
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if len(body.Hosts) != 0 {
		t.Errorf("expected 0 hosts, got %d", len(body.Hosts))
	}
}

func TestHandleAPI_HostnameAndStatusFilters_Combined(t *testing.T) {
	s := &Server{
		hosts: map[string]*host.Host{
			"ap1": {Name: "ap1"},
			"ap2": {Name: "ap2"},
		},
		statuses: make(map[string]map[string]*check.Status),
	}

	st := s.getOrCreateStatus("ap1", "ping")
	st.SetResult(check.Result{Success: true})
	st.SetLastUpdate(time.Now().Unix())
	// ap2 has no checks — unconfigured

	req := httptest.NewRequest("GET", "/api?hostname=ap1&hostname=ap2&status=up", nil)
	w := httptest.NewRecorder()
	s.handleAPI(w, req)

	var body APIResponse
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if len(body.Hosts) != 1 {
		t.Errorf("expected 1 host, got %d", len(body.Hosts))
	}
	if _, ok := body.Hosts["ap1"]; !ok {
		t.Error("expected ap1 in results")
	}
}

func TestHandleSummaryAPI_HostnameFilter(t *testing.T) {
	s := &Server{
		hosts: map[string]*host.Host{
			"router": {Name: "router"},
			"ap1":    {Name: "ap1"},
			"ap2":    {Name: "ap2"},
		},
		statuses: make(map[string]map[string]*check.Status),
	}

	req := httptest.NewRequest("GET", "/api/summary?hostname=ap1&hostname=ap2", nil)
	w := httptest.NewRecorder()
	s.handleSummaryAPI(w, req)

	var body SummaryResponse
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if body.Total != 2 {
		t.Errorf("expected total=2, got %d", body.Total)
	}
}

func TestHandleHostAPI_Found(t *testing.T) {
	s := &Server{
		hosts: map[string]*host.Host{
			"ap1": {Name: "ap1", Tags: map[string]string{"category": "ap"}},
		},
		statuses: make(map[string]map[string]*check.Status),
	}

	status := s.getOrCreateStatus("ap1", "ping")
	status.SetResult(check.Result{
		Success: true,
		Metrics: map[string]int64{"latency_us": 5000},
	})
	status.SetLastUpdate(time.Now().Unix())

	req := httptest.NewRequest("GET", "/api/hosts/ap1", nil)
	req.SetPathValue("hostname", "ap1")
	w := httptest.NewRecorder()

	s.handleHostAPI(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var body HostAPIResponse
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if body.Status != HostStatusUp {
		t.Errorf("expected status up, got %q", body.Status)
	}
	if body.Tags["category"] != "ap" {
		t.Errorf("expected category=ap, got %q", body.Tags["category"])
	}
	if _, ok := body.Checks["ping"]; !ok {
		t.Error("expected ping check in response")
	}
}

func TestHandleHostAPI_NotFound(t *testing.T) {
	s := &Server{
		hosts:    make(map[string]*host.Host),
		statuses: make(map[string]map[string]*check.Status),
	}

	req := httptest.NewRequest("GET", "/api/hosts/nobody", nil)
	req.SetPathValue("hostname", "nobody")
	w := httptest.NewRecorder()

	s.handleHostAPI(w, req)

	if w.Result().StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Result().StatusCode)
	}
}

func TestHandleAPI_TagsPassthrough(t *testing.T) {
	s := &Server{
		hosts: map[string]*host.Host{
			"ap1": {Name: "ap1", Tags: map[string]string{"category": "ap", "building": "expo"}},
			"router": {Name: "router"},
		},
		statuses: make(map[string]map[string]*check.Status),
	}

	req := httptest.NewRequest("GET", "/api", nil)
	w := httptest.NewRecorder()
	s.handleAPI(w, req)

	var body APIResponse
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	ap1 := body.Hosts["ap1"]
	if ap1.Tags["category"] != "ap" {
		t.Errorf("expected category=ap, got %q", ap1.Tags["category"])
	}
	if ap1.Tags["building"] != "expo" {
		t.Errorf("expected building=expo, got %q", ap1.Tags["building"])
	}

	router := body.Hosts["router"]
	if router.Tags != nil {
		t.Error("expected nil tags for router")
	}
}
