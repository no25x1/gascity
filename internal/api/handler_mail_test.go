package api

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gastownhall/gascity/internal/events"
	"github.com/gastownhall/gascity/internal/mail"
)

func TestMailLifecycle(t *testing.T) {
	state := newFakeState(t)
	srv := New(state)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	// Send a message. Bare "worker" resolves to "myrig/worker" (the qualified name).
	writeWSJSON(t, conn, wsRequestEnvelope{
		Type:   "request",
		ID:     "send-1",
		Action: "mail.send",
		Payload: map[string]any{
			"from":    "mayor",
			"to":      "worker",
			"subject": "Review needed",
			"body":    "Please check gc-456",
		},
	})
	var sendResp wsResponseEnvelope
	readWSJSON(t, conn, &sendResp)
	if sendResp.Type != "response" || sendResp.ID != "send-1" {
		t.Fatalf("send response = %#v, want correlated response", sendResp)
	}

	var sent mail.Message
	json.Unmarshal(sendResp.Result, &sent) //nolint:errcheck
	if sent.Subject != "Review needed" {
		t.Errorf("Subject = %q, want %q", sent.Subject, "Review needed")
	}
	if sent.To != "myrig/worker" {
		t.Errorf("To = %q, want %q (bare name should resolve to qualified)", sent.To, "myrig/worker")
	}

	// Check inbox using the resolved qualified name.
	writeWSJSON(t, conn, wsRequestEnvelope{
		Type:   "request",
		ID:     "list-1",
		Action: "mail.list",
		Payload: map[string]any{
			"agent": "myrig/worker",
		},
	})
	var listResp wsResponseEnvelope
	readWSJSON(t, conn, &listResp)

	var inbox struct {
		Items []mail.Message `json:"items"`
		Total int            `json:"total"`
	}
	json.Unmarshal(listResp.Result, &inbox) //nolint:errcheck
	if inbox.Total != 1 {
		t.Fatalf("inbox Total = %d, want 1", inbox.Total)
	}

	// Mark read.
	writeWSJSON(t, conn, wsRequestEnvelope{
		Type:   "request",
		ID:     "read-1",
		Action: "mail.read",
		Payload: map[string]any{
			"id": sent.ID,
		},
	})
	var readResp wsResponseEnvelope
	readWSJSON(t, conn, &readResp)
	if readResp.Type != "response" {
		t.Fatalf("read response type = %q, want response", readResp.Type)
	}

	// Inbox should be empty now (only unread).
	writeWSJSON(t, conn, wsRequestEnvelope{
		Type:   "request",
		ID:     "list-2",
		Action: "mail.list",
		Payload: map[string]any{
			"agent": "myrig/worker",
		},
	})
	var listResp2 wsResponseEnvelope
	readWSJSON(t, conn, &listResp2)

	json.Unmarshal(listResp2.Result, &inbox) //nolint:errcheck
	if inbox.Total != 0 {
		t.Errorf("inbox after read: Total = %d, want 0", inbox.Total)
	}

	// Get still works.
	writeWSJSON(t, conn, wsRequestEnvelope{
		Type:   "request",
		ID:     "get-1",
		Action: "mail.get",
		Payload: map[string]any{
			"id": sent.ID,
		},
	})
	var getResp wsResponseEnvelope
	readWSJSON(t, conn, &getResp)
	if getResp.Type != "response" {
		t.Fatalf("get response type = %q, want response", getResp.Type)
	}

	// Archive.
	writeWSJSON(t, conn, wsRequestEnvelope{
		Type:   "request",
		ID:     "archive-1",
		Action: "mail.archive",
		Payload: map[string]any{
			"id": sent.ID,
		},
	})
	var archiveResp wsResponseEnvelope
	readWSJSON(t, conn, &archiveResp)
	if archiveResp.Type != "response" {
		t.Fatalf("archive response type = %q, want response", archiveResp.Type)
	}
}

func TestMailSendValidation(t *testing.T) {
	state := newFakeState(t)
	srv := New(state)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	// Missing required fields.
	writeWSJSON(t, conn, wsRequestEnvelope{
		Type:   "request",
		ID:     "send-bad",
		Action: "mail.send",
		Payload: map[string]any{
			"from": "mayor",
		},
	})
	var errResp wsErrorEnvelope
	readWSJSON(t, conn, &errResp)
	if errResp.Type != "error" {
		t.Fatalf("expected error response, got type = %q", errResp.Type)
	}
	if len(errResp.Details) != 2 {
		t.Errorf("Details count = %d, want 2", len(errResp.Details))
	}
}

func TestMailCount(t *testing.T) {
	state := newFakeState(t)
	mp := state.cityMailProv
	mp.Send("a", "b", "msg1", "body1") //nolint:errcheck
	mp.Send("a", "b", "msg2", "body2") //nolint:errcheck
	srv := New(state)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	writeWSJSON(t, conn, wsRequestEnvelope{
		Type:   "request",
		ID:     "count-1",
		Action: "mail.count",
		Payload: map[string]any{
			"agent": "b",
		},
	})
	var resp wsResponseEnvelope
	readWSJSON(t, conn, &resp)

	var counts map[string]int
	json.Unmarshal(resp.Result, &counts) //nolint:errcheck
	if counts["unread"] != 2 {
		t.Errorf("unread = %d, want 2", counts["unread"])
	}
}

func TestMailDelete(t *testing.T) {
	state := newFakeState(t)
	mp := state.cityMailProv
	msg, _ := mp.Send("mayor", "worker", "To delete", "content")
	srv := New(state)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	writeWSJSON(t, conn, wsRequestEnvelope{
		Type:   "request",
		ID:     "delete-1",
		Action: "mail.delete",
		Payload: map[string]any{
			"id": msg.ID,
		},
	})
	var resp wsResponseEnvelope
	readWSJSON(t, conn, &resp)
	if resp.Type != "response" {
		t.Fatalf("delete response type = %q, want response", resp.Type)
	}

	// After delete (soft delete/archive), message should no longer appear in inbox.
	writeWSJSON(t, conn, wsRequestEnvelope{
		Type:   "request",
		ID:     "list-after-delete",
		Action: "mail.list",
		Payload: map[string]any{
			"agent": "worker",
		},
	})
	var listResp wsResponseEnvelope
	readWSJSON(t, conn, &listResp)

	var inbox struct {
		Items []mail.Message `json:"items"`
		Total int            `json:"total"`
	}
	json.Unmarshal(listResp.Result, &inbox) //nolint:errcheck
	if inbox.Total != 0 {
		t.Errorf("inbox after delete: Total = %d, want 0", inbox.Total)
	}
}

func TestMailDeleteNotFound(t *testing.T) {
	state := newFakeState(t)
	srv := New(state)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	writeWSJSON(t, conn, wsRequestEnvelope{
		Type:   "request",
		ID:     "delete-nf",
		Action: "mail.delete",
		Payload: map[string]any{
			"id": "nonexistent",
		},
	})
	var errResp wsErrorEnvelope
	readWSJSON(t, conn, &errResp)
	if errResp.Type != "error" {
		t.Fatalf("expected error response, got type = %q", errResp.Type)
	}
	if errResp.Code != "not_found" {
		t.Errorf("code = %q, want not_found", errResp.Code)
	}
}

func TestMailListStatusAll(t *testing.T) {
	state := newFakeState(t)
	mp := state.cityMailProv

	// Send two messages to worker.
	mp.Send("mayor", "worker", "First", "body1")  //nolint:errcheck
	mp.Send("mayor", "worker", "Second", "body2") //nolint:errcheck

	srv := New(state)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	// Default (no status) returns only unread — both should appear.
	writeWSJSON(t, conn, wsRequestEnvelope{
		Type:   "request",
		ID:     "list-unread",
		Action: "mail.list",
		Payload: map[string]any{
			"agent": "worker",
		},
	})
	var resp wsResponseEnvelope
	readWSJSON(t, conn, &resp)

	var list struct {
		Items []mail.Message `json:"items"`
		Total int            `json:"total"`
	}
	json.Unmarshal(resp.Result, &list) //nolint:errcheck
	if list.Total != 2 {
		t.Fatalf("unread Total = %d, want 2", list.Total)
	}

	// Mark the first message as read.
	mp.MarkRead(list.Items[0].ID) //nolint:errcheck

	// Default (unread) should now return 1.
	writeWSJSON(t, conn, wsRequestEnvelope{
		Type:   "request",
		ID:     "list-unread2",
		Action: "mail.list",
		Payload: map[string]any{
			"agent":  "worker",
			"status": "unread",
		},
	})
	var resp2 wsResponseEnvelope
	readWSJSON(t, conn, &resp2)

	json.Unmarshal(resp2.Result, &list) //nolint:errcheck
	if list.Total != 1 {
		t.Fatalf("unread after mark-read Total = %d, want 1", list.Total)
	}

	// status=all should return both (read + unread).
	writeWSJSON(t, conn, wsRequestEnvelope{
		Type:   "request",
		ID:     "list-all",
		Action: "mail.list",
		Payload: map[string]any{
			"agent":  "worker",
			"status": "all",
		},
	})
	var resp3 wsResponseEnvelope
	readWSJSON(t, conn, &resp3)
	if resp3.Type != "response" {
		t.Fatalf("status=all returned type = %q, want response", resp3.Type)
	}
	json.Unmarshal(resp3.Result, &list) //nolint:errcheck
	if list.Total != 2 {
		t.Errorf("status=all Total = %d, want 2", list.Total)
	}
}

func TestMailListStatusAllAcrossRigs(t *testing.T) {
	state := newFakeState(t)
	mp := state.cityMailProv

	mp.Send("mayor", "worker", "Msg1", "body1") //nolint:errcheck
	msg2, _ := mp.Send("mayor", "worker", "Msg2", "body2")
	mp.MarkRead(msg2.ID) //nolint:errcheck

	srv := New(state)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	// status=all without rig param aggregates across all rigs.
	writeWSJSON(t, conn, wsRequestEnvelope{
		Type:   "request",
		ID:     "list-all-rigs",
		Action: "mail.list",
		Payload: map[string]any{
			"agent":  "worker",
			"status": "all",
		},
	})
	var resp wsResponseEnvelope
	readWSJSON(t, conn, &resp)
	if resp.Type != "response" {
		t.Fatalf("status=all returned type = %q, want response", resp.Type)
	}

	var list struct {
		Items []mail.Message `json:"items"`
		Total int            `json:"total"`
	}
	json.Unmarshal(resp.Result, &list) //nolint:errcheck
	if list.Total != 2 {
		t.Errorf("status=all across rigs Total = %d, want 2", list.Total)
	}
}

func TestMailListStatusInvalid(t *testing.T) {
	state := newFakeState(t)
	srv := New(state)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	writeWSJSON(t, conn, wsRequestEnvelope{
		Type:   "request",
		ID:     "list-bogus",
		Action: "mail.list",
		Payload: map[string]any{
			"status": "bogus",
		},
	})
	var errResp wsErrorEnvelope
	readWSJSON(t, conn, &errResp)
	if errResp.Type != "error" {
		t.Fatalf("expected error response, got type = %q", errResp.Type)
	}
}

func TestMailReply(t *testing.T) {
	state := newFakeState(t)
	mp := state.cityMailProv
	msg, _ := mp.Send("mayor", "worker", "Initial", "content")
	srv := New(state)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	writeWSJSON(t, conn, wsRequestEnvelope{
		Type:   "request",
		ID:     "reply-1",
		Action: "mail.reply",
		Payload: map[string]any{
			"id":      msg.ID,
			"from":    "worker",
			"subject": "Re: Initial",
			"body":    "Done!",
		},
	})
	var resp wsResponseEnvelope
	readWSJSON(t, conn, &resp)
	if resp.Type != "response" {
		t.Fatalf("reply response type = %q, want response", resp.Type)
	}

	var reply mail.Message
	json.Unmarshal(resp.Result, &reply) //nolint:errcheck
	if reply.ThreadID == "" {
		t.Error("reply has no ThreadID")
	}
}

func TestMailListIncludesRig(t *testing.T) {
	state := newFakeState(t)
	mp := state.cityMailProv
	mp.Send("alice", "bob", "Hi", "hello") //nolint:errcheck
	srv := New(state)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	// List without rig filter — aggregation path.
	writeWSJSON(t, conn, wsRequestEnvelope{
		Type:   "request",
		ID:     "list-norigs",
		Action: "mail.list",
		Payload: map[string]any{
			"status": "all",
		},
	})
	var resp wsResponseEnvelope
	readWSJSON(t, conn, &resp)

	var list struct {
		Items []mail.Message `json:"items"`
	}
	json.Unmarshal(resp.Result, &list) //nolint:errcheck
	if len(list.Items) == 0 {
		t.Fatal("expected at least 1 message")
	}
	if list.Items[0].Rig != "test-city" {
		t.Errorf("Items[0].Rig = %q, want %q", list.Items[0].Rig, "test-city")
	}

	// List with rig filter — single-rig path.
	writeWSJSON(t, conn, wsRequestEnvelope{
		Type:   "request",
		ID:     "list-rig",
		Action: "mail.list",
		Payload: map[string]any{
			"rig":    "test-city",
			"status": "all",
		},
	})
	var resp2 wsResponseEnvelope
	readWSJSON(t, conn, &resp2)

	json.Unmarshal(resp2.Result, &list) //nolint:errcheck
	if len(list.Items) == 0 {
		t.Fatal("expected at least 1 message")
	}
	if list.Items[0].Rig != "test-city" {
		t.Errorf("Items[0].Rig = %q, want %q (single-rig path)", list.Items[0].Rig, "test-city")
	}
}

func TestMailThreadIncludesRig(t *testing.T) {
	state := newFakeState(t)
	mp := state.cityMailProv
	msg, _ := mp.Send("alice", "bob", "Thread test", "body")

	// Reply to create a thread.
	mp.Reply(msg.ID, "bob", "Re: Thread test", "reply body") //nolint:errcheck

	srv := New(state)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	writeWSJSON(t, conn, wsRequestEnvelope{
		Type:   "request",
		ID:     "thread-1",
		Action: "mail.thread",
		Payload: map[string]any{
			"id":  msg.ThreadID,
			"rig": "test-city",
		},
	})
	var resp wsResponseEnvelope
	readWSJSON(t, conn, &resp)

	var list struct {
		Items []mail.Message `json:"items"`
	}
	json.Unmarshal(resp.Result, &list) //nolint:errcheck
	if len(list.Items) == 0 {
		t.Fatal("expected thread messages")
	}
	for i, m := range list.Items {
		if m.Rig != "test-city" {
			t.Errorf("Items[%d].Rig = %q, want %q", i, m.Rig, "test-city")
		}
	}
}

func TestMailSendIdempotentReplayIncludesRig(t *testing.T) {
	state := newFakeState(t)
	srv := New(state)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	// First send.
	writeWSJSON(t, conn, wsRequestEnvelope{
		Type:   "request",
		ID:     "send-idem-1",
		Action: "mail.send",
		Payload: map[string]any{
			"rig":     "test-city",
			"from":    "alice",
			"to":      "worker",
			"subject": "Hi",
			"body":    "hello",
		},
	})
	var resp1 wsResponseEnvelope
	readWSJSON(t, conn, &resp1)
	if resp1.Type != "response" {
		t.Fatalf("first send type = %q, want response", resp1.Type)
	}

	var msg mail.Message
	json.Unmarshal(resp1.Result, &msg) //nolint:errcheck
	if msg.Rig != "test-city" {
		t.Fatalf("send Rig = %q, want %q", msg.Rig, "test-city")
	}
}

func TestMailGetWithoutRigHintIncludesResolvedRig(t *testing.T) {
	state := newFakeState(t)
	mp := state.cityMailProv
	msg, _ := mp.Send("alice", "bob", "Hi", "hello")
	srv := New(state)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	writeWSJSON(t, conn, wsRequestEnvelope{
		Type:   "request",
		ID:     "get-norig",
		Action: "mail.get",
		Payload: map[string]any{
			"id": msg.ID,
		},
	})
	var resp wsResponseEnvelope
	readWSJSON(t, conn, &resp)
	if resp.Type != "response" {
		t.Fatalf("get response type = %q, want response", resp.Type)
	}

	var got mail.Message
	json.Unmarshal(resp.Result, &got) //nolint:errcheck
	if got.Rig != "test-city" {
		t.Fatalf("get Rig = %q, want %q", got.Rig, "test-city")
	}
}

func TestMailMutationEventsUseResolvedRigWithoutHint(t *testing.T) {
	state := newFakeState(t)
	ep := state.eventProv.(*events.Fake)
	mp := state.cityMailProv
	msg, _ := mp.Send("alice", "bob", "Hi", "hello")
	srv := New(state)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	writeWSJSON(t, conn, wsRequestEnvelope{
		Type:   "request",
		ID:     "read-rig",
		Action: "mail.read",
		Payload: map[string]any{
			"id": msg.ID,
		},
	})
	var resp wsResponseEnvelope
	readWSJSON(t, conn, &resp)
	if resp.Type != "response" {
		t.Fatalf("read response type = %q, want response", resp.Type)
	}

	if len(ep.Events) == 0 {
		t.Fatal("expected read event")
	}

	var payload struct {
		Rig string `json:"rig"`
	}
	if err := json.Unmarshal(ep.Events[len(ep.Events)-1].Payload, &payload); err != nil {
		t.Fatalf("unmarshal read payload: %v", err)
	}
	if payload.Rig != "test-city" {
		t.Fatalf("read event rig = %q, want %q", payload.Rig, "test-city")
	}
}

func TestMailReplyWithoutRigHintUsesResolvedRig(t *testing.T) {
	state := newFakeState(t)
	ep := state.eventProv.(*events.Fake)
	mp := state.cityMailProv
	msg, _ := mp.Send("alice", "bob", "Hi", "hello")
	srv := New(state)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	writeWSJSON(t, conn, wsRequestEnvelope{
		Type:   "request",
		ID:     "reply-rig",
		Action: "mail.reply",
		Payload: map[string]any{
			"id":      msg.ID,
			"from":    "bob",
			"subject": "Re: Hi",
			"body":    "reply",
		},
	})
	var resp wsResponseEnvelope
	readWSJSON(t, conn, &resp)
	if resp.Type != "response" {
		t.Fatalf("reply response type = %q, want response", resp.Type)
	}

	var reply mail.Message
	json.Unmarshal(resp.Result, &reply) //nolint:errcheck
	if reply.Rig != "test-city" {
		t.Fatalf("reply Rig = %q, want %q", reply.Rig, "test-city")
	}

	if len(ep.Events) == 0 {
		t.Fatal("expected reply event")
	}

	var evtPayload struct {
		Rig     string       `json:"rig"`
		Message mail.Message `json:"message"`
	}
	if err := json.Unmarshal(ep.Events[len(ep.Events)-1].Payload, &evtPayload); err != nil {
		t.Fatalf("unmarshal reply payload: %v", err)
	}
	if evtPayload.Rig != "test-city" {
		t.Fatalf("reply event rig = %q, want %q", evtPayload.Rig, "test-city")
	}
	if evtPayload.Message.Rig != "test-city" {
		t.Fatalf("reply event message rig = %q, want %q", evtPayload.Message.Rig, "test-city")
	}
}
