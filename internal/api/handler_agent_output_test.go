package api

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gastownhall/gascity/internal/config"
)

// writeSessionJSONL creates a JSONL session file at the slug path for the
// given workDir.
func writeSessionJSONL(t *testing.T, searchBase, workDir string, lines ...string) {
	t.Helper()
	slug := strings.ReplaceAll(workDir, "/", "-")
	slug = strings.ReplaceAll(slug, ".", "-")
	dir := filepath.Join(searchBase, slug)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "test-session.jsonl")
	content := strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func newServerWithSearchPaths(state State, searchBase string) *Server {
	s := New(state)
	s.sessionLogSearchPaths = []string{searchBase}
	return s
}

func TestAgentOutputConversation(t *testing.T) {
	state := newFakeState(t)
	rigDir := t.TempDir()
	state.cfg.Rigs = []config.Rig{{Name: "myrig", Path: rigDir}}

	searchBase := t.TempDir()
	writeSessionJSONL(t, searchBase, rigDir,
		`{"uuid":"1","parentUuid":"","type":"user","message":"{\"role\":\"user\",\"content\":\"hello\"}","timestamp":"2025-01-01T00:00:00Z"}`,
		`{"uuid":"2","parentUuid":"1","type":"assistant","message":"{\"role\":\"assistant\",\"content\":\"world\"}","timestamp":"2025-01-01T00:00:01Z"}`,
	)

	srv := newServerWithSearchPaths(state, searchBase)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	writeWSJSON(t, conn, wsRequestEnvelope{
		Type:    "request",
		ID:      "test-agent-output",
		Action:  "agent.output.get",
		Payload: socketAgentOutputPayload{Name: "myrig/worker", Tail: intPtr(0)},
	})

	var resp wsResponseEnvelope
	readWSJSON(t, conn, &resp)
	if resp.Type != "response" || resp.ID != "test-agent-output" {
		t.Fatalf("response = %#v, want correlated response", resp)
	}

	var body agentOutputResponse
	if err := json.Unmarshal(resp.Result, &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Agent != "myrig/worker" {
		t.Errorf("Agent = %q, want %q", body.Agent, "myrig/worker")
	}
	if body.Format != "conversation" {
		t.Errorf("Format = %q, want %q", body.Format, "conversation")
	}
	if len(body.Turns) != 2 {
		t.Fatalf("got %d turns, want 2", len(body.Turns))
	}
	if body.Turns[0].Role != "user" || body.Turns[0].Text != "hello" {
		t.Errorf("turn[0] = %+v, want role=user text=hello", body.Turns[0])
	}
	if body.Turns[1].Role != "assistant" || body.Turns[1].Text != "world" {
		t.Errorf("turn[1] = %+v, want role=assistant text=world", body.Turns[1])
	}
}

func TestAgentOutputConversationUsesConfiguredWorkDir(t *testing.T) {
	state := newFakeState(t)
	rigDir := t.TempDir()
	state.cfg.Rigs = []config.Rig{{Name: "myrig", Path: rigDir}}
	state.cfg.Agents[0].WorkDir = ".gc/worktrees/{{.Rig}}/{{.AgentBase}}"

	searchBase := t.TempDir()
	workDir := filepath.Join(state.cityPath, ".gc", "worktrees", "myrig", "worker")
	writeSessionJSONL(t, searchBase, workDir,
		`{"uuid":"1","parentUuid":"","type":"user","message":"{\"role\":\"user\",\"content\":\"hello\"}","timestamp":"2025-01-01T00:00:00Z"}`,
		`{"uuid":"2","parentUuid":"1","type":"assistant","message":"{\"role\":\"assistant\",\"content\":\"from workdir\"}","timestamp":"2025-01-01T00:00:01Z"}`,
	)

	srv := newServerWithSearchPaths(state, searchBase)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	writeWSJSON(t, conn, wsRequestEnvelope{
		Type:    "request",
		ID:      "test-agent-output-workdir",
		Action:  "agent.output.get",
		Payload: socketAgentOutputPayload{Name: "myrig/worker", Tail: intPtr(0)},
	})

	var resp wsResponseEnvelope
	readWSJSON(t, conn, &resp)
	if resp.Type != "response" || resp.ID != "test-agent-output-workdir" {
		t.Fatalf("response = %#v, want correlated response", resp)
	}

	var body agentOutputResponse
	if err := json.Unmarshal(resp.Result, &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Format != "conversation" {
		t.Fatalf("Format = %q, want conversation", body.Format)
	}
	if len(body.Turns) != 2 || body.Turns[1].Text != "from workdir" {
		t.Fatalf("Turns = %+v, want configured work_dir session log", body.Turns)
	}
}

func TestAgentOutputNotFound(t *testing.T) {
	state := newFakeState(t)
	srv := New(state)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	writeWSJSON(t, conn, wsRequestEnvelope{
		Type:    "request",
		ID:      "test-agent-output-missing",
		Action:  "agent.output.get",
		Payload: socketAgentOutputPayload{Name: "nonexistent"},
	})

	var resp wsErrorEnvelope
	readWSJSON(t, conn, &resp)
	if resp.Type != "error" {
		t.Fatalf("response type = %q, want error", resp.Type)
	}
	if resp.Code != "not_found" {
		t.Fatalf("response code = %q, want not_found", resp.Code)
	}
}

func TestAgentOutputCityScoped(t *testing.T) {
	state := newFakeState(t)
	state.cfg.Agents = append(state.cfg.Agents, config.Agent{Name: "mayor"})

	searchBase := t.TempDir()
	writeSessionJSONL(t, searchBase, state.cityPath,
		`{"uuid":"1","parentUuid":"","type":"user","message":"{\"role\":\"user\",\"content\":\"plan\"}","timestamp":"2025-01-01T00:00:00Z"}`,
	)

	srv := newServerWithSearchPaths(state, searchBase)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	writeWSJSON(t, conn, wsRequestEnvelope{
		Type:    "request",
		ID:      "test-agent-output-city",
		Action:  "agent.output.get",
		Payload: socketAgentOutputPayload{Name: "mayor", Tail: intPtr(0)},
	})

	var resp wsResponseEnvelope
	readWSJSON(t, conn, &resp)
	if resp.Type != "response" || resp.ID != "test-agent-output-city" {
		t.Fatalf("response = %#v, want correlated response", resp)
	}

	var body agentOutputResponse
	if err := json.Unmarshal(resp.Result, &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Agent != "mayor" {
		t.Errorf("Agent = %q, want %q", body.Agent, "mayor")
	}
	if len(body.Turns) != 1 {
		t.Fatalf("got %d turns, want 1", len(body.Turns))
	}
	if body.Turns[0].Text != "plan" {
		t.Errorf("text = %q, want %q", body.Turns[0].Text, "plan")
	}
}

func TestAgentOutputPagination(t *testing.T) {
	state := newFakeState(t)
	rigDir := t.TempDir()
	state.cfg.Rigs = []config.Rig{{Name: "myrig", Path: rigDir}}
	searchBase := t.TempDir()

	writeSessionJSONL(t, searchBase, rigDir,
		`{"uuid":"1","parentUuid":"","type":"user","message":"{\"role\":\"user\",\"content\":\"first\"}","timestamp":"2025-01-01T00:00:00Z"}`,
		`{"uuid":"2","parentUuid":"1","type":"assistant","message":"{\"role\":\"assistant\",\"content\":\"first reply\"}","timestamp":"2025-01-01T00:00:01Z"}`,
		`{"uuid":"3","parentUuid":"2","type":"system","subtype":"compact_boundary","message":"{\"role\":\"system\",\"content\":\"compacted 1\"}","timestamp":"2025-01-01T00:00:02Z"}`,
		`{"uuid":"4","parentUuid":"3","type":"user","message":"{\"role\":\"user\",\"content\":\"second\"}","timestamp":"2025-01-01T00:00:03Z"}`,
		`{"uuid":"5","parentUuid":"4","type":"assistant","message":"{\"role\":\"assistant\",\"content\":\"second reply\"}","timestamp":"2025-01-01T00:00:04Z"}`,
		`{"uuid":"6","parentUuid":"5","type":"system","subtype":"compact_boundary","message":"{\"role\":\"system\",\"content\":\"compacted 2\"}","timestamp":"2025-01-01T00:00:05Z"}`,
		`{"uuid":"7","parentUuid":"6","type":"user","message":"{\"role\":\"user\",\"content\":\"third\"}","timestamp":"2025-01-01T00:00:06Z"}`,
		`{"uuid":"8","parentUuid":"7","type":"assistant","message":"{\"role\":\"assistant\",\"content\":\"third reply\"}","timestamp":"2025-01-01T00:00:07Z"}`,
	)

	srv := newServerWithSearchPaths(state, searchBase)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	writeWSJSON(t, conn, wsRequestEnvelope{
		Type:    "request",
		ID:      "test-agent-output-pagination",
		Action:  "agent.output.get",
		Payload: socketAgentOutputPayload{Name: "myrig/worker", Tail: intPtr(1)},
	})

	var resp wsResponseEnvelope
	readWSJSON(t, conn, &resp)
	if resp.Type != "response" || resp.ID != "test-agent-output-pagination" {
		t.Fatalf("response = %#v, want correlated response", resp)
	}

	var body agentOutputResponse
	if err := json.Unmarshal(resp.Result, &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Turns) != 3 {
		t.Fatalf("got %d turns, want 3", len(body.Turns))
	}
	if body.Pagination == nil {
		t.Fatal("pagination is nil, expected non-nil")
	}
	if !body.Pagination.HasOlderMessages {
		t.Error("expected HasOlderMessages=true")
	}
}

func TestAgentOutputCorruptedSessionFile(t *testing.T) {
	state := newFakeState(t)
	rigDir := t.TempDir()
	state.cfg.Rigs = []config.Rig{{Name: "myrig", Path: rigDir}}

	searchBase := t.TempDir()
	writeSessionJSONL(t, searchBase, rigDir, `not valid json at all {{{`)

	srv := newServerWithSearchPaths(state, searchBase)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	writeWSJSON(t, conn, wsRequestEnvelope{
		Type:    "request",
		ID:      "test-agent-output-corrupt",
		Action:  "agent.output.get",
		Payload: socketAgentOutputPayload{Name: "myrig/worker"},
	})

	var resp wsResponseEnvelope
	readWSJSON(t, conn, &resp)
	if resp.Type != "response" || resp.ID != "test-agent-output-corrupt" {
		t.Fatalf("response = %#v, want correlated response", resp)
	}

	var body agentOutputResponse
	if err := json.Unmarshal(resp.Result, &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Format != "conversation" {
		t.Errorf("Format = %q, want %q", body.Format, "conversation")
	}
}

func TestAgentOutputStreamSubscription(t *testing.T) {
	state := newFakeState(t)
	rigDir := t.TempDir()
	state.cfg.Rigs = []config.Rig{{Name: "myrig", Path: rigDir}}

	searchBase := t.TempDir()
	writeSessionJSONL(t, searchBase, rigDir,
		`{"uuid":"1","parentUuid":"","type":"user","message":"{\"role\":\"user\",\"content\":\"hello\"}","timestamp":"2025-01-01T00:00:00Z"}`,
	)

	srv := newServerWithSearchPaths(state, searchBase)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	writeWSJSON(t, conn, wsRequestEnvelope{
		Type:    "request",
		ID:      "test-agent-output-stream",
		Action:  "subscription.start",
		Payload: AgentOutputStreamSubscriptionPayload{Kind: subscriptionKindAgentOutputStream, Target: "myrig/worker"},
	})

	var resp wsResponseEnvelope
	readWSJSON(t, conn, &resp)
	if resp.Type != "response" || resp.ID != "test-agent-output-stream" {
		t.Fatalf("subscription response = %#v, want correlated response", resp)
	}

	var evt AgentOutputStreamTurnEventEnvelope
	readWSJSON(t, conn, &evt)
	if evt.Type != "event" || evt.EventType != "turn" {
		t.Fatalf("event = %#v, want turn event", evt)
	}
	if evt.Payload.Agent != "myrig/worker" {
		t.Fatalf("agent = %q, want myrig/worker", evt.Payload.Agent)
	}
	if len(evt.Payload.Turns) == 0 || evt.Payload.Turns[0].Text != "hello" {
		t.Fatalf("turns = %+v, want hello transcript text", evt.Payload.Turns)
	}
}

func TestAgentOutputStreamSubscriptionHonorsAfterCursor(t *testing.T) {
	state := newFakeState(t)
	rigDir := t.TempDir()
	state.cfg.Rigs = []config.Rig{{Name: "myrig", Path: rigDir}}

	searchBase := t.TempDir()
	writeSessionJSONL(t, searchBase, rigDir,
		`{"uuid":"1","parentUuid":"","type":"user","message":"{\"role\":\"user\",\"content\":\"hello\"}","timestamp":"2025-01-01T00:00:00Z"}`,
		`{"uuid":"2","parentUuid":"1","type":"assistant","message":"{\"role\":\"assistant\",\"content\":\"world\"}","timestamp":"2025-01-01T00:00:01Z"}`,
	)

	srv := newServerWithSearchPaths(state, searchBase)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	writeWSJSON(t, conn, wsRequestEnvelope{
		Type:   "request",
		ID:     "test-agent-output-stream-after-cursor",
		Action: "subscription.start",
		Payload: AgentOutputStreamSubscriptionPayload{
			Kind:        subscriptionKindAgentOutputStream,
			Target:      "myrig/worker",
			AfterCursor: "1",
		},
	})

	var resp wsResponseEnvelope
	readWSJSON(t, conn, &resp)
	if resp.Type != "response" || resp.ID != "test-agent-output-stream-after-cursor" {
		t.Fatalf("subscription response = %#v, want correlated response", resp)
	}

	var evt AgentOutputStreamTurnEventEnvelope
	readWSJSON(t, conn, &evt)
	if evt.Type != "event" || evt.EventType != "turn" {
		t.Fatalf("event = %#v, want turn event", evt)
	}
	if evt.Cursor != "2" {
		t.Fatalf("cursor = %q, want 2", evt.Cursor)
	}
	if len(evt.Payload.Turns) > 0 && evt.Payload.Turns[0].Text == "hello" {
		t.Fatalf("turns = %+v, did not expect pre-cursor turn", evt.Payload.Turns)
	}
	if len(evt.Payload.Turns) == 0 || evt.Payload.Turns[0].Text != "world" {
		t.Fatalf("turns = %+v, want post-cursor transcript text", evt.Payload.Turns)
	}
}

func TestAgentOutputStreamNotFound(t *testing.T) {
	state := newFakeState(t)
	srv := New(state)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	writeWSJSON(t, conn, wsRequestEnvelope{
		Type:    "request",
		ID:      "test-agent-output-stream-missing",
		Action:  "subscription.start",
		Payload: AgentOutputStreamSubscriptionPayload{Kind: subscriptionKindAgentOutputStream, Target: "nonexistent"},
	})

	var resp wsErrorEnvelope
	readWSJSON(t, conn, &resp)
	if resp.Type != "error" {
		t.Fatalf("response type = %q, want error", resp.Type)
	}
	if resp.Code != "not_found" {
		t.Fatalf("response code = %q, want not_found", resp.Code)
	}
}
