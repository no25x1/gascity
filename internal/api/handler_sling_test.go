package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/gastownhall/gascity/internal/config"
)

func wsSling(t *testing.T, ts *httptest.Server, payload map[string]any) (wsResponseEnvelope, wsErrorEnvelope) {
	t.Helper()
	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)
	writeWSJSON(t, conn, wsRequestEnvelope{Type: "request", ID: "sling-1", Action: "sling.run", Payload: payload})
	var raw map[string]json.RawMessage
	conn.ReadJSON(&raw) //nolint:errcheck
	var msgType string
	json.Unmarshal(raw["type"], &msgType)
	if msgType == "response" {
		var resp wsResponseEnvelope
		data, _ := json.Marshal(raw)
		json.Unmarshal(data, &resp)
		return resp, wsErrorEnvelope{}
	}
	var errResp wsErrorEnvelope
	data, _ := json.Marshal(raw)
	json.Unmarshal(data, &errResp)
	return wsResponseEnvelope{}, errResp
}

func TestSlingWithBead(t *testing.T) {
	state := newFakeMutatorState(t)
	srv := New(state)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	oldRunner := slingCommandRunner
	defer func() { slingCommandRunner = oldRunner }()

	var gotArgs []string
	slingCommandRunner = func(_ context.Context, _ string, args []string) (string, string, error) {
		gotArgs = args
		return "Slung test-1 → myrig/worker\n", "", nil
	}

	resp, errResp := wsSling(t, ts, map[string]any{"target": "myrig/worker", "bead": "test-1"})
	if errResp.Type == "error" {
		t.Fatalf("error: %s: %s", errResp.Code, errResp.Message)
	}
	var result map[string]string
	json.Unmarshal(resp.Result, &result)
	if result["status"] != "slung" {
		t.Fatalf("status = %q, want slung", result["status"])
	}
	if len(gotArgs) < 4 || gotArgs[2] != "sling" || gotArgs[3] != "myrig/worker" || gotArgs[4] != "test-1" {
		t.Fatalf("unexpected args: %v", gotArgs)
	}
}

func TestSlingMissingTarget(t *testing.T) {
	state := newFakeMutatorState(t)
	srv := New(state)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	_, errResp := wsSling(t, ts, map[string]any{"bead": "abc"})
	if errResp.Type != "error" {
		t.Fatalf("expected error, got response")
	}
}

func TestSlingTargetNotFound(t *testing.T) {
	state := newFakeMutatorState(t)
	srv := New(state)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	_, errResp := wsSling(t, ts, map[string]any{"target": "nonexistent", "bead": "abc"})
	if errResp.Type != "error" {
		t.Fatalf("expected error, got response")
	}
}

func TestSlingMissingBeadAndFormula(t *testing.T) {
	state := newFakeMutatorState(t)
	srv := New(state)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	_, errResp := wsSling(t, ts, map[string]any{"target": "myrig/worker"})
	if errResp.Type != "error" {
		t.Fatalf("expected error, got response")
	}
}

func TestSlingBeadAndFormulaMutuallyExclusive(t *testing.T) {
	state := newFakeMutatorState(t)
	srv := New(state)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	_, errResp := wsSling(t, ts, map[string]any{"target": "myrig/worker", "bead": "abc", "formula": "xyz"})
	if errResp.Type != "error" {
		t.Fatalf("expected error, got response")
	}
}

func TestSlingBeadNotFound(t *testing.T) {
	state := newFakeMutatorState(t)
	srv := New(state)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	oldRunner := slingCommandRunner
	defer func() { slingCommandRunner = oldRunner }()
	slingCommandRunner = func(_ context.Context, _ string, _ []string) (string, string, error) {
		return "", "bead nonexistent not found", errors.New("exit status 1")
	}

	_, errResp := wsSling(t, ts, map[string]any{"target": "myrig/worker", "bead": "nonexistent"})
	if errResp.Type != "error" {
		t.Fatalf("expected error, got response")
	}
}

func TestSlingFormulaDelegatesToGcSling(t *testing.T) {
	state := newFakeMutatorState(t)
	srv := New(state)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	oldRunner := slingCommandRunner
	defer func() { slingCommandRunner = oldRunner }()

	var gotCityPath string
	var gotArgs []string
	slingCommandRunner = func(_ context.Context, cityPath string, args []string) (string, string, error) {
		gotCityPath = cityPath
		gotArgs = append([]string(nil), args...)
		return "Started workflow wf_123 (formula \"mol-review\") → myrig/worker\n", "", nil
	}

	resp, errResp := wsSling(t, ts, map[string]any{
		"target": "myrig/worker", "formula": "mol-review",
		"scope_kind": "city", "scope_ref": "test-city",
		"vars": map[string]string{"pr_url": "https://example.test/pr/123"},
	})
	if errResp.Type == "error" {
		t.Fatalf("error: %s: %s", errResp.Code, errResp.Message)
	}
	if gotCityPath != state.CityPath() {
		t.Fatalf("cityPath = %q, want %q", gotCityPath, state.CityPath())
	}
	wantArgs := []string{
		"--city", state.CityPath(),
		"sling", "myrig/worker", "mol-review", "--formula",
		"--scope-kind", "city",
		"--scope-ref", "test-city",
		"--var", "pr_url=https://example.test/pr/123",
	}
	if !reflect.DeepEqual(gotArgs, wantArgs) {
		t.Fatalf("args = %#v, want %#v", gotArgs, wantArgs)
	}

	var result slingResponse
	json.Unmarshal(resp.Result, &result)
	if result.WorkflowID != "wf_123" || result.RootBeadID != "wf_123" {
		t.Fatalf("response = %+v, want workflow/root wf_123", result)
	}
}

func TestSlingPoolTargetDelegatesToGcSling(t *testing.T) {
	state := newFakeMutatorState(t)
	state.cfg.Agents = []config.Agent{
		{Name: "polecat", Dir: "myrig", MinActiveSessions: intPtr(0), MaxActiveSessions: intPtr(3)},
	}
	srv := New(state)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	oldRunner := slingCommandRunner
	defer func() { slingCommandRunner = oldRunner }()
	var gotArgs []string
	slingCommandRunner = func(_ context.Context, _ string, args []string) (string, string, error) {
		gotArgs = append([]string(nil), args...)
		return "Started workflow wf_pool (formula \"mol-review\") → myrig/polecat\n", "", nil
	}

	resp, errResp := wsSling(t, ts, map[string]any{
		"target": "myrig/polecat", "formula": "mol-review",
		"scope_kind": "city", "scope_ref": "test-city",
	})
	if errResp.Type == "error" {
		t.Fatalf("error: %s", errResp.Message)
	}
	_ = resp
	wantArgs := []string{
		"--city", state.CityPath(),
		"sling", "myrig/polecat", "mol-review", "--formula",
		"--scope-kind", "city", "--scope-ref", "test-city",
	}
	if !reflect.DeepEqual(gotArgs, wantArgs) {
		t.Fatalf("args = %#v, want %#v", gotArgs, wantArgs)
	}
}

func TestSlingFormulaParsesWispRootOutput(t *testing.T) {
	state := newFakeMutatorState(t)
	state.cfg.Agents = []config.Agent{
		{Name: "polecat", Dir: "myrig", MinActiveSessions: intPtr(0), MaxActiveSessions: intPtr(3)},
	}
	srv := New(state)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	oldRunner := slingCommandRunner
	defer func() { slingCommandRunner = oldRunner }()
	slingCommandRunner = func(_ context.Context, _ string, _ []string) (string, string, error) {
		return "Slung formula \"mol-review\" (wisp root wf_pool) → myrig/polecat\n", "", nil
	}

	resp, _ := wsSling(t, ts, map[string]any{
		"target": "myrig/polecat", "formula": "mol-review",
		"scope_kind": "city", "scope_ref": "test-city",
	})
	var result slingResponse
	json.Unmarshal(resp.Result, &result)
	if result.WorkflowID != "wf_pool" || result.RootBeadID != "wf_pool" {
		t.Fatalf("response = %+v, want workflow/root wf_pool", result)
	}
}

func TestSlingAttachedFormulaDelegatesToGcSling(t *testing.T) {
	state := newFakeMutatorState(t)
	srv := New(state)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	oldRunner := slingCommandRunner
	defer func() { slingCommandRunner = oldRunner }()
	var gotArgs []string
	slingCommandRunner = func(_ context.Context, _ string, args []string) (string, string, error) {
		gotArgs = append([]string(nil), args...)
		return "Attached workflow wf_456 (formula \"mol-review\") to BD-42\n", "", nil
	}

	resp, _ := wsSling(t, ts, map[string]any{
		"target": "myrig/worker", "formula": "mol-review",
		"attached_bead_id": "BD-42",
		"scope_kind": "city", "scope_ref": "test-city",
		"vars": map[string]string{"issue": "BD-42"},
	})
	wantArgs := []string{
		"--city", state.CityPath(),
		"sling", "myrig/worker", "BD-42", "--on", "mol-review",
		"--scope-kind", "city", "--scope-ref", "test-city",
		"--var", "issue=BD-42",
	}
	if !reflect.DeepEqual(gotArgs, wantArgs) {
		t.Fatalf("args = %#v, want %#v", gotArgs, wantArgs)
	}
	var result slingResponse
	json.Unmarshal(resp.Result, &result)
	if result.Mode != "attached" || result.AttachedBeadID != "BD-42" {
		t.Fatalf("response = %+v, want attached on BD-42", result)
	}
}

func TestSlingBeadWithDefaultFormulaDelegatesToGcSling(t *testing.T) {
	state := newFakeMutatorState(t)
	molReview := "mol-review"
	state.cfg.Agents[0].DefaultSlingFormula = &molReview
	srv := New(state)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	oldRunner := slingCommandRunner
	defer func() { slingCommandRunner = oldRunner }()
	var gotArgs []string
	slingCommandRunner = func(_ context.Context, _ string, args []string) (string, string, error) {
		gotArgs = append([]string(nil), args...)
		return "Attached workflow wf_789 (default formula \"mol-review\") to BD-42\n", "", nil
	}

	resp, _ := wsSling(t, ts, map[string]any{
		"target": "myrig/worker", "bead": "BD-42", "title": "Review PR",
		"scope_kind": "city", "scope_ref": "test-city",
		"vars": map[string]string{"issue": "BD-42"},
	})
	wantArgs := []string{
		"--city", state.CityPath(),
		"sling", "myrig/worker", "BD-42",
		"--title", "Review PR",
		"--scope-kind", "city", "--scope-ref", "test-city",
		"--var", "issue=BD-42",
	}
	if !reflect.DeepEqual(gotArgs, wantArgs) {
		t.Fatalf("args = %#v, want %#v", gotArgs, wantArgs)
	}
	var result slingResponse
	json.Unmarshal(resp.Result, &result)
	if result.WorkflowID != "wf_789" || result.Formula != "mol-review" {
		t.Fatalf("response = %+v", result)
	}
}

func TestSlingRejectsVarsWithoutFormula(t *testing.T) {
	state := newFakeMutatorState(t)
	srv := New(state)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	_, errResp := wsSling(t, ts, map[string]any{"target": "myrig/worker", "bead": "BD-42", "vars": map[string]string{"issue": "BD-42"}})
	if errResp.Type != "error" {
		t.Fatalf("expected error")
	}
}

func TestSlingRejectsScopeWithoutFormula(t *testing.T) {
	state := newFakeMutatorState(t)
	srv := New(state)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	_, errResp := wsSling(t, ts, map[string]any{"target": "myrig/worker", "bead": "BD-42", "scope_kind": "city", "scope_ref": "test-city"})
	if errResp.Type != "error" {
		t.Fatalf("expected error")
	}
}

func TestSlingRejectsPartialScope(t *testing.T) {
	state := newFakeMutatorState(t)
	srv := New(state)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	_, errResp := wsSling(t, ts, map[string]any{"target": "myrig/worker", "formula": "mol-review", "scope_kind": "city"})
	if errResp.Type != "error" {
		t.Fatalf("expected error")
	}
}

func TestSlingFormulaRunnerErrorSurfacesAsBadRequest(t *testing.T) {
	state := newFakeMutatorState(t)
	srv := New(state)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	oldRunner := slingCommandRunner
	defer func() { slingCommandRunner = oldRunner }()
	slingCommandRunner = func(_ context.Context, _ string, _ []string) (string, string, error) {
		return "", "gc sling: could not resolve session name", errors.New("exit status 1")
	}

	_, errResp := wsSling(t, ts, map[string]any{"target": "myrig/worker", "formula": "mol-review"})
	if errResp.Type != "error" {
		t.Fatalf("expected error")
	}
	if !strings.Contains(errResp.Message, "could not resolve session name") {
		t.Fatalf("message = %q, want session resolution error", errResp.Message)
	}
}
