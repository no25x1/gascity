package api

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
)

func TestHandleCityGet(t *testing.T) {
	fs := newFakeState(t)
	fs.cfg.Workspace.Provider = "claude"
	fs.cfg.Workspace.SessionTemplate = "{{.City}}--{{.Agent}}"
	srv := New(fs)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	writeWSJSON(t, conn, wsRequestEnvelope{
		Type:   "request",
		ID:     "city-get",
		Action: "city.get",
	})

	var resp wsResponseEnvelope
	readWSJSON(t, conn, &resp)
	if resp.Type != "response" || resp.ID != "city-get" {
		t.Fatalf("response = %#v, want correlated response", resp)
	}

	var city cityGetResponse
	if err := json.Unmarshal(resp.Result, &city); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if city.Name != "test-city" {
		t.Errorf("name = %q, want %q", city.Name, "test-city")
	}
	if city.Suspended {
		t.Error("expected suspended=false")
	}
	if city.Provider != "claude" {
		t.Errorf("provider = %q, want %q", city.Provider, "claude")
	}
	if city.SessionTemplate != "{{.City}}--{{.Agent}}" {
		t.Errorf("session_template = %q, want %q", city.SessionTemplate, "{{.City}}--{{.Agent}}")
	}
	if city.AgentCount != 1 {
		t.Errorf("agent_count = %d, want 1", city.AgentCount)
	}
	if city.RigCount != 1 {
		t.Errorf("rig_count = %d, want 1", city.RigCount)
	}
}

func TestHandleCityGet_Suspended(t *testing.T) {
	fs := newFakeState(t)
	fs.cfg.Workspace.Suspended = true
	srv := New(fs)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	writeWSJSON(t, conn, wsRequestEnvelope{
		Type:   "request",
		ID:     "city-suspended",
		Action: "city.get",
	})

	var resp wsResponseEnvelope
	readWSJSON(t, conn, &resp)
	if resp.Type != "response" {
		t.Fatalf("type = %q, want response", resp.Type)
	}

	var city cityGetResponse
	if err := json.Unmarshal(resp.Result, &city); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !city.Suspended {
		t.Error("expected suspended=true")
	}
}
