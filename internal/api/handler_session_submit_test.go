package api

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gastownhall/gascity/internal/beads"
	"github.com/gastownhall/gascity/internal/nudgequeue"
	"github.com/gastownhall/gascity/internal/runtime"
	"github.com/gastownhall/gascity/internal/session"
)

func TestHandleSessionSubmitDefaultsToProviderDefaultBehavior(t *testing.T) {
	fs := newSessionFakeState(t)
	info := createTestSession(t, fs.cityBeadStore, fs.sp, "Submit Me")
	mgr := session.NewManager(fs.cityBeadStore, fs.sp)
	if err := mgr.Suspend(info.ID); err != nil {
		t.Fatalf("Suspend: %v", err)
	}
	srv := New(fs)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	writeWSJSON(t, conn, wsRequestEnvelope{
		Type:   "request",
		ID:     "t1",
		Action: "session.submit",
		Payload: map[string]any{
			"id":      info.ID,
			"message": "hello",
		},
	})

	var resp wsResponseEnvelope
	readWSJSON(t, conn, &resp)
	if resp.Type != "response" || resp.ID != "t1" {
		t.Fatalf("response = %#v, want correlated response", resp)
	}
	var body map[string]any
	if err := json.Unmarshal(resp.Result, &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got := body["queued"]; got != false {
		t.Fatalf("queued = %#v, want false", got)
	}
	if got := body["intent"]; got != string(session.SubmitIntentDefault) {
		t.Fatalf("intent = %#v, want %q", got, session.SubmitIntentDefault)
	}
	if !fs.sp.IsRunning(info.SessionName) {
		t.Fatal("session should be running after submit")
	}
	found := false
	for _, call := range fs.sp.Calls {
		if call.Method == "Nudge" && call.Name == info.SessionName && call.Message == "hello" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("calls = %#v, want Nudge(hello)", fs.sp.Calls)
	}
}

func TestHandleSessionSubmitUsesImmediateDefaultForCodex(t *testing.T) {
	fs := newSessionFakeState(t)
	mgr := session.NewManager(fs.cityBeadStore, fs.sp)
	info, err := mgr.Create(context.Background(), "helper", "Codex Submit", "codex", t.TempDir(), "codex", nil, session.ProviderResume{}, runtime.Config{})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := mgr.Suspend(info.ID); err != nil {
		t.Fatalf("Suspend: %v", err)
	}
	srv := New(fs)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	writeWSJSON(t, conn, wsRequestEnvelope{
		Type:   "request",
		ID:     "t1",
		Action: "session.submit",
		Payload: map[string]any{
			"id":      info.ID,
			"message": "hello",
		},
	})

	var resp wsResponseEnvelope
	readWSJSON(t, conn, &resp)
	if resp.Type != "response" || resp.ID != "t1" {
		t.Fatalf("response = %#v, want correlated response", resp)
	}
	found := false
	for _, call := range fs.sp.Calls {
		if call.Method == "NudgeNow" && call.Name == info.SessionName && call.Message == "hello" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("calls = %#v, want NudgeNow(hello)", fs.sp.Calls)
	}
}

func TestHandleSessionSubmitFollowUpQueuesMessage(t *testing.T) {
	fs := newSessionFakeState(t)
	info := createTestSession(t, fs.cityBeadStore, fs.sp, "Queue Me")
	srv := New(fs)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	writeWSJSON(t, conn, wsRequestEnvelope{
		Type:   "request",
		ID:     "t1",
		Action: "session.submit",
		Payload: map[string]any{
			"id":      info.ID,
			"message": "later please",
			"intent":  "follow_up",
		},
	})

	var resp wsResponseEnvelope
	readWSJSON(t, conn, &resp)
	if resp.Type != "response" || resp.ID != "t1" {
		t.Fatalf("response = %#v, want correlated response", resp)
	}
	var body map[string]any
	if err := json.Unmarshal(resp.Result, &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got := body["queued"]; got != true {
		t.Fatalf("queued = %#v, want true", got)
	}
	state, err := nudgequeue.LoadState(fs.cityPath)
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	if len(state.Pending) != 1 {
		t.Fatalf("pending queued submits = %d, want 1", len(state.Pending))
	}
	item := state.Pending[0]
	if item.SessionID != info.ID {
		t.Fatalf("SessionID = %q, want %q", item.SessionID, info.ID)
	}
	if item.Message != "later please" {
		t.Fatalf("Message = %q, want %q", item.Message, "later please")
	}
}

func TestHandleSessionGetIncludesSubmissionCapabilities(t *testing.T) {
	fs := newSessionFakeState(t)
	info := createTestSession(t, fs.cityBeadStore, fs.sp, "Capabilities")
	if err := fs.cityBeadStore.Update(info.ID, beads.UpdateOpts{
		Metadata: map[string]string{
			"pool_managed": "true",
			"pool_slot":    "1",
		},
	}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	srv := New(fs)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	writeWSJSON(t, conn, wsRequestEnvelope{
		Type:   "request",
		ID:     "t1",
		Action: "session.get",
		Payload: map[string]any{
			"id": info.ID,
		},
	})

	var resp wsResponseEnvelope
	readWSJSON(t, conn, &resp)
	if resp.Type != "response" || resp.ID != "t1" {
		t.Fatalf("response = %#v, want correlated response", resp)
	}
	var body sessionResponse
	if err := json.Unmarshal(resp.Result, &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !body.SubmissionCapabilities.SupportsFollowUp {
		t.Fatal("SupportsFollowUp = false, want true")
	}
	if !body.SubmissionCapabilities.SupportsInterruptNow {
		t.Fatal("SupportsInterruptNow = false, want true")
	}
}

func TestHandleSessionStopUsesSoftEscapeForCodex(t *testing.T) {
	fs := newSessionFakeState(t)
	mgr := session.NewManager(fs.cityBeadStore, fs.sp)
	info, err := mgr.Create(context.Background(), "helper", "Codex", "codex", t.TempDir(), "codex", nil, session.ProviderResume{}, runtime.Config{})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := fs.cityBeadStore.Update(info.ID, beads.UpdateOpts{
		Metadata: map[string]string{"pool_managed": "true"},
	}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	srv := New(fs)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	writeWSJSON(t, conn, wsRequestEnvelope{
		Type:   "request",
		ID:     "t1",
		Action: "session.stop",
		Payload: map[string]any{
			"id": info.ID,
		},
	})

	var resp wsResponseEnvelope
	readWSJSON(t, conn, &resp)
	if resp.Type != "response" || resp.ID != "t1" {
		t.Fatalf("response = %#v, want correlated response", resp)
	}
	var sawEscape, sawInterrupt bool
	for _, call := range fs.sp.Calls {
		if call.Method == "SendKeys" && call.Name == info.SessionName && call.Message == "Escape" {
			sawEscape = true
		}
		if call.Method == "Interrupt" && call.Name == info.SessionName {
			sawInterrupt = true
		}
	}
	if !sawEscape {
		t.Fatalf("calls = %#v, want SendKeys(Escape)", fs.sp.Calls)
	}
	if sawInterrupt {
		t.Fatalf("calls = %#v, did not want Interrupt for codex stop", fs.sp.Calls)
	}
}
