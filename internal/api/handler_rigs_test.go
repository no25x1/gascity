package api

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gastownhall/gascity/internal/config"
	"github.com/gastownhall/gascity/internal/runtime"
)

func TestRigList(t *testing.T) {
	state := newFakeState(t)
	srv := New(state)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	writeWSJSON(t, conn, wsRequestEnvelope{Type: "request", ID: "rig-list", Action: "rigs.list"})
	var resp wsResponseEnvelope
	readWSJSON(t, conn, &resp)
	if resp.Type != "response" {
		t.Fatalf("type = %q, want response", resp.Type)
	}
	var lr listResponse
	json.Unmarshal(resp.Result, &lr)
	if lr.Total != 1 {
		t.Fatalf("total = %d, want 1", lr.Total)
	}
}

func TestRigGet(t *testing.T) {
	state := newFakeState(t)
	srv := New(state)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	writeWSJSON(t, conn, wsRequestEnvelope{Type: "request", ID: "rig-get", Action: "rig.get", Payload: map[string]any{"name": "myrig"}})
	var resp wsResponseEnvelope
	readWSJSON(t, conn, &resp)
	if resp.Type != "response" {
		t.Fatalf("type = %q, want response", resp.Type)
	}
	var rig rigResponse
	json.Unmarshal(resp.Result, &rig)
	if rig.Name != "myrig" {
		t.Fatalf("name = %q, want %q", rig.Name, "myrig")
	}
}

func TestRigGetNotFound(t *testing.T) {
	state := newFakeState(t)
	srv := New(state)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	writeWSJSON(t, conn, wsRequestEnvelope{Type: "request", ID: "rig-nf", Action: "rig.get", Payload: map[string]any{"name": "nonexistent"}})
	var errResp wsErrorEnvelope
	readWSJSON(t, conn, &errResp)
	if errResp.Type != "error" {
		t.Fatalf("type = %q, want error", errResp.Type)
	}
	if errResp.Code != "not_found" {
		t.Fatalf("code = %q, want not_found", errResp.Code)
	}
}

func TestRigEnrichment(t *testing.T) {
	state := newFakeState(t)
	state.cfg.Agents = []config.Agent{
		{Name: "worker", Dir: "myrig", MaxActiveSessions: intPtr(1)},
		{Name: "coder", Dir: "myrig", MaxActiveSessions: intPtr(1)},
	}
	state.sp.Start(context.Background(), "myrig--worker", runtime.Config{}) //nolint:errcheck
	srv := New(state)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	writeWSJSON(t, conn, wsRequestEnvelope{Type: "request", ID: "rig-enrich", Action: "rig.get", Payload: map[string]any{"name": "myrig"}})
	var resp wsResponseEnvelope
	readWSJSON(t, conn, &resp)
	if resp.Type != "response" {
		t.Fatalf("type = %q, want response", resp.Type)
	}
	var rig rigResponse
	json.Unmarshal(resp.Result, &rig)
	if rig.AgentCount != 2 {
		t.Errorf("AgentCount = %d, want 2", rig.AgentCount)
	}
	if rig.RunningCount != 1 {
		t.Errorf("RunningCount = %d, want 1", rig.RunningCount)
	}
}

func TestRigSuspendResume(t *testing.T) {
	state := newFakeMutatorState(t)
	srv := New(state)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	// Suspend
	writeWSJSON(t, conn, wsRequestEnvelope{Type: "request", ID: "rig-suspend", Action: "rig.suspend", Payload: map[string]any{"name": "myrig"}})
	var resp wsResponseEnvelope
	readWSJSON(t, conn, &resp)
	if resp.Type != "response" {
		t.Fatalf("suspend: type = %q, want response", resp.Type)
	}

	// Verify suspended
	writeWSJSON(t, conn, wsRequestEnvelope{Type: "request", ID: "rig-check-1", Action: "rig.get", Payload: map[string]any{"name": "myrig"}})
	readWSJSON(t, conn, &resp)
	var rig rigResponse
	json.Unmarshal(resp.Result, &rig)
	if !rig.Suspended {
		t.Fatal("rig should be suspended")
	}

	// Resume
	writeWSJSON(t, conn, wsRequestEnvelope{Type: "request", ID: "rig-resume", Action: "rig.resume", Payload: map[string]any{"name": "myrig"}})
	readWSJSON(t, conn, &resp)
	if resp.Type != "response" {
		t.Fatalf("resume: type = %q, want response", resp.Type)
	}

	// Verify not suspended
	writeWSJSON(t, conn, wsRequestEnvelope{Type: "request", ID: "rig-check-2", Action: "rig.get", Payload: map[string]any{"name": "myrig"}})
	readWSJSON(t, conn, &resp)
	json.Unmarshal(resp.Result, &rig)
	if rig.Suspended {
		t.Fatal("rig should not be suspended")
	}
}

func TestRigActionNotFound(t *testing.T) {
	state := newFakeMutatorState(t)
	srv := New(state)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	writeWSJSON(t, conn, wsRequestEnvelope{Type: "request", ID: "rig-nf", Action: "rig.suspend", Payload: map[string]any{"name": "nonexistent"}})
	var errResp wsErrorEnvelope
	readWSJSON(t, conn, &errResp)
	if errResp.Type != "error" {
		t.Fatalf("type = %q, want error", errResp.Type)
	}
}

func TestRigActionUnknown(t *testing.T) {
	state := newFakeMutatorState(t)
	srv := New(state)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	writeWSJSON(t, conn, wsRequestEnvelope{Type: "request", ID: "rig-unknown", Action: "rig.reboot", Payload: map[string]any{"name": "myrig"}})
	var errResp wsErrorEnvelope
	readWSJSON(t, conn, &errResp)
	if errResp.Type != "error" {
		t.Fatalf("type = %q, want error", errResp.Type)
	}
	if errResp.Code != "not_found" {
		t.Fatalf("code = %q, want not_found", errResp.Code)
	}
}
