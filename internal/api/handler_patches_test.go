package api

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gastownhall/gascity/internal/config"
)

// --- Agent patch tests ---

func TestHandleAgentPatchList(t *testing.T) {
	fs := newFakeState(t)
	suspended := true
	fs.cfg.Patches.Agents = []config.AgentPatch{{Dir: "rig1", Name: "worker", Suspended: &suspended}}
	srv := New(fs)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()
	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	writeWSJSON(t, conn, wsRequestEnvelope{Type: "request", ID: "ap-list", Action: "patches.agents.list"})
	var resp wsResponseEnvelope
	readWSJSON(t, conn, &resp)
	if resp.Type != "response" {
		t.Fatalf("type = %q, want response", resp.Type)
	}
	var lr listResponse
	json.Unmarshal(resp.Result, &lr)
	if lr.Total != 1 {
		t.Errorf("total = %d, want 1", lr.Total)
	}
}

func TestHandleAgentPatchList_Empty(t *testing.T) {
	fs := newFakeState(t)
	srv := New(fs)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()
	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	writeWSJSON(t, conn, wsRequestEnvelope{Type: "request", ID: "ap-empty", Action: "patches.agents.list"})
	var resp wsResponseEnvelope
	readWSJSON(t, conn, &resp)
	var lr listResponse
	json.Unmarshal(resp.Result, &lr)
	if lr.Total != 0 {
		t.Errorf("total = %d, want 0", lr.Total)
	}
}

func TestHandleAgentPatchGet(t *testing.T) {
	fs := newFakeState(t)
	suspended := true
	fs.cfg.Patches.Agents = []config.AgentPatch{{Dir: "rig1", Name: "worker", Suspended: &suspended}}
	srv := New(fs)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()
	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	writeWSJSON(t, conn, wsRequestEnvelope{Type: "request", ID: "ap-get", Action: "patches.agent.get", Payload: map[string]any{"name": "rig1/worker"}})
	var resp wsResponseEnvelope
	readWSJSON(t, conn, &resp)
	if resp.Type != "response" {
		t.Fatalf("type = %q, want response", resp.Type)
	}
}

func TestHandleAgentPatchGet_NotFound(t *testing.T) {
	fs := newFakeState(t)
	srv := New(fs)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()
	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	writeWSJSON(t, conn, wsRequestEnvelope{Type: "request", ID: "ap-nf", Action: "patches.agent.get", Payload: map[string]any{"name": "rig1/nonexistent"}})
	var errResp wsErrorEnvelope
	readWSJSON(t, conn, &errResp)
	if errResp.Type != "error" || errResp.Code != "not_found" {
		t.Fatalf("expected not_found error, got %#v", errResp)
	}
}

func TestHandleAgentPatchSet(t *testing.T) {
	fs := newFakeMutatorState(t)
	srv := New(fs)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()
	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	suspended := true
	writeWSJSON(t, conn, wsRequestEnvelope{Type: "request", ID: "ap-set", Action: "patches.agents.set", Payload: config.AgentPatch{Dir: "rig1", Name: "worker", Suspended: &suspended}})
	var resp wsResponseEnvelope
	readWSJSON(t, conn, &resp)
	if resp.Type != "response" {
		t.Fatalf("type = %q, want response", resp.Type)
	}
	if len(fs.cfg.Patches.Agents) != 1 {
		t.Fatalf("count = %d, want 1", len(fs.cfg.Patches.Agents))
	}
}

func TestHandleAgentPatchSet_MissingName(t *testing.T) {
	fs := newFakeMutatorState(t)
	srv := New(fs)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()
	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	suspended := true
	writeWSJSON(t, conn, wsRequestEnvelope{Type: "request", ID: "ap-no-name", Action: "patches.agents.set", Payload: config.AgentPatch{Dir: "rig1", Suspended: &suspended}})
	var errResp wsErrorEnvelope
	readWSJSON(t, conn, &errResp)
	if errResp.Type != "error" || errResp.Code != "invalid" {
		t.Fatalf("expected invalid error, got %#v", errResp)
	}
}

func TestHandleAgentPatchDelete(t *testing.T) {
	fs := newFakeMutatorState(t)
	suspended := true
	fs.cfg.Patches.Agents = []config.AgentPatch{{Dir: "rig1", Name: "worker", Suspended: &suspended}}
	srv := New(fs)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()
	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	writeWSJSON(t, conn, wsRequestEnvelope{Type: "request", ID: "ap-del", Action: "patches.agent.delete", Payload: map[string]any{"name": "rig1/worker"}})
	var resp wsResponseEnvelope
	readWSJSON(t, conn, &resp)
	if resp.Type != "response" {
		t.Fatalf("type = %q, want response", resp.Type)
	}
	if len(fs.cfg.Patches.Agents) != 0 {
		t.Errorf("count = %d, want 0", len(fs.cfg.Patches.Agents))
	}
}

func TestHandleAgentPatchDelete_NotFound(t *testing.T) {
	fs := newFakeMutatorState(t)
	srv := New(fs)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()
	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	writeWSJSON(t, conn, wsRequestEnvelope{Type: "request", ID: "ap-del-nf", Action: "patches.agent.delete", Payload: map[string]any{"name": "nonexistent"}})
	var errResp wsErrorEnvelope
	readWSJSON(t, conn, &errResp)
	if errResp.Type != "error" {
		t.Fatalf("type = %q, want error", errResp.Type)
	}
}

// --- Rig patch tests ---

func TestHandleRigPatchList(t *testing.T) {
	fs := newFakeState(t)
	suspended := true
	fs.cfg.Patches.Rigs = []config.RigPatch{{Name: "myrig", Suspended: &suspended}}
	srv := New(fs)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()
	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	writeWSJSON(t, conn, wsRequestEnvelope{Type: "request", ID: "rp-list", Action: "patches.rigs.list"})
	var resp wsResponseEnvelope
	readWSJSON(t, conn, &resp)
	var lr listResponse
	json.Unmarshal(resp.Result, &lr)
	if lr.Total != 1 {
		t.Errorf("total = %d, want 1", lr.Total)
	}
}

func TestHandleRigPatchSet(t *testing.T) {
	fs := newFakeMutatorState(t)
	srv := New(fs)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()
	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	suspended := true
	writeWSJSON(t, conn, wsRequestEnvelope{Type: "request", ID: "rp-set", Action: "patches.rigs.set", Payload: config.RigPatch{Name: "myrig", Suspended: &suspended}})
	var resp wsResponseEnvelope
	readWSJSON(t, conn, &resp)
	if resp.Type != "response" {
		t.Fatalf("type = %q, want response", resp.Type)
	}
	if len(fs.cfg.Patches.Rigs) != 1 {
		t.Fatalf("count = %d, want 1", len(fs.cfg.Patches.Rigs))
	}
}

func TestHandleRigPatchDelete(t *testing.T) {
	fs := newFakeMutatorState(t)
	suspended := true
	fs.cfg.Patches.Rigs = []config.RigPatch{{Name: "myrig", Suspended: &suspended}}
	srv := New(fs)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()
	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	writeWSJSON(t, conn, wsRequestEnvelope{Type: "request", ID: "rp-del", Action: "patches.rig.delete", Payload: map[string]any{"name": "myrig"}})
	var resp wsResponseEnvelope
	readWSJSON(t, conn, &resp)
	if resp.Type != "response" {
		t.Fatalf("type = %q, want response", resp.Type)
	}
	if len(fs.cfg.Patches.Rigs) != 0 {
		t.Errorf("count = %d, want 0", len(fs.cfg.Patches.Rigs))
	}
}

// --- Provider patch tests ---

func TestHandleProviderPatchList(t *testing.T) {
	fs := newFakeState(t)
	cmd := "new-cmd"
	fs.cfg.Patches.Providers = []config.ProviderPatch{{Name: "claude", Command: &cmd}}
	srv := New(fs)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()
	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	writeWSJSON(t, conn, wsRequestEnvelope{Type: "request", ID: "pp-list", Action: "patches.providers.list"})
	var resp wsResponseEnvelope
	readWSJSON(t, conn, &resp)
	var lr listResponse
	json.Unmarshal(resp.Result, &lr)
	if lr.Total != 1 {
		t.Errorf("total = %d, want 1", lr.Total)
	}
}

func TestHandleProviderPatchSet(t *testing.T) {
	fs := newFakeMutatorState(t)
	srv := New(fs)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()
	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	cmd := "my-claude"
	writeWSJSON(t, conn, wsRequestEnvelope{Type: "request", ID: "pp-set", Action: "patches.providers.set", Payload: config.ProviderPatch{Name: "claude", Command: &cmd}})
	var resp wsResponseEnvelope
	readWSJSON(t, conn, &resp)
	if resp.Type != "response" {
		t.Fatalf("type = %q, want response", resp.Type)
	}
	if len(fs.cfg.Patches.Providers) != 1 {
		t.Fatalf("count = %d, want 1", len(fs.cfg.Patches.Providers))
	}
}

func TestHandleProviderPatchDelete(t *testing.T) {
	fs := newFakeMutatorState(t)
	cmd := "my-claude"
	fs.cfg.Patches.Providers = []config.ProviderPatch{{Name: "claude", Command: &cmd}}
	srv := New(fs)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()
	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	writeWSJSON(t, conn, wsRequestEnvelope{Type: "request", ID: "pp-del", Action: "patches.provider.delete", Payload: map[string]any{"name": "claude"}})
	var resp wsResponseEnvelope
	readWSJSON(t, conn, &resp)
	if resp.Type != "response" {
		t.Fatalf("type = %q, want response", resp.Type)
	}
	if len(fs.cfg.Patches.Providers) != 0 {
		t.Errorf("count = %d, want 0", len(fs.cfg.Patches.Providers))
	}
}

func TestHandleProviderPatchDelete_NotFound(t *testing.T) {
	fs := newFakeMutatorState(t)
	srv := New(fs)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()
	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	writeWSJSON(t, conn, wsRequestEnvelope{Type: "request", ID: "pp-del-nf", Action: "patches.provider.delete", Payload: map[string]any{"name": "nonexistent"}})
	var errResp wsErrorEnvelope
	readWSJSON(t, conn, &errResp)
	if errResp.Type != "error" {
		t.Fatalf("type = %q, want error", errResp.Type)
	}
}
