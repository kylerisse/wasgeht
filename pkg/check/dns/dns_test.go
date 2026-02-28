package dns

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/kylerisse/wasgeht/pkg/check"
	"github.com/miekg/dns"
)

// startTestServer starts an in-process UDP DNS server on a random port.
// The provided handler is called for every incoming query. The server
// is shut down automatically when the test ends.
func startTestServer(t *testing.T, handler func(dns.ResponseWriter, *dns.Msg)) string {
	t.Helper()
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	srv := &dns.Server{PacketConn: pc, Handler: dns.HandlerFunc(handler)}
	go func() { _ = srv.ActivateAndServe() }()
	t.Cleanup(func() { _ = srv.Shutdown() })
	return pc.LocalAddr().String()
}

// --- New / construction tests ---

func TestNew_Valid(t *testing.T) {
	queries := []queryConfig{
		{name: "example.com", qtype: dns.TypeA, expect: "93.184.216.34", resultKey: "example.com", dsName: "q0"},
	}
	chk, err := New("127.0.0.1:53", queries)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if chk.Type() != TypeName {
		t.Errorf("expected type %q, got %q", TypeName, chk.Type())
	}
}

func TestNew_EmptyServer(t *testing.T) {
	queries := []queryConfig{
		{name: "example.com", qtype: dns.TypeA, expect: "93.184.216.34", resultKey: "example.com", dsName: "q0"},
	}
	_, err := New("", queries)
	if err == nil {
		t.Error("expected error for empty server")
	}
}

func TestNew_NoQueries(t *testing.T) {
	_, err := New("127.0.0.1:53", []queryConfig{})
	if err == nil {
		t.Error("expected error for empty queries")
	}
}

func TestNew_WithTimeout(t *testing.T) {
	queries := []queryConfig{
		{name: "example.com", qtype: dns.TypeA, expect: "1.2.3.4", resultKey: "example.com", dsName: "q0"},
	}
	chk, err := New("127.0.0.1:53", queries, WithTimeout(7*time.Second))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if chk.timeout != 7*time.Second {
		t.Errorf("expected timeout 7s, got %v", chk.timeout)
	}
}

func TestNew_WithTimeoutZero(t *testing.T) {
	queries := []queryConfig{
		{name: "example.com", qtype: dns.TypeA, expect: "1.2.3.4", resultKey: "example.com", dsName: "q0"},
	}
	_, err := New("127.0.0.1:53", queries, WithTimeout(0))
	if err == nil {
		t.Error("expected error for zero timeout")
	}
}

// --- Describe tests ---

func TestDescribe_SingleQuery(t *testing.T) {
	queries := []queryConfig{
		{name: "router.example.com", qtype: dns.TypeA, expect: "192.168.168.1", resultKey: "router.example.com", dsName: "q0"},
	}
	chk, err := New("127.0.0.1:53", queries)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	desc := chk.Describe()
	if desc.Label != "dns" {
		t.Errorf("expected Descriptor.Label 'dns', got %q", desc.Label)
	}
	if len(desc.Metrics) != 1 {
		t.Fatalf("expected 1 metric, got %d", len(desc.Metrics))
	}
	m := desc.Metrics[0]
	if m.DSName != "q0" {
		t.Errorf("expected DSName 'q0', got %q", m.DSName)
	}
	if m.ResultKey != "router.example.com" {
		t.Errorf("expected ResultKey 'router.example.com', got %q", m.ResultKey)
	}
	if m.Unit != "ms" {
		t.Errorf("expected Unit 'ms', got %q", m.Unit)
	}
	if m.Scale != 1000 {
		t.Errorf("expected Scale 1000, got %d", m.Scale)
	}
}

func TestDescribe_MultipleQueries(t *testing.T) {
	queries := []queryConfig{
		{name: "a.example.com", qtype: dns.TypeA, expect: "1.2.3.4", resultKey: "a.example.com", dsName: "q0"},
		{name: "b.example.com", qtype: dns.TypeA, expect: "5.6.7.8", resultKey: "b.example.com", dsName: "q1"},
		{name: "1.2.3.4.in-addr.arpa", qtype: dns.TypePTR, expect: "a.example.com.", resultKey: "1.2.3.4.in-addr.arpa", dsName: "q2"},
	}
	chk, err := New("127.0.0.1:53", queries)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	desc := chk.Describe()
	if len(desc.Metrics) != 3 {
		t.Fatalf("expected 3 metrics, got %d", len(desc.Metrics))
	}
	for i, q := range queries {
		if desc.Metrics[i].DSName != q.dsName {
			t.Errorf("metric %d: expected DSName %q, got %q", i, q.dsName, desc.Metrics[i].DSName)
		}
		if desc.Metrics[i].ResultKey != q.resultKey {
			t.Errorf("metric %d: expected ResultKey %q, got %q", i, q.resultKey, desc.Metrics[i].ResultKey)
		}
	}
}

// --- Factory tests ---

func TestFactory_MinimalConfig(t *testing.T) {
	config := map[string]any{
		"server": "127.0.0.1:53",
		"queries": []any{
			map[string]any{"name": "example.com", "type": "A", "expect": "93.184.216.34"},
		},
	}
	chk, err := Factory(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	dnsChk := chk.(*Check)
	if dnsChk.timeout != DefaultTimeout {
		t.Errorf("expected default timeout %v, got %v", DefaultTimeout, dnsChk.timeout)
	}
}

func TestFactory_WithTimeout(t *testing.T) {
	config := map[string]any{
		"server":  "127.0.0.1:53",
		"timeout": "7s",
		"queries": []any{
			map[string]any{"name": "example.com", "type": "A", "expect": "1.2.3.4"},
		},
	}
	chk, err := Factory(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	dnsChk := chk.(*Check)
	if dnsChk.timeout != 7*time.Second {
		t.Errorf("expected timeout 7s, got %v", dnsChk.timeout)
	}
}

func TestFactory_MissingServer(t *testing.T) {
	config := map[string]any{
		"queries": []any{
			map[string]any{"name": "example.com", "type": "A", "expect": "1.2.3.4"},
		},
	}
	_, err := Factory(config)
	if err == nil {
		t.Error("expected error for missing server")
	}
}

func TestFactory_EmptyServer(t *testing.T) {
	config := map[string]any{
		"server": "",
		"queries": []any{
			map[string]any{"name": "example.com", "type": "A", "expect": "1.2.3.4"},
		},
	}
	_, err := Factory(config)
	if err == nil {
		t.Error("expected error for empty server")
	}
}

func TestFactory_WrongServerType(t *testing.T) {
	config := map[string]any{
		"server": 12345,
		"queries": []any{
			map[string]any{"name": "example.com", "type": "A", "expect": "1.2.3.4"},
		},
	}
	_, err := Factory(config)
	if err == nil {
		t.Error("expected error for non-string server")
	}
}

func TestFactory_MissingQueries(t *testing.T) {
	config := map[string]any{
		"server": "127.0.0.1:53",
	}
	_, err := Factory(config)
	if err == nil {
		t.Error("expected error for missing queries")
	}
}

func TestFactory_EmptyQueries(t *testing.T) {
	config := map[string]any{
		"server":  "127.0.0.1:53",
		"queries": []any{},
	}
	_, err := Factory(config)
	if err == nil {
		t.Error("expected error for empty queries")
	}
}

func TestFactory_WrongQueriesType(t *testing.T) {
	config := map[string]any{
		"server":  "127.0.0.1:53",
		"queries": "not-a-list",
	}
	_, err := Factory(config)
	if err == nil {
		t.Error("expected error for non-list queries")
	}
}

func TestFactory_QueryMissingName(t *testing.T) {
	config := map[string]any{
		"server": "127.0.0.1:53",
		"queries": []any{
			map[string]any{"type": "A", "expect": "1.2.3.4"},
		},
	}
	_, err := Factory(config)
	if err == nil {
		t.Error("expected error for missing query name")
	}
}

func TestFactory_QueryMissingType(t *testing.T) {
	config := map[string]any{
		"server": "127.0.0.1:53",
		"queries": []any{
			map[string]any{"name": "example.com", "expect": "1.2.3.4"},
		},
	}
	_, err := Factory(config)
	if err == nil {
		t.Error("expected error for missing query type")
	}
}

func TestFactory_QueryMissingExpect(t *testing.T) {
	config := map[string]any{
		"server": "127.0.0.1:53",
		"queries": []any{
			map[string]any{"name": "example.com", "type": "A"},
		},
	}
	_, err := Factory(config)
	if err == nil {
		t.Error("expected error for missing query expect")
	}
}

func TestFactory_QueryUnsupportedType(t *testing.T) {
	config := map[string]any{
		"server": "127.0.0.1:53",
		"queries": []any{
			map[string]any{"name": "example.com", "type": "MX", "expect": "mail.example.com."},
		},
	}
	_, err := Factory(config)
	if err == nil {
		t.Error("expected error for unsupported query type")
	}
}

func TestFactory_InvalidTimeout(t *testing.T) {
	config := map[string]any{
		"server":  "127.0.0.1:53",
		"timeout": "not-a-duration",
		"queries": []any{
			map[string]any{"name": "example.com", "type": "A", "expect": "1.2.3.4"},
		},
	}
	_, err := Factory(config)
	if err == nil {
		t.Error("expected error for invalid timeout")
	}
}

func TestFactory_WrongTimeoutType(t *testing.T) {
	config := map[string]any{
		"server":  "127.0.0.1:53",
		"timeout": 5,
		"queries": []any{
			map[string]any{"name": "example.com", "type": "A", "expect": "1.2.3.4"},
		},
	}
	_, err := Factory(config)
	if err == nil {
		t.Error("expected error for non-string timeout")
	}
}

func TestFactory_QueryItemNotObject(t *testing.T) {
	config := map[string]any{
		"server":  "127.0.0.1:53",
		"queries": []any{"not-an-object"},
	}
	_, err := Factory(config)
	if err == nil {
		t.Error("expected error for non-object query item")
	}
}

func TestFactory_DSNameIndexing(t *testing.T) {
	config := map[string]any{
		"server": "127.0.0.1:53",
		"queries": []any{
			map[string]any{"name": "a.example.com", "type": "A", "expect": "1.1.1.1"},
			map[string]any{"name": "b.example.com", "type": "A", "expect": "2.2.2.2"},
			map[string]any{"name": "c.example.com", "type": "A", "expect": "3.3.3.3"},
		},
	}
	chk, err := Factory(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	desc := chk.Describe()
	expected := []string{"q0", "q1", "q2"}
	for i, e := range expected {
		if desc.Metrics[i].DSName != e {
			t.Errorf("metric %d: expected DSName %q, got %q", i, e, desc.Metrics[i].DSName)
		}
	}
}

func TestFactory_QueryTypeCaseInsensitive(t *testing.T) {
	for _, typeStr := range []string{"a", "A", "aaaa", "AAAA", "ptr", "PTR"} {
		config := map[string]any{
			"server": "127.0.0.1:53",
			"queries": []any{
				map[string]any{"name": "example.com", "type": typeStr, "expect": "1.2.3.4"},
			},
		}
		_, err := Factory(config)
		if err != nil {
			t.Errorf("type %q: unexpected error: %v", typeStr, err)
		}
	}
}

// --- normalizeIP tests ---

func TestNormalizeIP_IPv4(t *testing.T) {
	if normalizeIP("192.168.1.1") != "192.168.1.1" {
		t.Error("expected unchanged IPv4")
	}
}

func TestNormalizeIP_Invalid(t *testing.T) {
	if normalizeIP("not-an-ip") != "not-an-ip" {
		t.Error("expected unchanged invalid string")
	}
}

// --- normalizeFQDN tests ---

func TestNormalizeFQDN_WithDot(t *testing.T) {
	if normalizeFQDN("example.com.") != "example.com" {
		t.Error("expected trailing dot stripped")
	}
}

func TestNormalizeFQDN_WithoutDot(t *testing.T) {
	if normalizeFQDN("example.com") != "example.com" {
		t.Error("expected unchanged when no trailing dot")
	}
}

// --- validateAnswer tests ---

func TestValidateAnswer_AMatch(t *testing.T) {
	rrs := []dns.RR{
		&dns.A{Hdr: dns.RR_Header{Rrtype: dns.TypeA}, A: net.ParseIP("192.168.168.1")},
	}
	if err := validateAnswer(rrs, dns.TypeA, "192.168.168.1"); err != nil {
		t.Errorf("expected match, got: %v", err)
	}
}

func TestValidateAnswer_ANoMatch(t *testing.T) {
	rrs := []dns.RR{
		&dns.A{Hdr: dns.RR_Header{Rrtype: dns.TypeA}, A: net.ParseIP("10.0.0.1")},
	}
	if err := validateAnswer(rrs, dns.TypeA, "192.168.168.1"); err == nil {
		t.Error("expected mismatch error")
	}
}

func TestValidateAnswer_AAAAMatch(t *testing.T) {
	rrs := []dns.RR{
		&dns.AAAA{Hdr: dns.RR_Header{Rrtype: dns.TypeAAAA}, AAAA: net.ParseIP("2001:db8::1")},
	}
	if err := validateAnswer(rrs, dns.TypeAAAA, "2001:db8::1"); err != nil {
		t.Errorf("expected match, got: %v", err)
	}
}

func TestValidateAnswer_PTRMatchWithTrailingDot(t *testing.T) {
	rrs := []dns.RR{
		&dns.PTR{Hdr: dns.RR_Header{Rrtype: dns.TypePTR}, Ptr: "router.example.com."},
	}
	if err := validateAnswer(rrs, dns.TypePTR, "router.example.com."); err != nil {
		t.Errorf("expected match, got: %v", err)
	}
}

func TestValidateAnswer_PTRMatchWithoutTrailingDot(t *testing.T) {
	rrs := []dns.RR{
		&dns.PTR{Hdr: dns.RR_Header{Rrtype: dns.TypePTR}, Ptr: "router.example.com."},
	}
	if err := validateAnswer(rrs, dns.TypePTR, "router.example.com"); err != nil {
		t.Errorf("expected match regardless of trailing dot, got: %v", err)
	}
}

func TestValidateAnswer_EmptyAnswer(t *testing.T) {
	if err := validateAnswer([]dns.RR{}, dns.TypeA, "1.2.3.4"); err == nil {
		t.Error("expected error for empty answer")
	}
}

func TestValidateAnswer_WrongType(t *testing.T) {
	rrs := []dns.RR{
		&dns.AAAA{Hdr: dns.RR_Header{Rrtype: dns.TypeAAAA}, AAAA: net.ParseIP("::1")},
	}
	if err := validateAnswer(rrs, dns.TypeA, "::1"); err == nil {
		t.Error("expected error when RR type does not match query type")
	}
}

// --- Run integration tests using in-process test server ---

func TestRun_AQuery_Success(t *testing.T) {
	addr := startTestServer(t, func(w dns.ResponseWriter, r *dns.Msg) {
		m := new(dns.Msg)
		m.SetReply(r)
		m.Answer = append(m.Answer, &dns.A{
			Hdr: dns.RR_Header{Name: r.Question[0].Name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60},
			A:   net.ParseIP("192.168.168.1"),
		})
		_ = w.WriteMsg(m)
	})

	queries := []queryConfig{
		{name: "router.example.com", qtype: dns.TypeA, expect: "192.168.168.1", resultKey: "router.example.com", dsName: "q0"},
	}
	chk, err := New(addr, queries)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result := chk.Run(context.Background())
	if !result.Success {
		t.Errorf("expected success, got failure: %v", result.Err)
	}
	if p := result.Metrics["router.example.com"]; p == nil || *p <= 0 {
		t.Errorf("expected positive RTT, got %v", result.Metrics["router.example.com"])
	}
	if result.Timestamp.IsZero() {
		t.Error("expected non-zero timestamp")
	}
}

func TestRun_PTRQuery_Success(t *testing.T) {
	addr := startTestServer(t, func(w dns.ResponseWriter, r *dns.Msg) {
		m := new(dns.Msg)
		m.SetReply(r)
		m.Answer = append(m.Answer, &dns.PTR{
			Hdr: dns.RR_Header{Name: r.Question[0].Name, Rrtype: dns.TypePTR, Class: dns.ClassINET, Ttl: 60},
			Ptr: "router.example.com.",
		})
		_ = w.WriteMsg(m)
	})

	queries := []queryConfig{
		{name: "1.168.168.192.in-addr.arpa", qtype: dns.TypePTR, expect: "router.example.com.", resultKey: "1.168.168.192.in-addr.arpa", dsName: "q0"},
	}
	chk, err := New(addr, queries)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result := chk.Run(context.Background())
	if !result.Success {
		t.Errorf("expected success, got failure: %v", result.Err)
	}
}

func TestRun_AAAAQuery_Success(t *testing.T) {
	addr := startTestServer(t, func(w dns.ResponseWriter, r *dns.Msg) {
		m := new(dns.Msg)
		m.SetReply(r)
		m.Answer = append(m.Answer, &dns.AAAA{
			Hdr:  dns.RR_Header{Name: r.Question[0].Name, Rrtype: dns.TypeAAAA, Class: dns.ClassINET, Ttl: 60},
			AAAA: net.ParseIP("2001:db8::1"),
		})
		_ = w.WriteMsg(m)
	})

	queries := []queryConfig{
		{name: "ipv6.example.com", qtype: dns.TypeAAAA, expect: "2001:db8::1", resultKey: "ipv6.example.com", dsName: "q0"},
	}
	chk, err := New(addr, queries)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result := chk.Run(context.Background())
	if !result.Success {
		t.Errorf("expected success, got failure: %v", result.Err)
	}
}

func TestRun_MultipleQueries_AllSuccess(t *testing.T) {
	addr := startTestServer(t, func(w dns.ResponseWriter, r *dns.Msg) {
		m := new(dns.Msg)
		m.SetReply(r)
		switch r.Question[0].Qtype {
		case dns.TypeA:
			m.Answer = append(m.Answer, &dns.A{
				Hdr: dns.RR_Header{Name: r.Question[0].Name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60},
				A:   net.ParseIP("192.168.168.1"),
			})
		case dns.TypePTR:
			m.Answer = append(m.Answer, &dns.PTR{
				Hdr: dns.RR_Header{Name: r.Question[0].Name, Rrtype: dns.TypePTR, Class: dns.ClassINET, Ttl: 60},
				Ptr: "router.example.com.",
			})
		}
		_ = w.WriteMsg(m)
	})

	queries := []queryConfig{
		{name: "router.example.com", qtype: dns.TypeA, expect: "192.168.168.1", resultKey: "router.example.com", dsName: "q0"},
		{name: "1.168.168.192.in-addr.arpa", qtype: dns.TypePTR, expect: "router.example.com.", resultKey: "1.168.168.192.in-addr.arpa", dsName: "q1"},
	}
	chk, err := New(addr, queries)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result := chk.Run(context.Background())
	if !result.Success {
		t.Errorf("expected success, got failure: %v", result.Err)
	}
	if len(result.Metrics) != 2 {
		t.Errorf("expected 2 metrics, got %d", len(result.Metrics))
	}
}

func TestRun_WrongAnswer_Failure(t *testing.T) {
	addr := startTestServer(t, func(w dns.ResponseWriter, r *dns.Msg) {
		m := new(dns.Msg)
		m.SetReply(r)
		m.Answer = append(m.Answer, &dns.A{
			Hdr: dns.RR_Header{Name: r.Question[0].Name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60},
			A:   net.ParseIP("10.0.0.1"),
		})
		_ = w.WriteMsg(m)
	})

	queries := []queryConfig{
		{name: "router.example.com", qtype: dns.TypeA, expect: "192.168.168.1", resultKey: "router.example.com", dsName: "q0"},
	}
	chk, err := New(addr, queries)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result := chk.Run(context.Background())
	if result.Success {
		t.Error("expected failure for wrong answer")
	}
	if result.Err == nil {
		t.Error("expected non-nil error")
	}
}

func TestRun_NXDomain_Failure(t *testing.T) {
	addr := startTestServer(t, func(w dns.ResponseWriter, r *dns.Msg) {
		m := new(dns.Msg)
		m.SetReply(r)
		m.Rcode = dns.RcodeNameError // NXDOMAIN
		_ = w.WriteMsg(m)
	})

	queries := []queryConfig{
		{name: "nonexistent.example.com", qtype: dns.TypeA, expect: "1.2.3.4", resultKey: "nonexistent.example.com", dsName: "q0"},
	}
	chk, err := New(addr, queries)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result := chk.Run(context.Background())
	if result.Success {
		t.Error("expected failure for NXDOMAIN")
	}
}

func TestRun_PartialSuccess_Failure(t *testing.T) {
	call := 0
	addr := startTestServer(t, func(w dns.ResponseWriter, r *dns.Msg) {
		m := new(dns.Msg)
		m.SetReply(r)
		call++
		if call == 1 {
			m.Answer = append(m.Answer, &dns.A{
				Hdr: dns.RR_Header{Name: r.Question[0].Name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60},
				A:   net.ParseIP("192.168.168.1"),
			})
		} else {
			m.Rcode = dns.RcodeNameError
		}
		_ = w.WriteMsg(m)
	})

	queries := []queryConfig{
		{name: "router.example.com", qtype: dns.TypeA, expect: "192.168.168.1", resultKey: "router.example.com", dsName: "q0"},
		{name: "missing.example.com", qtype: dns.TypeA, expect: "5.6.7.8", resultKey: "missing.example.com", dsName: "q1"},
	}
	chk, err := New(addr, queries)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result := chk.Run(context.Background())
	if result.Success {
		t.Error("expected failure when one query fails")
	}
	if len(result.Metrics) != 2 {
		t.Errorf("expected 2 metrics (one nil, one non-nil), got %d", len(result.Metrics))
	}
	if result.Metrics["router.example.com"] == nil {
		t.Error("expected non-nil metric for successful query")
	}
	if result.Metrics["missing.example.com"] != nil {
		t.Error("expected nil metric for failed query")
	}
}

func TestRun_ContextCancelled(t *testing.T) {
	addr := startTestServer(t, func(w dns.ResponseWriter, r *dns.Msg) {
		m := new(dns.Msg)
		m.SetReply(r)
		m.Answer = append(m.Answer, &dns.A{
			Hdr: dns.RR_Header{Name: r.Question[0].Name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60},
			A:   net.ParseIP("1.2.3.4"),
		})
		_ = w.WriteMsg(m)
	})

	queries := []queryConfig{
		{name: "example.com", qtype: dns.TypeA, expect: "1.2.3.4", resultKey: "example.com", dsName: "q0"},
	}
	chk, err := New(addr, queries)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result := chk.Run(ctx)
	if result.Success {
		t.Error("expected failure with cancelled context")
	}
}

// --- Registry integration ---

func TestRegistryIntegration(t *testing.T) {
	reg := check.NewRegistry()
	if err := reg.Register(TypeName, Factory); err != nil {
		t.Fatalf("failed to register dns: %v", err)
	}
	chk, err := reg.Create("dns", map[string]any{
		"server": "127.0.0.1:53",
		"queries": []any{
			map[string]any{"name": "example.com", "type": "A", "expect": "93.184.216.34"},
		},
	})
	if err != nil {
		t.Fatalf("failed to create dns check: %v", err)
	}
	if chk.Type() != TypeName {
		t.Errorf("expected type %q, got %q", TypeName, chk.Type())
	}
}

func TestCheckInterface(t *testing.T) {
	var _ check.Check = &Check{}
}
