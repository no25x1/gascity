package api

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gastownhall/gascity/internal/config"
)

func TestHandleProviderList(t *testing.T) {
	fs := newFakeState(t)
	fs.cfg.Providers = map[string]config.ProviderSpec{
		"custom": {DisplayName: "Custom Agent", Command: "custom-cli"},
		"claude": {DisplayName: "My Claude", Command: "my-claude"},
	}
	srv := New(fs)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()
	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	writeWSJSON(t, conn, wsRequestEnvelope{Type: "request", ID: "p-list", Action: "providers.list"})
	var resp wsResponseEnvelope
	readWSJSON(t, conn, &resp)
	if resp.Type != "response" {
		t.Fatalf("type = %q, want response", resp.Type)
	}
	var lr listResponse
	json.Unmarshal(resp.Result, &lr)
	if lr.Total < 10 {
		t.Errorf("total = %d, want >= 10 (builtins)", lr.Total)
	}
}

func TestHandleProviderGet_CityLevel(t *testing.T) {
	fs := newFakeState(t)
	fs.cfg.Providers = map[string]config.ProviderSpec{
		"custom": {DisplayName: "Custom Agent", Command: "custom-cli"},
	}
	srv := New(fs)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()
	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	writeWSJSON(t, conn, wsRequestEnvelope{Type: "request", ID: "p-get", Action: "provider.get", Payload: map[string]any{"name": "custom"}})
	var resp wsResponseEnvelope
	readWSJSON(t, conn, &resp)
	if resp.Type != "response" {
		t.Fatalf("type = %q, want response", resp.Type)
	}
	var pr providerResponse
	json.Unmarshal(resp.Result, &pr)
	if pr.Name != "custom" {
		t.Errorf("name = %q, want custom", pr.Name)
	}
	if !pr.CityLevel {
		t.Error("expected city_level=true")
	}
}

func TestHandleProviderGet_Builtin(t *testing.T) {
	fs := newFakeState(t)
	srv := New(fs)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()
	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	writeWSJSON(t, conn, wsRequestEnvelope{Type: "request", ID: "p-builtin", Action: "provider.get", Payload: map[string]any{"name": "claude"}})
	var resp wsResponseEnvelope
	readWSJSON(t, conn, &resp)
	if resp.Type != "response" {
		t.Fatalf("type = %q, want response", resp.Type)
	}
	var pr providerResponse
	json.Unmarshal(resp.Result, &pr)
	if pr.Name != "claude" {
		t.Errorf("name = %q, want claude", pr.Name)
	}
	if !pr.Builtin {
		t.Error("expected builtin=true")
	}
}

func TestHandleProviderGet_NotFound(t *testing.T) {
	fs := newFakeState(t)
	srv := New(fs)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()
	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	writeWSJSON(t, conn, wsRequestEnvelope{Type: "request", ID: "p-nf", Action: "provider.get", Payload: map[string]any{"name": "nonexistent"}})
	var errResp wsErrorEnvelope
	readWSJSON(t, conn, &errResp)
	if errResp.Type != "error" || errResp.Code != "not_found" {
		t.Fatalf("expected not_found error, got %#v", errResp)
	}
}

func TestHandleProviderCreate(t *testing.T) {
	fs := newFakeMutatorState(t)
	srv := New(fs)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()
	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	writeWSJSON(t, conn, wsRequestEnvelope{Type: "request", ID: "p-create", Action: "provider.create", Payload: map[string]any{
		"name": "myagent",
		"spec": map[string]any{"command": "myagent-cli", "display_name": "My Agent"},
	}})
	var resp wsResponseEnvelope
	readWSJSON(t, conn, &resp)
	if resp.Type != "response" {
		t.Fatalf("type = %q, want response; result = %s", resp.Type, resp.Result)
	}

	spec, ok := fs.cfg.Providers["myagent"]
	if !ok {
		t.Fatal("provider 'myagent' not found after create")
	}
	if spec.Command != "myagent-cli" {
		t.Errorf("command = %q, want myagent-cli", spec.Command)
	}
}

func TestHandleProviderCreate_MissingName(t *testing.T) {
	fs := newFakeMutatorState(t)
	srv := New(fs)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()
	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	writeWSJSON(t, conn, wsRequestEnvelope{Type: "request", ID: "p-no-name", Action: "provider.create", Payload: map[string]any{
		"spec": map[string]any{"command": "cli"},
	}})
	var errResp wsErrorEnvelope
	readWSJSON(t, conn, &errResp)
	if errResp.Type != "error" || errResp.Code != "invalid" {
		t.Fatalf("expected invalid error, got %#v", errResp)
	}
}

func TestHandleProviderUpdate(t *testing.T) {
	fs := newFakeMutatorState(t)
	fs.cfg.Providers = map[string]config.ProviderSpec{
		"custom": {Command: "old-cli", DisplayName: "Old Name"},
	}
	srv := New(fs)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()
	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	cmd := "new-cli"
	dn := "New Name"
	writeWSJSON(t, conn, wsRequestEnvelope{Type: "request", ID: "p-update", Action: "provider.update", Payload: map[string]any{
		"name":   "custom",
		"update": map[string]any{"command": &cmd, "display_name": &dn},
	}})
	var resp wsResponseEnvelope
	readWSJSON(t, conn, &resp)
	if resp.Type != "response" {
		t.Fatalf("type = %q, want response", resp.Type)
	}
}

func TestHandleProviderUpdate_NotFound(t *testing.T) {
	fs := newFakeMutatorState(t)
	srv := New(fs)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()
	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	writeWSJSON(t, conn, wsRequestEnvelope{Type: "request", ID: "p-upd-nf", Action: "provider.update", Payload: map[string]any{
		"name":   "nonexistent",
		"update": map[string]any{},
	}})
	var errResp wsErrorEnvelope
	readWSJSON(t, conn, &errResp)
	if errResp.Type != "error" {
		t.Fatalf("type = %q, want error", errResp.Type)
	}
}

func TestHandleProviderDelete(t *testing.T) {
	fs := newFakeMutatorState(t)
	fs.cfg.Providers = map[string]config.ProviderSpec{
		"custom": {Command: "custom-cli"},
	}
	srv := New(fs)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()
	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	writeWSJSON(t, conn, wsRequestEnvelope{Type: "request", ID: "p-delete", Action: "provider.delete", Payload: map[string]any{"name": "custom"}})
	var resp wsResponseEnvelope
	readWSJSON(t, conn, &resp)
	if resp.Type != "response" {
		t.Fatalf("type = %q, want response", resp.Type)
	}

	if _, ok := fs.cfg.Providers["custom"]; ok {
		t.Error("provider still exists after delete")
	}
}

func TestHandleProviderDelete_NotFound(t *testing.T) {
	fs := newFakeMutatorState(t)
	srv := New(fs)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()
	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	writeWSJSON(t, conn, wsRequestEnvelope{Type: "request", ID: "p-del-nf", Action: "provider.delete", Payload: map[string]any{"name": "nonexistent"}})
	var errResp wsErrorEnvelope
	readWSJSON(t, conn, &errResp)
	if errResp.Type != "error" {
		t.Fatalf("type = %q, want error", errResp.Type)
	}
}
