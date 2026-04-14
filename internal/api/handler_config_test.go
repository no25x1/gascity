package api

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gastownhall/gascity/internal/config"
)

func TestHandleConfigGet(t *testing.T) {
	fs := newFakeState(t)
	fs.cfg.Workspace.Provider = "claude"
	fs.cfg.Agents[0].MinActiveSessions = intPtr(0)
	fs.cfg.Agents[0].MaxActiveSessions = intPtr(3)
	fs.cfg.Providers = map[string]config.ProviderSpec{
		"custom": {DisplayName: "Custom", Command: "custom-cli"},
	}
	srv := New(fs)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	writeWSJSON(t, conn, wsRequestEnvelope{
		Type:   "request",
		ID:     "config-1",
		Action: "config.get",
	})
	var resp wsResponseEnvelope
	readWSJSON(t, conn, &resp)

	var cr configResponse
	json.Unmarshal(resp.Result, &cr) //nolint:errcheck

	if cr.Workspace.Name != "test-city" {
		t.Errorf("workspace.name = %q, want %q", cr.Workspace.Name, "test-city")
	}
	if cr.Workspace.Provider != "claude" {
		t.Errorf("workspace.provider = %q, want %q", cr.Workspace.Provider, "claude")
	}
	if len(cr.Agents) != 1 {
		t.Errorf("agents count = %d, want 1", len(cr.Agents))
	}
	if !cr.Agents[0].IsPool {
		t.Error("expected config agent to expose is_pool=true")
	}
	if len(cr.Rigs) != 1 {
		t.Errorf("rigs count = %d, want 1", len(cr.Rigs))
	}
	if _, ok := cr.Providers["custom"]; !ok {
		t.Error("expected 'custom' in providers")
	}
}

func TestHandleConfigGet_NoPatches(t *testing.T) {
	fs := newFakeState(t)
	srv := New(fs)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	writeWSJSON(t, conn, wsRequestEnvelope{
		Type:   "request",
		ID:     "config-np",
		Action: "config.get",
	})
	var resp wsResponseEnvelope
	readWSJSON(t, conn, &resp)

	// Patches should be omitted when empty.
	var raw map[string]any
	json.Unmarshal(resp.Result, &raw) //nolint:errcheck
	if _, ok := raw["patches"]; ok {
		t.Error("expected patches to be omitted when empty")
	}
}

func TestHandleConfigGet_WithPatches(t *testing.T) {
	fs := newFakeState(t)
	fs.cfg.Patches.Agents = []config.AgentPatch{
		{Dir: "rig1", Name: "worker"},
	}
	srv := New(fs)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	writeWSJSON(t, conn, wsRequestEnvelope{
		Type:   "request",
		ID:     "config-wp",
		Action: "config.get",
	})
	var resp wsResponseEnvelope
	readWSJSON(t, conn, &resp)

	var cr configResponse
	json.Unmarshal(resp.Result, &cr) //nolint:errcheck
	if cr.Patches == nil {
		t.Fatal("expected patches to be present")
	}
	if cr.Patches.AgentCount != 1 {
		t.Errorf("patches.agent_count = %d, want 1", cr.Patches.AgentCount)
	}
}

func TestHandleConfigExplain(t *testing.T) {
	fs := newFakeState(t)
	fs.cfg.Agents[0].MinActiveSessions = intPtr(0)
	fs.cfg.Agents[0].MaxActiveSessions = intPtr(3)
	fs.cfg.Providers = map[string]config.ProviderSpec{
		"claude": {DisplayName: "My Claude", Command: "my-claude"},
	}
	srv := New(fs)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	writeWSJSON(t, conn, wsRequestEnvelope{
		Type:   "request",
		ID:     "explain-1",
		Action: "config.explain",
	})
	var resp wsResponseEnvelope
	readWSJSON(t, conn, &resp)

	var result map[string]any
	json.Unmarshal(resp.Result, &result) //nolint:errcheck

	// Check agents have origin annotations.
	agents, ok := result["agents"].([]any)
	if !ok {
		t.Fatal("expected agents array")
	}
	if len(agents) == 0 {
		t.Fatal("expected at least one agent")
	}
	agent0 := agents[0].(map[string]any)
	if agent0["origin"] != "inline" {
		t.Errorf("agent origin = %q, want %q", agent0["origin"], "inline")
	}
	if agent0["is_pool"] != true {
		t.Errorf("agent is_pool = %#v, want true", agent0["is_pool"])
	}

	// Check providers have origin annotations.
	providers, ok := result["providers"].(map[string]any)
	if !ok {
		t.Fatal("expected providers map")
	}
	claude := providers["claude"].(map[string]any)
	if claude["origin"] != "builtin+city" {
		t.Errorf("claude origin = %q, want %q", claude["origin"], "builtin+city")
	}
	// A builtin-only provider should have origin "builtin".
	codex := providers["codex"].(map[string]any)
	if codex["origin"] != "builtin" {
		t.Errorf("codex origin = %q, want %q", codex["origin"], "builtin")
	}
}

func TestHandleConfigValidate_Valid(t *testing.T) {
	fs := newFakeState(t)
	srv := New(fs)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	writeWSJSON(t, conn, wsRequestEnvelope{
		Type:   "request",
		ID:     "validate-1",
		Action: "config.validate",
	})
	var resp wsResponseEnvelope
	readWSJSON(t, conn, &resp)

	var result map[string]any
	json.Unmarshal(resp.Result, &result) //nolint:errcheck
	if result["valid"] != true {
		t.Error("expected valid=true for well-formed config")
	}
}

func TestHandleConfigValidate_WithWarnings(t *testing.T) {
	fs := newFakeState(t)
	// Agent references a nonexistent provider — should produce a warning.
	fs.cfg.Agents[0].Provider = "nonexistent-provider"
	srv := New(fs)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	writeWSJSON(t, conn, wsRequestEnvelope{
		Type:   "request",
		ID:     "validate-warn",
		Action: "config.validate",
	})
	var resp wsResponseEnvelope
	readWSJSON(t, conn, &resp)

	var result map[string]any
	json.Unmarshal(resp.Result, &result) //nolint:errcheck

	// Config is still valid (warnings are non-fatal).
	if result["valid"] != true {
		t.Error("expected valid=true (warnings are non-fatal)")
	}

	warnings, ok := result["warnings"].([]any)
	if !ok || len(warnings) == 0 {
		t.Error("expected at least one warning for unknown provider")
	}
}

func TestHandleConfigValidate_InvalidServiceRuntimeSupport(t *testing.T) {
	fs := newFakeState(t)
	fs.cfg.Services = []config.Service{{
		Name:     "review-intake",
		Workflow: config.ServiceWorkflowConfig{Contract: "missing.contract"},
	}}
	srv := New(fs)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	writeWSJSON(t, conn, wsRequestEnvelope{
		Type:   "request",
		ID:     "validate-svc",
		Action: "config.validate",
	})
	var resp wsResponseEnvelope
	readWSJSON(t, conn, &resp)

	var result struct {
		Valid  bool     `json:"valid"`
		Errors []string `json:"errors"`
	}
	json.Unmarshal(resp.Result, &result) //nolint:errcheck
	if result.Valid {
		t.Fatal("expected valid=false for unsupported service runtime")
	}
	if len(result.Errors) == 0 || !strings.Contains(result.Errors[0], `unsupported workflow contract`) {
		t.Fatalf("errors = %#v, want unsupported workflow contract", result.Errors)
	}
}

func TestHandleConfigExplain_PackDerivedAgent(t *testing.T) {
	fs := newFakeState(t)
	// Simulate pack-derived agent: present in expanded config (cfg) but
	// absent from raw config. The explain handler uses RawConfigProvider
	// for accurate provenance detection.
	fs.rawCfg = &config.City{
		Workspace: config.Workspace{Name: "test-city"},
		// No agents in raw — worker comes from pack expansion.
		Rigs: []config.Rig{
			{Name: "myrig", Path: "/tmp/myrig"},
		},
	}
	srv := New(fs)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	writeWSJSON(t, conn, wsRequestEnvelope{
		Type:   "request",
		ID:     "explain-pack",
		Action: "config.explain",
	})
	var resp wsResponseEnvelope
	readWSJSON(t, conn, &resp)

	var result map[string]any
	json.Unmarshal(resp.Result, &result) //nolint:errcheck
	agents := result["agents"].([]any)
	agent0 := agents[0].(map[string]any)
	if agent0["origin"] != "pack-derived" {
		t.Errorf("agent origin = %q, want %q", agent0["origin"], "pack-derived")
	}
}
