package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gastownhall/gascity/internal/config"
	"github.com/gastownhall/gascity/internal/runtime"
	"github.com/gastownhall/gascity/internal/sessionlog"
)

// WS test types and helpers are in websocket_test.go.
// This file uses: wsRequestEnvelope, wsResponseEnvelope, wsErrorEnvelope,
// dialWebSocket, drainWSHello, writeWSJSON, readWSJSON.

// --- Tests ---

func TestAgentList(t *testing.T) {
	state := newFakeState(t)
	state.sp.Start(context.Background(), "myrig--worker", runtime.Config{}) //nolint:errcheck
	srv := New(state)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	writeWSJSON(t, conn, wsRequestEnvelope{
		Type:   "request",
		ID:     "test-1",
		Action: "agents.list",
	})

	var resp wsResponseEnvelope
	readWSJSON(t, conn, &resp)
	if resp.Type != "response" {
		t.Fatalf("type = %q, want response", resp.Type)
	}
	if resp.ID != "test-1" {
		t.Fatalf("id = %q, want test-1", resp.ID)
	}

	var body listResponse
	if err := json.Unmarshal(resp.Result, &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Total != 1 {
		t.Errorf("Total = %d, want 1", body.Total)
	}
}

func TestAgentListPoolExpansion(t *testing.T) {
	state := newFakeState(t)
	state.cfg.Agents = []config.Agent{
		{
			Name:              "polecat",
			Dir:               "myrig",
			MinActiveSessions: intPtr(1), MaxActiveSessions: intPtr(3), ScaleCheck: "echo 3",
		},
	}
	srv := New(state)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	writeWSJSON(t, conn, wsRequestEnvelope{
		Type:   "request",
		ID:     "test-pool",
		Action: "agents.list",
	})

	var resp wsResponseEnvelope
	readWSJSON(t, conn, &resp)

	var body struct {
		Items []agentResponse `json:"items"`
		Total int             `json:"total"`
	}
	if err := json.Unmarshal(resp.Result, &body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if body.Total != 3 {
		t.Fatalf("Total = %d, want 3", body.Total)
	}

	// Check pool member names.
	want := []string{"myrig/polecat-1", "myrig/polecat-2", "myrig/polecat-3"}
	for i, name := range want {
		if body.Items[i].Name != name {
			t.Errorf("Items[%d].Name = %q, want %q", i, body.Items[i].Name, name)
		}
		if body.Items[i].Pool != "myrig/polecat" {
			t.Errorf("Items[%d].Pool = %q, want %q", i, body.Items[i].Pool, "myrig/polecat")
		}
	}
}

func TestAgentListUnlimitedPoolDiscovery(t *testing.T) {
	state := newFakeState(t)
	state.cfg.Agents = []config.Agent{
		{
			Name:              "polecat",
			Dir:               "myrig",
			MinActiveSessions: intPtr(0), MaxActiveSessions: intPtr(-1),
		},
	}
	// Start 2 running sessions matching the pool pattern.
	state.sp.Start(context.Background(), "myrig--polecat-1", runtime.Config{}) //nolint:errcheck
	state.sp.Start(context.Background(), "myrig--polecat-2", runtime.Config{}) //nolint:errcheck
	srv := New(state)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	writeWSJSON(t, conn, wsRequestEnvelope{
		Type:   "request",
		ID:     "test-unlimited",
		Action: "agents.list",
	})

	var resp wsResponseEnvelope
	readWSJSON(t, conn, &resp)

	var body struct {
		Items []agentResponse `json:"items"`
		Total int             `json:"total"`
	}
	if err := json.Unmarshal(resp.Result, &body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if body.Total != 2 {
		t.Fatalf("Total = %d, want 2", body.Total)
	}

	// Both discovered instances should reference the pool.
	for i, item := range body.Items {
		if item.Pool != "myrig/polecat" {
			t.Errorf("Items[%d].Pool = %q, want %q", i, item.Pool, "myrig/polecat")
		}
		if !item.Running {
			t.Errorf("Items[%d].Running = false, want true", i)
		}
	}
}

func TestFindAgentUnlimitedPoolMember(t *testing.T) {
	cfg := &config.City{
		Agents: []config.Agent{
			{
				Name:              "polecat",
				Dir:               "myrig",
				MinActiveSessions: intPtr(0), MaxActiveSessions: intPtr(-1),
			},
		},
	}
	// Unlimited pool members follow the pattern {name}-{N}.
	a, ok := findAgent(cfg, "myrig/polecat-5")
	if !ok {
		t.Fatal("findAgent(myrig/polecat-5) = false, want true for unlimited pool")
	}
	if a.Name != "polecat" {
		t.Errorf("agent.Name = %q, want %q", a.Name, "polecat")
	}

	// Non-numeric suffix should not match.
	_, ok = findAgent(cfg, "myrig/polecat-abc")
	if ok {
		t.Error("findAgent(myrig/polecat-abc) = true, want false for non-numeric suffix")
	}
}

func TestAgentListFilterByRig(t *testing.T) {
	state := newFakeState(t)
	state.cfg.Agents = []config.Agent{
		{Name: "worker", Dir: "rig1", MaxActiveSessions: intPtr(1)},
		{Name: "worker", Dir: "rig2", MaxActiveSessions: intPtr(1)},
	}
	state.cfg.Rigs = []config.Rig{
		{Name: "rig1", Path: filepath.Join(state.cityPath, "repos", "rig1")},
		{Name: "rig2", Path: filepath.Join(state.cityPath, "repos", "rig2")},
	}
	srv := New(state)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	writeWSJSON(t, conn, wsRequestEnvelope{
		Type:   "request",
		ID:     "test-rig-filter",
		Action: "agents.list",
		Payload: map[string]interface{}{
			"rig": "rig1",
		},
	})

	var resp wsResponseEnvelope
	readWSJSON(t, conn, &resp)

	var body struct {
		Items []agentResponse `json:"items"`
		Total int             `json:"total"`
	}
	if err := json.Unmarshal(resp.Result, &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Total != 1 {
		t.Errorf("Total = %d, want 1", body.Total)
	}
	if body.Items[0].Name != "rig1/worker" {
		t.Errorf("Name = %q, want %q", body.Items[0].Name, "rig1/worker")
	}
}

func TestAgentListFilterByRunning(t *testing.T) {
	state := newFakeState(t)
	state.cfg.Agents = []config.Agent{
		{Name: "running-agent", MaxActiveSessions: intPtr(1)},
		{Name: "stopped-agent", MaxActiveSessions: intPtr(1)},
	}
	state.sp.Start(context.Background(), "running-agent", runtime.Config{}) //nolint:errcheck
	srv := New(state)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	writeWSJSON(t, conn, wsRequestEnvelope{
		Type:   "request",
		ID:     "test-running-filter",
		Action: "agents.list",
		Payload: map[string]interface{}{
			"running": "true",
		},
	})

	var resp wsResponseEnvelope
	readWSJSON(t, conn, &resp)

	var body struct {
		Items []agentResponse `json:"items"`
		Total int             `json:"total"`
	}
	if err := json.Unmarshal(resp.Result, &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Total != 1 {
		t.Errorf("Total = %d, want 1", body.Total)
	}
	if body.Items[0].Name != "running-agent" {
		t.Errorf("Name = %q, want %q", body.Items[0].Name, "running-agent")
	}
}

func TestAgentGet(t *testing.T) {
	state := newFakeState(t)
	srv := New(state)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	writeWSJSON(t, conn, wsRequestEnvelope{
		Type:   "request",
		ID:     "test-get",
		Action: "agent.get",
		Payload: map[string]interface{}{
			"name": "myrig/worker",
		},
	})

	var resp wsResponseEnvelope
	readWSJSON(t, conn, &resp)

	var body agentResponse
	if err := json.Unmarshal(resp.Result, &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Name != "myrig/worker" {
		t.Errorf("Name = %q, want %q", body.Name, "myrig/worker")
	}
}

func TestAgentGetNotFound(t *testing.T) {
	state := newFakeState(t)
	srv := New(state)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	writeWSJSON(t, conn, wsRequestEnvelope{
		Type:   "request",
		ID:     "test-get-404",
		Action: "agent.get",
		Payload: map[string]interface{}{
			"name": "nonexistent",
		},
	})

	var resp wsErrorEnvelope
	readWSJSON(t, conn, &resp)
	if resp.Type != "error" {
		t.Errorf("type = %q, want error", resp.Type)
	}
	if resp.Code != "not_found" {
		t.Errorf("code = %q, want not_found", resp.Code)
	}
}

func TestAgentOutputPeekFallback(t *testing.T) {
	t.Skip("agent output peek requires session-based resolution; skipping until session.get with peek is wired")
}

func TestFindAgentPoolMaxZero(t *testing.T) {
	// Regression: pool with Max=0 should default to 1, matching expandAgent.
	cfg := &config.City{
		Agents: []config.Agent{
			{
				Name:              "polecat",
				Dir:               "myrig",
				MinActiveSessions: intPtr(0), MaxActiveSessions: intPtr(0), ScaleCheck: "echo 0",
			},
		},
	}
	// Max=0 defaults to 1 member, so "polecat" (no suffix) should be found.
	a, ok := findAgent(cfg, "myrig/polecat")
	if !ok {
		t.Fatal("findAgent(myrig/polecat) = false, want true for pool with Max=0")
	}
	if a.Name != "polecat" {
		t.Errorf("agent.Name = %q, want %q", a.Name, "polecat")
	}
}

func TestAgentOutputNotRunning(t *testing.T) {
	state := newFakeState(t)
	srv := New(state)

	// Agent output is a sub-resource route that stays HTTP.
	req := httptest.NewRequest("GET", "/v0/agent/myrig/worker/output", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestAgentSuspendResume(t *testing.T) {
	state := newFakeMutatorState(t)
	srv := New(state)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	// Suspend.
	writeWSJSON(t, conn, wsRequestEnvelope{
		Type:   "request",
		ID:     "test-suspend",
		Action: "agent.suspend",
		Payload: map[string]interface{}{
			"name": "myrig/worker",
		},
	})

	var suspendResp wsResponseEnvelope
	readWSJSON(t, conn, &suspendResp)
	if suspendResp.Type != "response" {
		t.Fatalf("suspend: type = %q, want response", suspendResp.Type)
	}
	if !state.suspended["myrig/worker"] {
		t.Error("agent not suspended")
	}

	// Resume.
	writeWSJSON(t, conn, wsRequestEnvelope{
		Type:   "request",
		ID:     "test-resume",
		Action: "agent.resume",
		Payload: map[string]interface{}{
			"name": "myrig/worker",
		},
	})

	var resumeResp wsResponseEnvelope
	readWSJSON(t, conn, &resumeResp)
	if resumeResp.Type != "response" {
		t.Fatalf("resume: type = %q, want response", resumeResp.Type)
	}
	if state.suspended["myrig/worker"] {
		t.Error("agent still suspended after resume")
	}
}

func TestAgentActionNotFound(t *testing.T) {
	state := newFakeMutatorState(t)
	srv := New(state)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	writeWSJSON(t, conn, wsRequestEnvelope{
		Type:   "request",
		ID:     "test-action-404",
		Action: "agent.suspend",
		Payload: map[string]interface{}{
			"name": "nonexistent",
		},
	})

	var resp wsErrorEnvelope
	readWSJSON(t, conn, &resp)
	if resp.Code != "not_found" {
		t.Errorf("code = %q, want not_found", resp.Code)
	}
}

func TestAgentActionNotMutator(t *testing.T) {
	// fakeState (not fakeMutatorState) doesn't implement StateMutator.
	state := newFakeState(t)
	srv := New(state)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	writeWSJSON(t, conn, wsRequestEnvelope{
		Type:   "request",
		ID:     "test-no-mutator",
		Action: "agent.suspend",
		Payload: map[string]interface{}{
			"name": "myrig/worker",
		},
	})

	var resp wsErrorEnvelope
	readWSJSON(t, conn, &resp)
	if resp.Type != "error" {
		t.Errorf("type = %q, want error", resp.Type)
	}
	// Not-implemented maps to internal error code over WS.
	if resp.Code != "internal" {
		t.Errorf("code = %q, want internal", resp.Code)
	}
}

func TestAgentProviderAndDisplayName(t *testing.T) {
	state := newFakeState(t)
	state.cfg.Workspace.Provider = "claude"
	state.cfg.Agents = []config.Agent{
		{Name: "worker", Dir: "myrig", Provider: "claude", MaxActiveSessions: intPtr(1)},
		{Name: "coder", Dir: "myrig", MaxActiveSessions: intPtr(1)},
	}
	srv := New(state)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	writeWSJSON(t, conn, wsRequestEnvelope{
		Type:   "request",
		ID:     "test-provider",
		Action: "agents.list",
	})

	var resp wsResponseEnvelope
	readWSJSON(t, conn, &resp)

	var body struct {
		Items []agentResponse `json:"items"`
	}
	if err := json.Unmarshal(resp.Result, &body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if len(body.Items) < 2 {
		t.Fatalf("expected at least 2 agents, got %d", len(body.Items))
	}

	// First agent has explicit provider.
	if body.Items[0].Provider != "claude" {
		t.Errorf("Items[0].Provider = %q, want %q", body.Items[0].Provider, "claude")
	}
	if body.Items[0].DisplayName != "Claude Code" {
		t.Errorf("Items[0].DisplayName = %q, want %q", body.Items[0].DisplayName, "Claude Code")
	}

	// Second agent inherits workspace default.
	if body.Items[1].Provider != "claude" {
		t.Errorf("Items[1].Provider = %q, want %q", body.Items[1].Provider, "claude")
	}
}

func TestAgentStateEnum(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(*fakeState)
		wantState string
	}{
		{
			name:      "stopped",
			setup:     func(_ *fakeState) {},
			wantState: "stopped",
		},
		{
			name: "idle",
			setup: func(s *fakeState) {
				s.sp.Start(context.Background(), "myrig--worker", runtime.Config{}) //nolint:errcheck
			},
			wantState: "idle",
		},
		{
			name: "suspended",
			setup: func(s *fakeState) {
				s.cfg.Agents = []config.Agent{
					{Name: "worker", Dir: "myrig", Suspended: true, MaxActiveSessions: intPtr(1)},
				}
			},
			wantState: "suspended",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := newFakeState(t)
			tt.setup(state)
			srv := New(state)
			ts := httptest.NewServer(srv.handler())
			defer ts.Close()

			conn := dialWebSocket(t, ts.URL+"/v0/ws")
			defer conn.Close()
			drainWSHello(t, conn)

			writeWSJSON(t, conn, wsRequestEnvelope{
				Type:   "request",
				ID:     "test-state",
				Action: "agent.get",
				Payload: map[string]interface{}{
					"name": "myrig/worker",
				},
			})

			var resp wsResponseEnvelope
			readWSJSON(t, conn, &resp)

			var body agentResponse
			if err := json.Unmarshal(resp.Result, &body); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if body.State != tt.wantState {
				t.Errorf("State = %q, want %q", body.State, tt.wantState)
			}
		})
	}
}

func TestAgentPeekViaQueryParam(t *testing.T) {
	state := newFakeState(t)
	state.sp.Start(context.Background(), "myrig--worker", runtime.Config{}) //nolint:errcheck
	state.sp.SetPeekOutput("myrig--worker", "line1\nline2\nline3")
	srv := New(state)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	// Without peek — no last_output.
	writeWSJSON(t, conn, wsRequestEnvelope{
		Type:   "request",
		ID:     "test-no-peek",
		Action: "agents.list",
	})

	var resp1 wsResponseEnvelope
	readWSJSON(t, conn, &resp1)

	var body1 struct {
		Items []agentResponse `json:"items"`
	}
	if err := json.Unmarshal(resp1.Result, &body1); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body1.Items[0].LastOutput != "" {
		t.Error("expected empty last_output without peek")
	}

	// With peek=true — includes last_output.
	writeWSJSON(t, conn, wsRequestEnvelope{
		Type:   "request",
		ID:     "test-peek",
		Action: "agents.list",
		Payload: map[string]interface{}{
			"peek": true,
		},
	})

	var resp2 wsResponseEnvelope
	readWSJSON(t, conn, &resp2)

	var body2 struct {
		Items []agentResponse `json:"items"`
	}
	if err := json.Unmarshal(resp2.Result, &body2); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body2.Items[0].LastOutput == "" {
		t.Error("expected non-empty last_output with peek=true")
	}
}

func TestAgentModelAndContext(t *testing.T) {
	state := newFakeState(t)
	state.cfg.Workspace.Provider = "claude"
	state.cfg.Agents = []config.Agent{
		{Name: "worker", Dir: "myrig", Provider: "claude", MaxActiveSessions: intPtr(1)},
	}
	state.cfg.Rigs = []config.Rig{{Name: "myrig", Path: "/tmp/myrig"}}
	state.sp.Start(context.Background(), "myrig--worker", runtime.Config{}) //nolint:errcheck

	// Create a fake session JSONL file for the rig path.
	searchDir := t.TempDir()
	slug := sessionlog.ProjectSlug("/tmp/myrig")
	slugDir := filepath.Join(searchDir, slug)
	if err := os.MkdirAll(slugDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Write session JSONL with model + usage.
	sessionFile := filepath.Join(slugDir, "test-session.jsonl")
	lines := `{"type":"assistant","message":{"role":"assistant","model":"claude-opus-4-5-20251101","usage":{"input_tokens":10000,"cache_read_input_tokens":5000,"cache_creation_input_tokens":2000}}}` + "\n"
	if err := os.WriteFile(sessionFile, []byte(lines), 0o644); err != nil {
		t.Fatal(err)
	}

	srv := New(state)
	srv.sessionLogSearchPaths = []string{searchDir}
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	writeWSJSON(t, conn, wsRequestEnvelope{
		Type:   "request",
		ID:     "test-model",
		Action: "agent.get",
		Payload: map[string]interface{}{
			"name": "myrig/worker",
		},
	})

	var resp wsResponseEnvelope
	readWSJSON(t, conn, &resp)

	var body agentResponse
	if err := json.Unmarshal(resp.Result, &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Model != "claude-opus-4-5-20251101" {
		t.Errorf("Model = %q, want %q", body.Model, "claude-opus-4-5-20251101")
	}
	if body.ContextPct == nil {
		t.Error("expected non-nil ContextPct")
	} else if *body.ContextPct != 8 {
		t.Errorf("ContextPct = %d, want 8", *body.ContextPct)
	}
	if body.ContextWindow == nil {
		t.Error("expected non-nil ContextWindow")
	} else if *body.ContextWindow != 200000 {
		t.Errorf("ContextWindow = %d, want 200000", *body.ContextWindow)
	}
}

func TestAgentActivityFromSessionLog(t *testing.T) {
	state := newFakeState(t)
	state.cfg.Workspace.Provider = "claude"
	state.cfg.Agents = []config.Agent{
		{Name: "worker", Dir: "myrig", Provider: "claude", MaxActiveSessions: intPtr(1)},
	}
	state.cfg.Rigs = []config.Rig{{Name: "myrig", Path: "/tmp/myrig"}}
	state.sp.Start(context.Background(), "myrig--worker", runtime.Config{}) //nolint:errcheck

	searchDir := t.TempDir()
	slug := sessionlog.ProjectSlug("/tmp/myrig")
	slugDir := filepath.Join(searchDir, slug)
	if err := os.MkdirAll(slugDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Write session JSONL ending with tool_use stop_reason.
	sessionFile := filepath.Join(slugDir, "test-session.jsonl")
	lines := `{"type":"assistant","message":{"role":"assistant","model":"claude-opus-4-5-20251101","stop_reason":"tool_use","content":[{"type":"tool_use"}],"usage":{"input_tokens":10000}}}` + "\n"
	if err := os.WriteFile(sessionFile, []byte(lines), 0o644); err != nil {
		t.Fatal(err)
	}

	srv := New(state)
	srv.sessionLogSearchPaths = []string{searchDir}
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	writeWSJSON(t, conn, wsRequestEnvelope{
		Type:   "request",
		ID:     "test-activity",
		Action: "agent.get",
		Payload: map[string]interface{}{
			"name": "myrig/worker",
		},
	})

	var resp wsResponseEnvelope
	readWSJSON(t, conn, &resp)

	var body agentResponse
	if err := json.Unmarshal(resp.Result, &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Activity != "in-turn" {
		t.Errorf("Activity = %q, want %q", body.Activity, "in-turn")
	}
}

func TestResolveProviderInfo(t *testing.T) {
	cfg := &config.City{
		Workspace: config.Workspace{Provider: "claude"},
		Providers: map[string]config.ProviderSpec{
			"custom": {DisplayName: "My Custom Agent"},
		},
	}

	tests := []struct {
		agentProvider   string
		wantProvider    string
		wantDisplayName string
	}{
		{"claude", "claude", "Claude Code"},
		{"", "claude", "Claude Code"},           // falls back to workspace
		{"custom", "custom", "My Custom Agent"}, // city-level override
		{"unknown", "unknown", "Unknown"},       // title-cased fallback
	}

	for _, tt := range tests {
		t.Run(tt.agentProvider, func(t *testing.T) {
			provider, displayName := resolveProviderInfo(tt.agentProvider, cfg)
			if provider != tt.wantProvider {
				t.Errorf("provider = %q, want %q", provider, tt.wantProvider)
			}
			if displayName != tt.wantDisplayName {
				t.Errorf("displayName = %q, want %q", displayName, tt.wantDisplayName)
			}
		})
	}
}

func TestComputeAgentState(t *testing.T) {
	now := func() *time.Time { t := time.Now(); return &t }()
	old := func() *time.Time { t := time.Now().Add(-20 * time.Minute); return &t }()

	tests := []struct {
		name        string
		suspended   bool
		quarantined bool
		running     bool
		activeBead  string
		lastAct     *time.Time
		want        string
	}{
		{"suspended", true, false, true, "", nil, "suspended"},
		{"quarantined", false, true, false, "", nil, "quarantined"},
		{"stopped", false, false, false, "", nil, "stopped"},
		{"idle", false, false, true, "", nil, "idle"},
		{"working", false, false, true, "bead-1", now, "working"},
		{"waiting", false, false, true, "bead-1", old, "waiting"},
		{"working-no-activity", false, false, true, "bead-1", nil, "waiting"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := computeAgentState(tt.suspended, tt.quarantined, tt.running, tt.activeBead, tt.lastAct)
			if got != tt.want {
				t.Errorf("computeAgentState() = %q, want %q", got, tt.want)
			}
		})
	}
}
