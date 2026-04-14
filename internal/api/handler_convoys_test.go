package api

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gastownhall/gascity/internal/beads"
)

func TestConvoyCreateAndGet(t *testing.T) {
	state := newFakeMutatorState(t)
	srv := New(state)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()
	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	store := state.stores["myrig"]
	item, err := store.Create(beads.Bead{Title: "task-1"})
	if err != nil {
		t.Fatalf("create item: %v", err)
	}

	writeWSJSON(t, conn, wsRequestEnvelope{Type: "request", ID: "cc-1", Action: "convoy.create", Payload: map[string]any{"rig": "myrig", "title": "test convoy", "items": []string{item.ID}}})
	var resp wsResponseEnvelope
	readWSJSON(t, conn, &resp)
	if resp.Type != "response" {
		t.Fatalf("create: type = %q, want response; result = %s", resp.Type, resp.Result)
	}
	var convoy beads.Bead
	json.Unmarshal(resp.Result, &convoy)
	if convoy.Type != "convoy" {
		t.Fatalf("type = %q, want convoy", convoy.Type)
	}

	writeWSJSON(t, conn, wsRequestEnvelope{Type: "request", ID: "cg-1", Action: "convoy.get", Payload: map[string]any{"id": convoy.ID}})
	readWSJSON(t, conn, &resp)
	if resp.Type != "response" {
		t.Fatalf("get: type = %q, want response", resp.Type)
	}
}

func TestConvoyCreateInvalidItem(t *testing.T) {
	state := newFakeMutatorState(t)
	srv := New(state)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()
	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	writeWSJSON(t, conn, wsRequestEnvelope{Type: "request", ID: "cc-bad", Action: "convoy.create", Payload: map[string]any{"rig": "myrig", "title": "test", "items": []string{"nonexistent"}}})
	var errResp wsErrorEnvelope
	readWSJSON(t, conn, &errResp)
	if errResp.Type != "error" {
		t.Fatalf("type = %q, want error", errResp.Type)
	}
}

func TestConvoyAddItems(t *testing.T) {
	state := newFakeMutatorState(t)
	srv := New(state)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()
	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	store := state.stores["myrig"]
	convoy, _ := store.Create(beads.Bead{Title: "convoy", Type: "convoy"})
	item, _ := store.Create(beads.Bead{Title: "task"})

	writeWSJSON(t, conn, wsRequestEnvelope{Type: "request", ID: "ca-1", Action: "convoy.add", Payload: map[string]any{"id": convoy.ID, "items": []string{item.ID}}})
	var resp wsResponseEnvelope
	readWSJSON(t, conn, &resp)
	if resp.Type != "response" {
		t.Fatalf("add: type = %q, want response", resp.Type)
	}
}

func TestConvoyClose(t *testing.T) {
	state := newFakeMutatorState(t)
	srv := New(state)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()
	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	store := state.stores["myrig"]
	convoy, _ := store.Create(beads.Bead{Title: "convoy", Type: "convoy"})

	writeWSJSON(t, conn, wsRequestEnvelope{Type: "request", ID: "ccl-1", Action: "convoy.close", Payload: map[string]any{"id": convoy.ID}})
	var resp wsResponseEnvelope
	readWSJSON(t, conn, &resp)
	if resp.Type != "response" {
		t.Fatalf("close: type = %q, want response", resp.Type)
	}
}

func TestConvoyNotFound(t *testing.T) {
	state := newFakeMutatorState(t)
	srv := New(state)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()
	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	writeWSJSON(t, conn, wsRequestEnvelope{Type: "request", ID: "cnf", Action: "convoy.get", Payload: map[string]any{"id": "nonexistent"}})
	var errResp wsErrorEnvelope
	readWSJSON(t, conn, &errResp)
	if errResp.Type != "error" {
		t.Fatalf("type = %q, want error", errResp.Type)
	}
}

func TestConvoyRemoveItems(t *testing.T) {
	state := newFakeMutatorState(t)
	srv := New(state)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()
	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	store := state.stores["myrig"]
	convoy, _ := store.Create(beads.Bead{Title: "convoy", Type: "convoy"})
	item, _ := store.Create(beads.Bead{Title: "task"})
	pid := convoy.ID
	store.Update(item.ID, beads.UpdateOpts{ParentID: &pid}) //nolint:errcheck

	writeWSJSON(t, conn, wsRequestEnvelope{Type: "request", ID: "cr-1", Action: "convoy.remove", Payload: map[string]any{"id": convoy.ID, "items": []string{item.ID}}})
	var resp wsResponseEnvelope
	readWSJSON(t, conn, &resp)
	if resp.Type != "response" {
		t.Fatalf("remove: type = %q, want response", resp.Type)
	}

	got, _ := store.Get(item.ID)
	if got.ParentID != "" {
		t.Errorf("ParentID = %q, want empty", got.ParentID)
	}
}

func TestConvoyRemoveNonMember(t *testing.T) {
	state := newFakeMutatorState(t)
	srv := New(state)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()
	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	store := state.stores["myrig"]
	convoy, _ := store.Create(beads.Bead{Title: "convoy", Type: "convoy"})
	item, _ := store.Create(beads.Bead{Title: "unrelated"})

	writeWSJSON(t, conn, wsRequestEnvelope{Type: "request", ID: "cr-nm", Action: "convoy.remove", Payload: map[string]any{"id": convoy.ID, "items": []string{item.ID}}})
	var errResp wsErrorEnvelope
	readWSJSON(t, conn, &errResp)
	if errResp.Type != "error" {
		t.Fatalf("type = %q, want error", errResp.Type)
	}
}

func TestConvoyCheck(t *testing.T) {
	state := newFakeMutatorState(t)
	srv := New(state)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()
	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	store := state.stores["myrig"]
	convoy, _ := store.Create(beads.Bead{Title: "convoy", Type: "convoy"})
	item1, _ := store.Create(beads.Bead{Title: "task1"})
	item2, _ := store.Create(beads.Bead{Title: "task2"})
	pid := convoy.ID
	store.Update(item1.ID, beads.UpdateOpts{ParentID: &pid}) //nolint:errcheck
	store.Update(item2.ID, beads.UpdateOpts{ParentID: &pid}) //nolint:errcheck
	store.Close(item1.ID)                                    //nolint:errcheck

	writeWSJSON(t, conn, wsRequestEnvelope{Type: "request", ID: "chk-1", Action: "convoy.check", Payload: map[string]any{"id": convoy.ID}})
	var resp wsResponseEnvelope
	readWSJSON(t, conn, &resp)
	if resp.Type != "response" {
		t.Fatalf("check: type = %q, want response", resp.Type)
	}
	var result map[string]any
	json.Unmarshal(resp.Result, &result)
	if result["total"] != float64(2) {
		t.Errorf("total = %v, want 2", result["total"])
	}
	if result["closed"] != float64(1) {
		t.Errorf("closed = %v, want 1", result["closed"])
	}
	if result["complete"] != false {
		t.Errorf("complete = %v, want false", result["complete"])
	}
}

func TestConvoyCheckComplete(t *testing.T) {
	state := newFakeMutatorState(t)
	srv := New(state)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()
	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	store := state.stores["myrig"]
	convoy, _ := store.Create(beads.Bead{Title: "convoy", Type: "convoy"})
	item, _ := store.Create(beads.Bead{Title: "task"})
	pid := convoy.ID
	store.Update(item.ID, beads.UpdateOpts{ParentID: &pid}) //nolint:errcheck
	store.Close(item.ID)                                    //nolint:errcheck

	writeWSJSON(t, conn, wsRequestEnvelope{Type: "request", ID: "chk-complete", Action: "convoy.check", Payload: map[string]any{"id": convoy.ID}})
	var resp wsResponseEnvelope
	readWSJSON(t, conn, &resp)
	var result map[string]any
	json.Unmarshal(resp.Result, &result)
	if result["complete"] != true {
		t.Errorf("complete = %v, want true", result["complete"])
	}
}

func TestConvoyDelete(t *testing.T) {
	state := newFakeMutatorState(t)
	srv := New(state)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()
	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	store := state.stores["myrig"]
	convoy, _ := store.Create(beads.Bead{Title: "convoy", Type: "convoy"})

	writeWSJSON(t, conn, wsRequestEnvelope{Type: "request", ID: "cd-1", Action: "convoy.delete", Payload: map[string]any{"id": convoy.ID}})
	var resp wsResponseEnvelope
	readWSJSON(t, conn, &resp)
	if resp.Type != "response" {
		t.Fatalf("delete: type = %q, want response", resp.Type)
	}

	got, _ := store.Get(convoy.ID)
	if got.Status != "closed" {
		t.Errorf("Status = %q, want closed", got.Status)
	}
}

func TestConvoyDeleteNotConvoy(t *testing.T) {
	state := newFakeMutatorState(t)
	srv := New(state)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()
	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	store := state.stores["myrig"]
	task, _ := store.Create(beads.Bead{Title: "task", Type: "task"})

	writeWSJSON(t, conn, wsRequestEnvelope{Type: "request", ID: "cd-nc", Action: "convoy.delete", Payload: map[string]any{"id": task.ID}})
	var errResp wsErrorEnvelope
	readWSJSON(t, conn, &errResp)
	if errResp.Type != "error" {
		t.Fatalf("type = %q, want error", errResp.Type)
	}
}

func TestConvoyList(t *testing.T) {
	state := newFakeMutatorState(t)
	srv := New(state)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()
	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	store := state.stores["myrig"]
	store.Create(beads.Bead{Title: "convoy", Type: "convoy"}) //nolint:errcheck
	store.Create(beads.Bead{Title: "task", Type: "task"})     //nolint:errcheck

	writeWSJSON(t, conn, wsRequestEnvelope{Type: "request", ID: "cl-1", Action: "convoys.list"})
	var resp wsResponseEnvelope
	readWSJSON(t, conn, &resp)
	if resp.Type != "response" {
		t.Fatalf("list: type = %q, want response", resp.Type)
	}
	var lr listResponse
	json.Unmarshal(resp.Result, &lr)
	if lr.Total != 1 {
		t.Fatalf("total = %d, want 1 (only convoys)", lr.Total)
	}
}
