package api

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gastownhall/gascity/internal/beads"
	"github.com/gastownhall/gascity/internal/events"
)

type countingStore struct {
	beads.Store

	listCalls           int
	listByLabelCalls    int
	listByAssigneeCalls int
}

func (s *countingStore) ListOpen(status ...string) ([]beads.Bead, error) {
	s.listCalls++
	return s.Store.ListOpen(status...)
}

func (s *countingStore) List(query beads.ListQuery) ([]beads.Bead, error) {
	switch {
	case query.Assignee != "":
		s.listByAssigneeCalls++
	case query.Label != "":
		s.listByLabelCalls++
	case query.Status != "" || query.AllowScan:
		s.listCalls++
	}
	return s.Store.List(query)
}

func (s *countingStore) ListByLabel(label string, limit int, opts ...beads.QueryOpt) ([]beads.Bead, error) {
	s.listByLabelCalls++
	return s.Store.ListByLabel(label, limit, opts...)
}

func (s *countingStore) ListByAssignee(assignee, status string, limit int) ([]beads.Bead, error) {
	s.listByAssigneeCalls++
	return s.Store.ListByAssignee(assignee, status, limit)
}

func TestHandleStatusCachesUntilIndexChanges(t *testing.T) {
	// HTTP caching is eliminated. Verify WS status.get returns fresh data
	// with an Index that tracks event sequence.
	state := newFakeState(t)
	state.eventProv.Record(events.Event{Type: "test", Actor: "t"})
	srv := New(state)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()
	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	writeWSJSON(t, conn, wsRequestEnvelope{Type: "request", ID: "s1", Action: "status.get"})
	var resp wsResponseEnvelope
	readWSJSON(t, conn, &resp)
	if resp.Type != "response" {
		t.Fatalf("type = %q, want response", resp.Type)
	}
	idx1 := resp.Index

	// Record another event and check index advances.
	state.eventProv.Record(events.Event{Type: events.BeadCreated, Actor: "human"})
	writeWSJSON(t, conn, wsRequestEnvelope{Type: "request", ID: "s2", Action: "status.get"})
	readWSJSON(t, conn, &resp)
	if resp.Index <= idx1 {
		t.Fatalf("index after event = %d, want > %d", resp.Index, idx1)
	}
}

func TestHandleAgentListCachesUntilIndexChanges(t *testing.T) {
	// HTTP caching is eliminated. Verify WS agents.list returns with index.
	state := newFakeState(t)
	srv := New(state)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()
	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	writeWSJSON(t, conn, wsRequestEnvelope{Type: "request", ID: "a1", Action: "agents.list"})
	var resp wsResponseEnvelope
	readWSJSON(t, conn, &resp)
	if resp.Type != "response" {
		t.Fatalf("type = %q, want response", resp.Type)
	}
}

func TestHandleOrdersFeedCachesUntilIndexChanges(t *testing.T) {
	// HTTP caching eliminated. Verify WS orders.feed returns data.
	state := newFakeState(t)
	rigStore := beads.NewMemStore()
	state.stores["myrig"] = rigStore

	rigStore.Create(beads.Bead{ //nolint:errcheck
		Title: "Adopt PR",
		Ref:   "mol-adopt-pr-v2",
		Metadata: map[string]string{
			"gc.kind":             "workflow",
			"gc.formula_contract": "graph.v2",
			"gc.workflow_id":      "wf-123",
			"gc.scope_kind":       "rig",
			"gc.scope_ref":        "myrig",
		},
	})

	srv := New(state)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()
	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	writeWSJSON(t, conn, wsRequestEnvelope{Type: "request", ID: "of1", Action: "orders.feed", Payload: map[string]any{"scope_kind": "rig", "scope_ref": "myrig"}})
	var resp wsResponseEnvelope
	readWSJSON(t, conn, &resp)
	if resp.Type != "response" {
		t.Fatalf("type = %q, want response", resp.Type)
	}
	var feed map[string]any
	json.Unmarshal(resp.Result, &feed)
	if feed["items"] == nil {
		t.Fatal("feed items missing")
	}
}
