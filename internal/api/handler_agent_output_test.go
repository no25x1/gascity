package api

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gastownhall/gascity/internal/runtime"
	"github.com/gastownhall/gascity/internal/session"
)

// createTranscriptSession creates a session with a real workDir (temp dir)
// and a SessionIDFlag so session_key-based lookup works. Returns the info
// and a searchBase suitable for writeNamedSessionJSONL.
func createTranscriptSession(t *testing.T, fs *fakeState, title string) (session.Info, string) {
	t.Helper()
	workDir := t.TempDir()
	mgr := session.NewManager(fs.cityBeadStore, fs.sp)
	resume := session.ProviderResume{
		ResumeFlag:    "--resume",
		ResumeStyle:   "flag",
		SessionIDFlag: "--session-id",
	}
	info, err := mgr.Create(context.Background(), "default", title, "echo test", workDir, "claude", nil, resume, runtime.Config{})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	searchBase := t.TempDir()
	return info, searchBase
}

func TestAgentOutputConversation(t *testing.T) {
	fs := newSessionFakeState(t)
	info, searchBase := createTranscriptSession(t, fs, "Conversation")
	srv := New(fs)
	srv.sessionLogSearchPaths = []string{searchBase}

	writeNamedSessionJSONL(t, searchBase, info.WorkDir, info.SessionKey+".jsonl",
		`{"uuid":"1","parentUuid":"","type":"user","message":"{\"role\":\"user\",\"content\":\"hello\"}","timestamp":"2025-01-01T00:00:00Z"}`,
		`{"uuid":"2","parentUuid":"1","type":"assistant","message":"{\"role\":\"assistant\",\"content\":\"world\"}","timestamp":"2025-01-01T00:00:01Z"}`,
	)
	if err := session.NewManager(fs.cityBeadStore, fs.sp).Close(info.ID); err != nil {
		t.Fatalf("Close: %v", err)
	}

	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	writeWSJSON(t, conn, wsRequestEnvelope{
		Type:   "request",
		ID:     "test-1",
		Action: "session.transcript",
		Payload: map[string]any{
			"id":   info.ID,
			"tail": 0,
		},
	})

	var resp wsResponseEnvelope
	readWSJSON(t, conn, &resp)
	if resp.Type != "response" || resp.ID != "test-1" {
		t.Fatalf("response = %#v, want correlated response", resp)
	}

	var body sessionTranscriptResponse
	if err := json.Unmarshal(resp.Result, &body); err != nil {
		t.Fatalf("unmarshal result: %v", err)
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
	fs := newSessionFakeState(t)
	// Create a session with a specific workDir to verify it's resolved correctly.
	workDir := t.TempDir()
	mgr := session.NewManager(fs.cityBeadStore, fs.sp)
	resume := session.ProviderResume{
		ResumeFlag:    "--resume",
		ResumeStyle:   "flag",
		SessionIDFlag: "--session-id",
	}
	info, err := mgr.Create(context.Background(), "default", "WorkDir Test", "echo test", workDir, "claude", nil, resume, runtime.Config{})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	searchBase := t.TempDir()
	writeNamedSessionJSONL(t, searchBase, info.WorkDir, info.SessionKey+".jsonl",
		`{"uuid":"1","parentUuid":"","type":"user","message":"{\"role\":\"user\",\"content\":\"hello\"}","timestamp":"2025-01-01T00:00:00Z"}`,
		`{"uuid":"2","parentUuid":"1","type":"assistant","message":"{\"role\":\"assistant\",\"content\":\"from workdir\"}","timestamp":"2025-01-01T00:00:01Z"}`,
	)
	if err := mgr.Close(info.ID); err != nil {
		t.Fatalf("Close: %v", err)
	}

	srv := New(fs)
	srv.sessionLogSearchPaths = []string{searchBase}
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	writeWSJSON(t, conn, wsRequestEnvelope{
		Type:   "request",
		ID:     "test-workdir",
		Action: "session.transcript",
		Payload: map[string]any{
			"id":   info.ID,
			"tail": 0,
		},
	})

	var resp wsResponseEnvelope
	readWSJSON(t, conn, &resp)
	if resp.Type != "response" || resp.ID != "test-workdir" {
		t.Fatalf("response = %#v, want correlated response", resp)
	}

	var body sessionTranscriptResponse
	if err := json.Unmarshal(resp.Result, &body); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if body.Format != "conversation" {
		t.Fatalf("Format = %q, want conversation", body.Format)
	}
	if len(body.Turns) != 2 || body.Turns[1].Text != "from workdir" {
		t.Fatalf("Turns = %+v, want configured work_dir session log", body.Turns)
	}
}

func TestAgentOutputNotFound(t *testing.T) {
	fs := newSessionFakeState(t)
	srv := New(fs)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	writeWSJSON(t, conn, wsRequestEnvelope{
		Type:   "request",
		ID:     "test-not-found",
		Action: "session.transcript",
		Payload: map[string]any{
			"id": "nonexistent-session-id",
		},
	})

	var resp wsErrorEnvelope
	readWSJSON(t, conn, &resp)
	if resp.Type != "error" {
		t.Fatalf("response type = %q, want error", resp.Type)
	}
	if resp.ID != "test-not-found" {
		t.Fatalf("response id = %q, want test-not-found", resp.ID)
	}
}

func TestAgentOutputCityScoped(t *testing.T) {
	fs := newSessionFakeState(t)
	info, searchBase := createTranscriptSession(t, fs, "CityScoped")

	writeNamedSessionJSONL(t, searchBase, info.WorkDir, info.SessionKey+".jsonl",
		`{"uuid":"1","parentUuid":"","type":"user","message":"{\"role\":\"user\",\"content\":\"plan\"}","timestamp":"2025-01-01T00:00:00Z"}`,
	)
	if err := session.NewManager(fs.cityBeadStore, fs.sp).Close(info.ID); err != nil {
		t.Fatalf("Close: %v", err)
	}

	srv := New(fs)
	srv.sessionLogSearchPaths = []string{searchBase}
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	writeWSJSON(t, conn, wsRequestEnvelope{
		Type:   "request",
		ID:     "test-city-scoped",
		Action: "session.transcript",
		Payload: map[string]any{
			"id":   info.ID,
			"tail": 0,
		},
	})

	var resp wsResponseEnvelope
	readWSJSON(t, conn, &resp)
	if resp.Type != "response" || resp.ID != "test-city-scoped" {
		t.Fatalf("response = %#v, want correlated response", resp)
	}

	var body sessionTranscriptResponse
	if err := json.Unmarshal(resp.Result, &body); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if body.ID != info.ID {
		t.Errorf("ID = %q, want %q", body.ID, info.ID)
	}
	if len(body.Turns) != 1 {
		t.Fatalf("got %d turns, want 1", len(body.Turns))
	}
	if body.Turns[0].Text != "plan" {
		t.Errorf("text = %q, want %q", body.Turns[0].Text, "plan")
	}
}

func TestAgentOutputPagination(t *testing.T) {
	fs := newSessionFakeState(t)
	info, searchBase := createTranscriptSession(t, fs, "Pagination")

	var lines []string
	lines = append(lines, `{"uuid":"1","parentUuid":"","type":"user","message":"{\"role\":\"user\",\"content\":\"first\"}","timestamp":"2025-01-01T00:00:00Z"}`)
	lines = append(lines, `{"uuid":"2","parentUuid":"1","type":"assistant","message":"{\"role\":\"assistant\",\"content\":\"first reply\"}","timestamp":"2025-01-01T00:00:01Z"}`)
	lines = append(lines, `{"uuid":"3","parentUuid":"2","type":"system","subtype":"compact_boundary","message":"{\"role\":\"system\",\"content\":\"compacted 1\"}","timestamp":"2025-01-01T00:00:02Z"}`)
	lines = append(lines, `{"uuid":"4","parentUuid":"3","type":"user","message":"{\"role\":\"user\",\"content\":\"second\"}","timestamp":"2025-01-01T00:00:03Z"}`)
	lines = append(lines, `{"uuid":"5","parentUuid":"4","type":"assistant","message":"{\"role\":\"assistant\",\"content\":\"second reply\"}","timestamp":"2025-01-01T00:00:04Z"}`)
	lines = append(lines, `{"uuid":"6","parentUuid":"5","type":"system","subtype":"compact_boundary","message":"{\"role\":\"system\",\"content\":\"compacted 2\"}","timestamp":"2025-01-01T00:00:05Z"}`)
	lines = append(lines, `{"uuid":"7","parentUuid":"6","type":"user","message":"{\"role\":\"user\",\"content\":\"third\"}","timestamp":"2025-01-01T00:00:06Z"}`)
	lines = append(lines, `{"uuid":"8","parentUuid":"7","type":"assistant","message":"{\"role\":\"assistant\",\"content\":\"third reply\"}","timestamp":"2025-01-01T00:00:07Z"}`)

	writeNamedSessionJSONL(t, searchBase, info.WorkDir, info.SessionKey+".jsonl", lines...)
	if err := session.NewManager(fs.cityBeadStore, fs.sp).Close(info.ID); err != nil {
		t.Fatalf("Close: %v", err)
	}

	srv := New(fs)
	srv.sessionLogSearchPaths = []string{searchBase}
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	// tail=1 should return messages from the last compact boundary onward.
	writeWSJSON(t, conn, wsRequestEnvelope{
		Type:   "request",
		ID:     "test-pagination",
		Action: "session.transcript",
		Payload: map[string]any{
			"id":    info.ID,
			"turns": 1,
		},
	})

	var resp wsResponseEnvelope
	readWSJSON(t, conn, &resp)
	if resp.Type != "response" || resp.ID != "test-pagination" {
		t.Fatalf("response = %#v, want correlated response", resp)
	}

	var body sessionTranscriptResponse
	if err := json.Unmarshal(resp.Result, &body); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	// Boundary text + 2 turns after = 3 turns (system entry "compacted 2" + user + assistant).
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
	fs := newSessionFakeState(t)
	info, searchBase := createTranscriptSession(t, fs, "Corrupted")

	// Write corrupt JSONL content.
	writeNamedSessionJSONL(t, searchBase, info.WorkDir, info.SessionKey+".jsonl",
		`not valid json at all {{{`,
	)
	if err := session.NewManager(fs.cityBeadStore, fs.sp).Close(info.ID); err != nil {
		t.Fatalf("Close: %v", err)
	}

	srv := New(fs)
	srv.sessionLogSearchPaths = []string{searchBase}
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	writeWSJSON(t, conn, wsRequestEnvelope{
		Type:   "request",
		ID:     "test-corrupt",
		Action: "session.transcript",
		Payload: map[string]any{
			"id": info.ID,
		},
	})

	var resp wsResponseEnvelope
	readWSJSON(t, conn, &resp)
	if resp.Type != "response" || resp.ID != "test-corrupt" {
		t.Fatalf("response = %#v, want correlated response", resp)
	}

	var body sessionTranscriptResponse
	if err := json.Unmarshal(resp.Result, &body); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if body.Format != "conversation" {
		t.Errorf("Format = %q, want %q", body.Format, "conversation")
	}
}
