package api

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/websocket"
)

func TestHandleAgentCreate(t *testing.T) {
	fs := newFakeMutatorState(t)
	srv := New(fs)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	writeWSJSON(t, conn, wsRequestEnvelope{
		Type:   "request",
		ID:     "create-1",
		Action: "agent.create",
		Payload: map[string]any{
			"name":     "coder",
			"provider": "claude",
		},
	})

	var resp wsResponseEnvelope
	readWSJSON(t, conn, &resp)
	if resp.Type != "response" || resp.ID != "create-1" {
		t.Fatalf("response = %#v, want correlated response", resp)
	}

	// Verify agent was added.
	found := false
	for _, a := range fs.cfg.Agents {
		if a.Name == "coder" && a.Provider == "claude" {
			found = true
		}
	}
	if !found {
		t.Error("agent 'coder' not found in config after create")
	}
}

func TestHandleAgentCreate_MissingName(t *testing.T) {
	fs := newFakeMutatorState(t)
	srv := New(fs)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	writeWSJSON(t, conn, wsRequestEnvelope{
		Type:   "request",
		ID:     "create-bad",
		Action: "agent.create",
		Payload: map[string]any{
			"provider": "claude",
		},
	})

	var errResp wsErrorEnvelope
	readWSJSON(t, conn, &errResp)
	if errResp.Type != "error" {
		t.Fatalf("type = %q, want error", errResp.Type)
	}
	if errResp.Code != "invalid" {
		t.Fatalf("code = %q, want invalid", errResp.Code)
	}
}

func TestHandleAgentUpdate(t *testing.T) {
	fs := newFakeMutatorState(t)
	srv := New(fs)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	writeWSJSON(t, conn, wsRequestEnvelope{
		Type:   "request",
		ID:     "update-1",
		Action: "agent.update",
		Payload: map[string]any{
			"name":     "myrig/worker",
			"provider": "gemini",
		},
	})

	var resp wsResponseEnvelope
	readWSJSON(t, conn, &resp)
	if resp.Type != "response" || resp.ID != "update-1" {
		t.Fatalf("response = %#v, want correlated response", resp)
	}
	var result map[string]string
	json.Unmarshal(resp.Result, &result)
	if result["status"] != "updated" {
		t.Fatalf("status = %q, want updated", result["status"])
	}
}

func TestHandleAgentDelete(t *testing.T) {
	fs := newFakeMutatorState(t)
	srv := New(fs)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	writeWSJSON(t, conn, wsRequestEnvelope{
		Type:   "request",
		ID:     "delete-1",
		Action: "agent.delete",
		Payload: map[string]any{
			"name": "myrig/worker",
		},
	})

	var resp wsResponseEnvelope
	readWSJSON(t, conn, &resp)
	if resp.Type != "response" || resp.ID != "delete-1" {
		t.Fatalf("response = %#v, want correlated response", resp)
	}
	var result map[string]string
	json.Unmarshal(resp.Result, &result)
	if result["status"] != "deleted" {
		t.Fatalf("status = %q, want deleted", result["status"])
	}
}

func TestHandleCityPatch_Suspend(t *testing.T) {
	state := newFakeMutatorState(t)
	srv := New(state)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	writeWSJSON(t, conn, wsRequestEnvelope{
		Type:   "request",
		ID:     "patch-1",
		Action: "city.patch",
		Payload: map[string]any{
			"suspended": true,
		},
	})

	var resp wsResponseEnvelope
	readWSJSON(t, conn, &resp)
	if resp.Type != "response" || resp.ID != "patch-1" {
		t.Fatalf("response = %#v, want correlated response", resp)
	}
}

func TestHandleCityPatch_Resume(t *testing.T) {
	state := newFakeMutatorState(t)
	srv := New(state)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	writeWSJSON(t, conn, wsRequestEnvelope{
		Type:   "request",
		ID:     "patch-2",
		Action: "city.patch",
		Payload: map[string]any{
			"suspended": false,
		},
	})

	var resp wsResponseEnvelope
	readWSJSON(t, conn, &resp)
	if resp.Type != "response" || resp.ID != "patch-2" {
		t.Fatalf("response = %#v, want correlated response", resp)
	}
}

// Ensure websocket import is used.
var _ = websocket.DefaultDialer
