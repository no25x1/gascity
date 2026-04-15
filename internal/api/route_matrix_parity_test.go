package api

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gastownhall/gascity/internal/config"
	"github.com/gastownhall/gascity/internal/events"
	"github.com/gastownhall/gascity/internal/runtime"
	"github.com/gastownhall/gascity/internal/session"
	"github.com/gastownhall/gascity/internal/workspacesvc"
	"github.com/gorilla/websocket"
)

// Route-matrix parity tests use the former HTTP/SSE route names in the test
// names and exercise the canonical WS replacements described in #646.

func TestRouteMatrixParity_GET_v0_agent_name_output_ViaWS(t *testing.T) {
	conn, info := openRouteMatrixAgentOutputSocket(t)

	sessionID := resolveAgentSessionID(t, conn, "myrig/worker")
	if sessionID != info.ID {
		t.Fatalf("resolved session id = %q, want %q", sessionID, info.ID)
	}

	writeWSJSON(t, conn, wsRequestEnvelope{
		Type:   "request",
		ID:     "route-agent-output",
		Action: "session.transcript",
		Payload: map[string]any{
			"id":    sessionID,
			"turns": 0,
		},
	})

	var resp wsResponseEnvelope
	readWSJSON(t, conn, &resp)
	if resp.Type != "response" || resp.ID != "route-agent-output" {
		t.Fatalf("response = %#v, want correlated response", resp)
	}

	var body sessionTranscriptResponse
	if err := json.Unmarshal(resp.Result, &body); err != nil {
		t.Fatalf("decode transcript: %v", err)
	}
	if body.ID != info.ID {
		t.Fatalf("transcript id = %q, want %q", body.ID, info.ID)
	}
	if body.Format != "conversation" {
		t.Fatalf("transcript format = %q, want conversation", body.Format)
	}
	if len(body.Turns) != 2 {
		t.Fatalf("turn count = %d, want 2", len(body.Turns))
	}
	if body.Turns[0].Text != "hello" || body.Turns[1].Text != "world" {
		t.Fatalf("turns = %+v, want hello/world transcript", body.Turns)
	}
}

func TestRouteMatrixParity_GET_v0_agent_name_output_stream_ViaWS(t *testing.T) {
	conn, info := openRouteMatrixAgentOutputSocket(t)

	sessionID := resolveAgentSessionID(t, conn, "myrig/worker")
	if sessionID != info.ID {
		t.Fatalf("resolved session id = %q, want %q", sessionID, info.ID)
	}

	writeWSJSON(t, conn, wsRequestEnvelope{
		Type:   "request",
		ID:     "route-agent-output-stream",
		Action: "subscription.start",
		Payload: map[string]any{
			"kind":   "session.stream",
			"target": sessionID,
		},
	})

	var resp wsResponseEnvelope
	readWSJSON(t, conn, &resp)
	if resp.Type != "response" || resp.ID != "route-agent-output-stream" {
		t.Fatalf("subscription response = %#v, want correlated response", resp)
	}

	var turnEvt wsEventEnvelope
	readWSJSON(t, conn, &turnEvt)
	if turnEvt.Type != "event" || turnEvt.EventType != "turn" {
		t.Fatalf("turn event = %#v, want turn event", turnEvt)
	}
	if !strings.Contains(string(turnEvt.Payload), `"hello"`) || !strings.Contains(string(turnEvt.Payload), `"world"`) {
		t.Fatalf("turn payload = %s, want transcript snapshot", turnEvt.Payload)
	}

	var activityEvt wsEventEnvelope
	readWSJSON(t, conn, &activityEvt)
	if activityEvt.Type != "event" || activityEvt.EventType != "activity" {
		t.Fatalf("activity event = %#v, want activity event", activityEvt)
	}
	if !strings.Contains(string(activityEvt.Payload), `"idle"`) {
		t.Fatalf("activity payload = %s, want closed-session idle state", activityEvt.Payload)
	}
}

func TestRouteMatrixParity_GET_v0_events_ViaWS(t *testing.T) {
	alpha := newFakeState(t)
	alpha.cityName = "alpha"
	beta := newFakeState(t)
	beta.cityName = "beta"

	alpha.eventProv.Record(events.Event{Type: events.SessionWoke, Actor: "alpha-mayor"})
	beta.eventProv.Record(events.Event{Type: events.SessionStopped, Actor: "beta-mayor"})

	sm := newTestSupervisorMux(t, map[string]*fakeState{
		"alpha": alpha,
		"beta":  beta,
	})
	ts := httptest.NewServer(sm.Handler())
	defer ts.Close()

	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	writeWSJSON(t, conn, wsRequestEnvelope{
		Type:   "request",
		ID:     "route-events-list",
		Action: "events.list",
	})

	var resp wsResponseEnvelope
	readWSJSON(t, conn, &resp)
	if resp.Type != "response" || resp.ID != "route-events-list" {
		t.Fatalf("response = %#v, want correlated response", resp)
	}

	var body struct {
		Items []events.TaggedEvent `json:"items"`
		Total int                  `json:"total"`
	}
	if err := json.Unmarshal(resp.Result, &body); err != nil {
		t.Fatalf("decode events list: %v", err)
	}
	if body.Total != 2 {
		t.Fatalf("total = %d, want 2", body.Total)
	}
	cities := map[string]bool{}
	for _, item := range body.Items {
		cities[item.City] = true
	}
	if !cities["alpha"] || !cities["beta"] {
		t.Fatalf("events cities = %v, want alpha and beta", cities)
	}
}

func TestRouteMatrixParity_GET_v0_events_stream_ViaWS(t *testing.T) {
	alpha := newFakeState(t)
	alpha.cityName = "alpha"
	beta := newFakeState(t)
	beta.cityName = "beta"

	sm := newTestSupervisorMux(t, map[string]*fakeState{
		"alpha": alpha,
		"beta":  beta,
	})
	ts := httptest.NewServer(sm.Handler())
	defer ts.Close()

	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	writeWSJSON(t, conn, wsRequestEnvelope{
		Type:   "request",
		ID:     "route-events-stream",
		Action: "subscription.start",
		Payload: map[string]any{
			"kind": "events",
		},
	})

	var resp wsResponseEnvelope
	readWSJSON(t, conn, &resp)
	if resp.Type != "response" || resp.ID != "route-events-stream" {
		t.Fatalf("subscription response = %#v, want correlated response", resp)
	}

	alpha.eventProv.Record(events.Event{Type: events.SessionWoke, Actor: "alpha-mayor"})

	var evt wsEventEnvelope
	readWSJSON(t, conn, &evt)
	if evt.Type != "event" || evt.EventType != events.SessionWoke {
		t.Fatalf("event = %#v, want session.woke event", evt)
	}
	if evt.Cursor == "" {
		t.Fatal("global event cursor empty")
	}

	var payload struct {
		City string `json:"city"`
		Type string `json:"type"`
	}
	if err := json.Unmarshal(evt.Payload, &payload); err != nil {
		t.Fatalf("decode event payload: %v", err)
	}
	if payload.City != "alpha" || payload.Type != events.SessionWoke {
		t.Fatalf("payload = %+v, want city alpha type %q", payload, events.SessionWoke)
	}
}

func TestRouteMatrixParity_GET_v0_beads_index_wait_ViaWSWatch(t *testing.T) {
	state := newFakeState(t)
	srv := New(state)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	go func() {
		time.Sleep(75 * time.Millisecond)
		state.eventProv.Record(events.Event{Type: "bead.changed", Actor: "tester"})
	}()

	start := time.Now()
	writeWSJSON(t, conn, map[string]any{
		"type":   "request",
		"id":     "route-beads-watch",
		"action": "beads.list",
		"payload": map[string]any{
			"limit": 10,
		},
		"watch": map[string]any{
			"index": 0,
			"wait":  "1s",
		},
	})

	var resp wsResponseEnvelope
	readWSJSON(t, conn, &resp)
	elapsed := time.Since(start)

	if resp.Type != "response" || resp.ID != "route-beads-watch" {
		t.Fatalf("response = %#v, want correlated response", resp)
	}
	if resp.Index == 0 {
		t.Fatal("watch response index = 0, want event-driven index > 0")
	}
	if elapsed < 50*time.Millisecond || elapsed > 900*time.Millisecond {
		t.Fatalf("watch elapsed = %v, want delayed unblock before timeout", elapsed)
	}
}

func TestRouteMatrixParity_GET_v0_packs_ViaWS(t *testing.T) {
	state := newFakeState(t)
	state.cfg.Packs = map[string]config.PackSource{
		"gastown": {
			Source: "https://github.com/example/gastown-pack",
			Ref:    "v1.0.0",
			Path:   "packs/gastown",
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
		ID:     "route-packs-list",
		Action: "packs.list",
	})

	var resp wsResponseEnvelope
	readWSJSON(t, conn, &resp)
	if resp.Type != "response" || resp.ID != "route-packs-list" {
		t.Fatalf("response = %#v, want correlated response", resp)
	}

	var body struct {
		Packs []packResponse `json:"packs"`
	}
	if err := json.Unmarshal(resp.Result, &body); err != nil {
		t.Fatalf("decode packs list: %v", err)
	}
	if len(body.Packs) != 1 || body.Packs[0].Name != "gastown" {
		t.Fatalf("packs = %+v, want gastown pack", body.Packs)
	}
}

func TestRouteMatrixParity_POST_v0_service_name_restart_ViaWS(t *testing.T) {
	state := newFakeState(t)
	state.services = &fakeServiceRegistry{
		items: []workspacesvc.Status{{
			ServiceName:      "review-intake",
			Kind:             "workflow",
			WorkflowContract: "pack.gc/review-intake.v1",
			MountPath:        "/svc/review-intake",
			PublishMode:      "private",
			StateRoot:        ".gc/services/review-intake",
			State:            "ready",
			LocalState:       "ready",
			PublicationState: "private",
		}},
	}
	srv := New(state)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	writeWSJSON(t, conn, wsRequestEnvelope{
		Type:   "request",
		ID:     "route-service-restart",
		Action: "service.restart",
		Payload: map[string]any{
			"name": "review-intake",
		},
	})

	var resp wsResponseEnvelope
	readWSJSON(t, conn, &resp)
	if resp.Type != "response" || resp.ID != "route-service-restart" {
		t.Fatalf("response = %#v, want correlated response", resp)
	}

	var body map[string]string
	if err := json.Unmarshal(resp.Result, &body); err != nil {
		t.Fatalf("decode restart response: %v", err)
	}
	if body["status"] != "ok" || body["action"] != "restart" || body["service"] != "review-intake" {
		t.Fatalf("restart body = %+v, want status ok action restart service review-intake", body)
	}
}

func openRouteMatrixAgentOutputSocket(t *testing.T) (*websocket.Conn, session.Info) {
	t.Helper()

	fs := newSessionFakeState(t)
	searchBase := t.TempDir()
	srv := New(fs)
	srv.sessionLogSearchPaths = []string{searchBase}

	mgr := session.NewManager(fs.cityBeadStore, fs.sp)
	resume := session.ProviderResume{
		ResumeFlag:    "--resume",
		ResumeStyle:   "flag",
		SessionIDFlag: "--session-id",
	}
	workDir := t.TempDir()
	info, err := mgr.Create(context.Background(), "myrig/worker", "Chat", "claude", workDir, "claude", nil, resume, runtime.Config{})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	writeNamedSessionJSONL(t, searchBase, workDir, info.SessionKey+".jsonl",
		`{"uuid":"1","parentUuid":"","type":"user","message":"{\"role\":\"user\",\"content\":\"hello\"}","timestamp":"2025-01-01T00:00:00Z"}`,
		`{"uuid":"2","parentUuid":"1","type":"assistant","message":"{\"role\":\"assistant\",\"content\":\"world\"}","timestamp":"2025-01-01T00:00:01Z"}`,
	)
	if err := mgr.Close(info.ID); err != nil {
		t.Fatalf("Close: %v", err)
	}

	ts := httptest.NewServer(srv.handler())
	t.Cleanup(ts.Close)

	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	t.Cleanup(func() { _ = conn.Close() })
	drainWSHello(t, conn)
	return conn, info
}

func resolveAgentSessionID(t *testing.T, conn *websocket.Conn, agentName string) string {
	t.Helper()

	writeWSJSON(t, conn, wsRequestEnvelope{
		Type:   "request",
		ID:     "route-agent-get",
		Action: "agent.get",
		Payload: map[string]any{
			"name": agentName,
		},
	})

	var resp wsResponseEnvelope
	readWSJSON(t, conn, &resp)
	if resp.Type != "response" || resp.ID != "route-agent-get" {
		t.Fatalf("agent.get response = %#v, want correlated response", resp)
	}

	var body agentResponse
	if err := json.Unmarshal(resp.Result, &body); err != nil {
		t.Fatalf("decode agent.get: %v", err)
	}
	if body.Session == nil || body.Session.ID == "" {
		t.Fatalf("agent.get session = %+v, want canonical session id", body.Session)
	}
	return body.Session.ID
}
