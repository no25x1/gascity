package api

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
)

func TestHandleRigCreate(t *testing.T) {
	fs := newFakeMutatorState(t)
	srv := New(fs)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()
	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	writeWSJSON(t, conn, wsRequestEnvelope{Type: "request", ID: "rig-create", Action: "rig.create", Payload: map[string]any{"name": "new-rig", "path": "/tmp/new-rig"}})
	var resp wsResponseEnvelope
	readWSJSON(t, conn, &resp)
	if resp.Type != "response" {
		t.Fatalf("type = %q, want response; result = %s", resp.Type, resp.Result)
	}

	found := false
	for _, r := range fs.cfg.Rigs {
		if r.Name == "new-rig" && r.Path == "/tmp/new-rig" {
			found = true
		}
	}
	if !found {
		t.Error("rig 'new-rig' not found in config after create")
	}
}

func TestHandleRigCreate_MissingName(t *testing.T) {
	fs := newFakeMutatorState(t)
	srv := New(fs)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()
	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	writeWSJSON(t, conn, wsRequestEnvelope{Type: "request", ID: "rig-no-name", Action: "rig.create", Payload: map[string]any{"path": "/tmp/x"}})
	var errResp wsErrorEnvelope
	readWSJSON(t, conn, &errResp)
	if errResp.Type != "error" || errResp.Code != "invalid" {
		t.Fatalf("expected invalid error, got %#v", errResp)
	}
}

func TestHandleRigUpdate(t *testing.T) {
	fs := newFakeMutatorState(t)
	srv := New(fs)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()
	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	writeWSJSON(t, conn, wsRequestEnvelope{Type: "request", ID: "rig-update", Action: "rig.update", Payload: map[string]any{"name": "myrig", "path": "/tmp/updated"}})
	var resp wsResponseEnvelope
	readWSJSON(t, conn, &resp)
	if resp.Type != "response" {
		t.Fatalf("type = %q, want response", resp.Type)
	}
	var result map[string]string
	json.Unmarshal(resp.Result, &result)
	if result["status"] != "updated" {
		t.Fatalf("status = %q, want updated", result["status"])
	}
}

func TestHandleRigUpdate_NotFound(t *testing.T) {
	fs := newFakeMutatorState(t)
	srv := New(fs)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()
	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	writeWSJSON(t, conn, wsRequestEnvelope{Type: "request", ID: "rig-nf", Action: "rig.update", Payload: map[string]any{"name": "nonexistent", "path": "/tmp/x"}})
	var errResp wsErrorEnvelope
	readWSJSON(t, conn, &errResp)
	if errResp.Type != "error" {
		t.Fatalf("type = %q, want error", errResp.Type)
	}
}

func TestHandleRigDelete(t *testing.T) {
	fs := newFakeMutatorState(t)
	srv := New(fs)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()
	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	writeWSJSON(t, conn, wsRequestEnvelope{Type: "request", ID: "rig-delete", Action: "rig.delete", Payload: map[string]any{"name": "myrig"}})
	var resp wsResponseEnvelope
	readWSJSON(t, conn, &resp)
	if resp.Type != "response" {
		t.Fatalf("type = %q, want response", resp.Type)
	}

	for _, r := range fs.cfg.Rigs {
		if r.Name == "myrig" {
			t.Error("rig 'myrig' still exists after delete")
		}
	}
}

func TestHandleRigDelete_NotFound(t *testing.T) {
	fs := newFakeMutatorState(t)
	srv := New(fs)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()
	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	writeWSJSON(t, conn, wsRequestEnvelope{Type: "request", ID: "rig-del-nf", Action: "rig.delete", Payload: map[string]any{"name": "nonexistent"}})
	var errResp wsErrorEnvelope
	readWSJSON(t, conn, &errResp)
	if errResp.Type != "error" {
		t.Fatalf("type = %q, want error", errResp.Type)
	}
}
