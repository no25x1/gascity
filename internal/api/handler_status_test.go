package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gastownhall/gascity/internal/runtime"
)

func TestHandleStatus(t *testing.T) {
	state := newFakeState(t)
	// Start a fake session so Running > 0.
	state.sp.Start(context.Background(), "myrig--worker", runtime.Config{}) //nolint:errcheck
	srv := New(state)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	writeWSJSON(t, conn, wsRequestEnvelope{
		Type:   "request",
		ID:     "status-1",
		Action: "status.get",
	})
	var resp wsResponseEnvelope
	readWSJSON(t, conn, &resp)

	var sr statusResponse
	if err := json.Unmarshal(resp.Result, &sr); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if sr.Name != "test-city" {
		t.Errorf("Name = %q, want %q", sr.Name, "test-city")
	}
	if sr.AgentCount != 1 {
		t.Errorf("AgentCount = %d, want 1", sr.AgentCount)
	}
	if sr.RigCount != 1 {
		t.Errorf("RigCount = %d, want 1", sr.RigCount)
	}
	if sr.Running != 1 {
		t.Errorf("Running = %d, want 1", sr.Running)
	}
}

func TestHandleStatusEnriched(t *testing.T) {
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
		ID:     "status-e",
		Action: "status.get",
	})
	var resp wsResponseEnvelope
	readWSJSON(t, conn, &resp)

	var sr statusResponse
	json.Unmarshal(resp.Result, &sr) //nolint:errcheck

	// Version from fakeState.
	if sr.Version != "test" {
		t.Errorf("Version = %q, want %q", sr.Version, "test")
	}

	// Uptime should be >= 0.
	if sr.UptimeSec < 0 {
		t.Errorf("UptimeSec = %d, want >= 0", sr.UptimeSec)
	}

	// Agent counts.
	if sr.Agents.Total != 1 {
		t.Errorf("Agents.Total = %d, want 1", sr.Agents.Total)
	}
	if sr.Agents.Running != 1 {
		t.Errorf("Agents.Running = %d, want 1", sr.Agents.Running)
	}

	// Rig counts.
	if sr.Rigs.Total != 1 {
		t.Errorf("Rigs.Total = %d, want 1", sr.Rigs.Total)
	}
}

func TestHandleHealth(t *testing.T) {
	state := newFakeState(t)
	srv := New(state)

	// /health stays on HTTP — it is not a /v0/* route.
	req := httptest.NewRequest("GET", "/health", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp) //nolint:errcheck

	if resp["status"] != "ok" {
		t.Errorf("status = %v, want %q", resp["status"], "ok")
	}
	if resp["version"] != "test" {
		t.Errorf("version = %v, want %q", resp["version"], "test")
	}
	if resp["city"] != "test-city" {
		t.Errorf("city = %v, want %q", resp["city"], "test-city")
	}
	if _, ok := resp["uptime_sec"]; !ok {
		t.Error("missing uptime_sec in health response")
	}
}

func TestHandleStatus_Suspended(t *testing.T) {
	state := newFakeState(t)
	state.cfg.Workspace.Suspended = true
	srv := New(state)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	writeWSJSON(t, conn, wsRequestEnvelope{
		Type:   "request",
		ID:     "status-susp",
		Action: "status.get",
	})
	var resp wsResponseEnvelope
	readWSJSON(t, conn, &resp)

	var sr statusResponse
	if err := json.Unmarshal(resp.Result, &sr); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !sr.Suspended {
		t.Error("expected suspended=true in status response")
	}
}
