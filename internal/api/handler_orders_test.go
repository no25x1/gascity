package api

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gastownhall/gascity/internal/beads"
	"github.com/gastownhall/gascity/internal/orders"
)

func TestHandleOrderList_Empty(t *testing.T) {
	fs := newFakeState(t)
	srv := New(fs)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	writeWSJSON(t, conn, wsRequestEnvelope{Type: "request", ID: "test-1", Action: "orders.list"})
	var resp wsResponseEnvelope
	readWSJSON(t, conn, &resp)

	if resp.Type != "response" || resp.ID != "test-1" {
		t.Fatalf("response = %#v, want correlated response", resp)
	}

	var body struct {
		Orders []orderResponse `json:"orders"`
	}
	if err := json.Unmarshal(resp.Result, &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(body.Orders) != 0 {
		t.Errorf("len(orders) = %d, want 0", len(body.Orders))
	}
}

func TestHandleOrderList(t *testing.T) {
	fs := newFakeState(t)
	enabled := true
	fs.autos = []orders.Order{
		{
			Name:        "dolt-health",
			Description: "Check dolt status",
			Exec:        "dolt status",
			Gate:        "cooldown",
			Interval:    "5m",
			Enabled:     &enabled,
		},
		{
			Name:    "deploy",
			Formula: "deploy-steps",
			Gate:    "manual",
			Pool:    "workers",
			Rig:     "myrig",
		},
	}
	srv := New(fs)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	writeWSJSON(t, conn, wsRequestEnvelope{Type: "request", ID: "test-1", Action: "orders.list"})
	var resp wsResponseEnvelope
	readWSJSON(t, conn, &resp)

	if resp.Type != "response" || resp.ID != "test-1" {
		t.Fatalf("response = %#v, want correlated response", resp)
	}

	var body struct {
		Orders []orderResponse `json:"orders"`
	}
	if err := json.Unmarshal(resp.Result, &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(body.Orders) != 2 {
		t.Fatalf("len(orders) = %d, want 2", len(body.Orders))
	}

	a0 := body.Orders[0]
	if a0.Name != "dolt-health" {
		t.Errorf("name = %q, want %q", a0.Name, "dolt-health")
	}
	if a0.Type != "exec" {
		t.Errorf("type = %q, want %q", a0.Type, "exec")
	}
	if a0.Gate != "cooldown" {
		t.Errorf("gate = %q, want %q", a0.Gate, "cooldown")
	}
	if a0.Interval != "5m" {
		t.Errorf("interval = %q, want %q", a0.Interval, "5m")
	}
	if !a0.Enabled {
		t.Error("expected enabled=true")
	}

	a1 := body.Orders[1]
	if a1.Name != "deploy" {
		t.Errorf("name = %q, want %q", a1.Name, "deploy")
	}
	if a1.Type != "formula" {
		t.Errorf("type = %q, want %q", a1.Type, "formula")
	}
	if a1.Rig != "myrig" {
		t.Errorf("rig = %q, want %q", a1.Rig, "myrig")
	}
	if a1.Pool != "workers" {
		t.Errorf("pool = %q, want %q", a1.Pool, "workers")
	}
}

func TestHandleOrderGet(t *testing.T) {
	fs := newFakeState(t)
	fs.autos = []orders.Order{
		{
			Name:        "dolt-health",
			Description: "Check dolt status",
			Exec:        "dolt status",
			Gate:        "cooldown",
			Interval:    "5m",
		},
	}
	srv := New(fs)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	writeWSJSON(t, conn, wsRequestEnvelope{
		Type:    "request",
		ID:      "test-1",
		Action:  "order.get",
		Payload: map[string]any{"name": "dolt-health"},
	})
	var resp wsResponseEnvelope
	readWSJSON(t, conn, &resp)

	if resp.Type != "response" || resp.ID != "test-1" {
		t.Fatalf("response = %#v, want correlated response", resp)
	}

	var body orderResponse
	if err := json.Unmarshal(resp.Result, &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body.Name != "dolt-health" {
		t.Errorf("name = %q, want %q", body.Name, "dolt-health")
	}
	if body.Type != "exec" {
		t.Errorf("type = %q, want %q", body.Type, "exec")
	}
}

func TestHandleOrderGet_ScopedName(t *testing.T) {
	fs := newFakeState(t)
	fs.autos = []orders.Order{
		{
			Name: "health",
			Exec: "echo ok",
			Gate: "cooldown",
			Rig:  "myrig",
		},
	}
	srv := New(fs)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	// Match by scoped name: health:rig:myrig
	writeWSJSON(t, conn, wsRequestEnvelope{
		Type:    "request",
		ID:      "test-1",
		Action:  "order.get",
		Payload: map[string]any{"name": "health:rig:myrig"},
	})
	var resp wsResponseEnvelope
	readWSJSON(t, conn, &resp)

	if resp.Type != "response" || resp.ID != "test-1" {
		t.Fatalf("response = %#v, want correlated response", resp)
	}

	var body orderResponse
	if err := json.Unmarshal(resp.Result, &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body.Name != "health" {
		t.Errorf("name = %q, want %q", body.Name, "health")
	}
	if body.Rig != "myrig" {
		t.Errorf("rig = %q, want %q", body.Rig, "myrig")
	}
}

func TestHandleOrderGet_NotFound(t *testing.T) {
	fs := newFakeState(t)
	srv := New(fs)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	writeWSJSON(t, conn, wsRequestEnvelope{
		Type:    "request",
		ID:      "test-1",
		Action:  "order.get",
		Payload: map[string]any{"name": "nonexistent"},
	})
	var errResp wsErrorEnvelope
	readWSJSON(t, conn, &errResp)

	if errResp.Type != "error" {
		t.Fatalf("type = %q, want error", errResp.Type)
	}
	if errResp.Code != "not_found" {
		t.Fatalf("code = %q, want not_found", errResp.Code)
	}
}

func TestHandleOrderDisable(t *testing.T) {
	fs := newFakeMutatorState(t)
	fs.autos = []orders.Order{
		{Name: "health", Exec: "echo ok", Gate: "cooldown"},
	}
	srv := New(fs)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	writeWSJSON(t, conn, wsRequestEnvelope{
		Type:    "request",
		ID:      "test-1",
		Action:  "order.disable",
		Payload: map[string]any{"name": "health"},
	})
	var resp wsResponseEnvelope
	readWSJSON(t, conn, &resp)

	if resp.Type != "response" || resp.ID != "test-1" {
		t.Fatalf("response = %#v, want correlated response", resp)
	}

	// Verify override was written.
	if len(fs.cfg.Orders.Overrides) != 1 {
		t.Fatalf("expected 1 override, got %d", len(fs.cfg.Orders.Overrides))
	}
	ov := fs.cfg.Orders.Overrides[0]
	if ov.Name != "health" {
		t.Errorf("override name = %q, want %q", ov.Name, "health")
	}
	if ov.Enabled == nil || *ov.Enabled {
		t.Error("expected enabled=false")
	}
}

func TestHandleOrderEnable(t *testing.T) {
	fs := newFakeMutatorState(t)
	fs.autos = []orders.Order{
		{Name: "health", Exec: "echo ok", Gate: "cooldown"},
	}
	srv := New(fs)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	writeWSJSON(t, conn, wsRequestEnvelope{
		Type:    "request",
		ID:      "test-1",
		Action:  "order.enable",
		Payload: map[string]any{"name": "health"},
	})
	var resp wsResponseEnvelope
	readWSJSON(t, conn, &resp)

	if resp.Type != "response" || resp.ID != "test-1" {
		t.Fatalf("response = %#v, want correlated response", resp)
	}

	if len(fs.cfg.Orders.Overrides) != 1 {
		t.Fatalf("expected 1 override, got %d", len(fs.cfg.Orders.Overrides))
	}
	ov := fs.cfg.Orders.Overrides[0]
	if ov.Enabled == nil || !*ov.Enabled {
		t.Error("expected enabled=true")
	}
}

func TestHandleOrdersFeedReturnsWorkflowAndScheduledOrderRuns(t *testing.T) {
	fs := newFakeState(t)
	fs.cityBeadStore = beads.NewMemStore()
	fs.autos = []orders.Order{
		{Name: "nightly-review", Formula: "mol-adopt-pr-v2", Gate: "cron", Pool: "reviewers", Rig: "myrig"},
	}

	rigStore := fs.stores["myrig"]
	if rigStore == nil {
		t.Fatal("expected rig store")
	}
	root, err := rigStore.Create(beads.Bead{
		Title: "Adopt PR",
		Ref:   "mol-adopt-pr-v2",
		Metadata: map[string]string{
			"gc.kind":             "workflow",
			"gc.formula_contract": "graph.v2",
			"gc.workflow_id":      "wf-123",
			"gc.run_target":       "myrig/claude",
			"gc.scope_kind":       "rig",
			"gc.scope_ref":        "myrig",
			"gc.source_bead_id":   "bd-42",
		},
	})
	if err != nil {
		t.Fatalf("create workflow root: %v", err)
	}
	inProgress := "in_progress"
	assignee := "myrig/claude"
	if err := rigStore.Update(root.ID, beads.UpdateOpts{Status: &inProgress, Assignee: &assignee}); err != nil {
		t.Fatalf("set workflow in_progress: %v", err)
	}

	_, err = fs.cityBeadStore.Create(beads.Bead{
		Title:  "order:nightly-review:rig:myrig",
		Status: "closed",
		Labels: []string{"order-tracking", "order-run:nightly-review:rig:myrig", "wisp"},
	})
	if err != nil {
		t.Fatalf("create tracking bead: %v", err)
	}
	time.Sleep(time.Millisecond)
	_, err = fs.cityBeadStore.Create(beads.Bead{
		Title:  "nightly-review wisp",
		Type:   "wisp",
		Status: "in_progress",
		Labels: []string{"order-run:nightly-review:rig:myrig", "wisp"},
	})
	if err != nil {
		t.Fatalf("create wisp bead: %v", err)
	}

	srv := New(fs)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	writeWSJSON(t, conn, wsRequestEnvelope{
		Type:   "request",
		ID:     "test-1",
		Action: "orders.feed",
		Payload: map[string]any{
			"scope_kind": "rig",
			"scope_ref":  "myrig",
		},
	})
	var resp wsResponseEnvelope
	readWSJSON(t, conn, &resp)

	if resp.Type != "response" || resp.ID != "test-1" {
		t.Fatalf("response = %#v, want correlated response", resp)
	}

	var body struct {
		Items         []monitorFeedItemResponse `json:"items"`
		Partial       bool                      `json:"partial"`
		PartialErrors []string                  `json:"partial_errors"`
	}
	if err := json.Unmarshal(resp.Result, &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(body.Items) != 2 {
		t.Fatalf("len(items) = %d, want 2", len(body.Items))
	}

	if body.Items[0].WorkflowID != "wf-123" || body.Items[0].Type != "formula" {
		t.Fatalf("items[0] = %+v, want workflow feed item first", body.Items[0])
	}
	if body.Items[0].Target != "myrig/claude" {
		t.Fatalf("workflow target = %q, want myrig/claude", body.Items[0].Target)
	}
	if !body.Items[0].RunDetailAvailable || body.Items[0].DetailAvailable {
		t.Fatalf("workflow detail flags = %+v, want run_detail_available only", body.Items[0])
	}

	if body.Items[1].BeadID == "" || body.Items[1].Type != "formula" {
		t.Fatalf("items[1] = %+v, want scheduled formula order tracking item", body.Items[1])
	}
	if body.Items[1].Target != "myrig/reviewers" {
		t.Fatalf("scheduled order target = %q, want myrig/reviewers", body.Items[1].Target)
	}
	if body.Items[1].UpdatedAt == body.Items[1].StartedAt {
		t.Fatalf("scheduled order timestamps = started %q updated %q, want updated_at to reflect newer run activity", body.Items[1].StartedAt, body.Items[1].UpdatedAt)
	}
}

func TestHandleOrderCheckTreatsWispFailedAsFailed(t *testing.T) {
	fs := newFakeState(t)
	fs.cityBeadStore = beads.NewMemStore()
	fs.autos = []orders.Order{
		{Name: "nightly-review", Formula: "mol-adopt-pr-v2", Gate: "cooldown", Interval: "1h", Rig: "myrig"},
	}

	_, err := fs.cityBeadStore.Create(beads.Bead{
		Title:  "order:nightly-review:rig:myrig",
		Status: "closed",
		Labels: []string{"order-tracking", "order-run:nightly-review:rig:myrig", "wisp", "wisp-failed"},
	})
	if err != nil {
		t.Fatalf("create tracking bead: %v", err)
	}

	srv := New(fs)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	writeWSJSON(t, conn, wsRequestEnvelope{Type: "request", ID: "test-1", Action: "orders.check"})
	var resp wsResponseEnvelope
	readWSJSON(t, conn, &resp)

	if resp.Type != "response" || resp.ID != "test-1" {
		t.Fatalf("response = %#v, want correlated response", resp)
	}

	var body struct {
		Checks []struct {
			ScopedName     string  `json:"scoped_name"`
			LastRunOutcome *string `json:"last_run_outcome"`
		} `json:"checks"`
	}
	if err := json.Unmarshal(resp.Result, &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(body.Checks) != 1 {
		t.Fatalf("len(checks) = %d, want 1", len(body.Checks))
	}
	if body.Checks[0].LastRunOutcome == nil || *body.Checks[0].LastRunOutcome != "failed" {
		t.Fatalf("last_run_outcome = %v, want failed", body.Checks[0].LastRunOutcome)
	}
}

func TestLastRunOutcomeFromLabelsPrioritizesTerminalLabels(t *testing.T) {
	tests := []struct {
		name   string
		labels []string
		want   string
	}{
		{name: "wisp failed dominates success", labels: []string{"wisp", "wisp-failed"}, want: "failed"},
		{name: "failed alone", labels: []string{"wisp-failed"}, want: "failed"},
		{name: "exec failed dominates success", labels: []string{"exec", "exec-failed"}, want: "failed"},
		{name: "canceled dominates success", labels: []string{"wisp", "wisp-canceled"}, want: "canceled"},
		{name: "success fallback", labels: []string{"exec"}, want: "success"},
		{name: "unknown", labels: []string{"order-tracking"}, want: ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := lastRunOutcomeFromLabels(tc.labels); got != tc.want {
				t.Fatalf("lastRunOutcomeFromLabels(%v) = %q, want %q", tc.labels, got, tc.want)
			}
		})
	}
}

func TestHandleOrdersFeedIgnoresUnrelatedStoreListFailures(t *testing.T) {
	fs := newFakeState(t)
	fs.stores["alpha"] = failListStore{Store: beads.NewMemStore()}
	rigStore := fs.stores["myrig"]
	if rigStore == nil {
		t.Fatal("expected rig store")
	}

	root, err := rigStore.Create(beads.Bead{
		Title: "Adopt PR",
		Ref:   "mol-adopt-pr-v2",
		Metadata: map[string]string{
			"gc.kind":             "workflow",
			"gc.formula_contract": "graph.v2",
			"gc.workflow_id":      "wf-healthy",
			"gc.run_target":       "myrig/claude",
			"gc.scope_kind":       "rig",
			"gc.scope_ref":        "myrig",
		},
	})
	if err != nil {
		t.Fatalf("create workflow root: %v", err)
	}
	inProgress := "in_progress"
	if err := rigStore.Update(root.ID, beads.UpdateOpts{Status: &inProgress}); err != nil {
		t.Fatalf("set workflow in_progress: %v", err)
	}

	srv := New(fs)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	writeWSJSON(t, conn, wsRequestEnvelope{
		Type:   "request",
		ID:     "test-1",
		Action: "orders.feed",
		Payload: map[string]any{
			"scope_kind": "rig",
			"scope_ref":  "myrig",
		},
	})
	var resp wsResponseEnvelope
	readWSJSON(t, conn, &resp)

	if resp.Type != "response" || resp.ID != "test-1" {
		t.Fatalf("response = %#v, want correlated response", resp)
	}

	var body struct {
		Items         []monitorFeedItemResponse `json:"items"`
		Partial       bool                      `json:"partial"`
		PartialErrors []string                  `json:"partial_errors"`
	}
	if err := json.Unmarshal(resp.Result, &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(body.Items) != 1 {
		t.Fatalf("len(items) = %d, want 1", len(body.Items))
	}
	if body.Items[0].WorkflowID != "wf-healthy" {
		t.Fatalf("items[0] = %+v, want healthy workflow result", body.Items[0])
	}
	if body.Partial {
		t.Fatalf("partial = true, want false; errors = %v", body.PartialErrors)
	}
}

func TestHandleOrdersFeedCityScopeIncludesRigWorkflowRuns(t *testing.T) {
	fs := newFakeState(t)
	rigStore := fs.stores["myrig"]
	if rigStore == nil {
		t.Fatal("expected rig store")
	}

	_, err := rigStore.Create(beads.Bead{
		Title: "Cross-rig run",
		Ref:   "mol-adopt-pr-v2",
		Metadata: map[string]string{
			"gc.kind":             "workflow",
			"gc.formula_contract": "graph.v2",
			"gc.workflow_id":      "wf-city-view",
			"gc.run_target":       "myrig/codex",
			"gc.scope_kind":       "rig",
			"gc.scope_ref":        "myrig",
		},
	})
	if err != nil {
		t.Fatalf("create workflow root: %v", err)
	}

	srv := New(fs)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	writeWSJSON(t, conn, wsRequestEnvelope{
		Type:   "request",
		ID:     "test-1",
		Action: "orders.feed",
		Payload: map[string]any{
			"scope_kind": "city",
			"scope_ref":  "test-city",
		},
	})
	var resp wsResponseEnvelope
	readWSJSON(t, conn, &resp)

	if resp.Type != "response" || resp.ID != "test-1" {
		t.Fatalf("response = %#v, want correlated response", resp)
	}

	var body struct {
		Items []monitorFeedItemResponse `json:"items"`
	}
	if err := json.Unmarshal(resp.Result, &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(body.Items) != 1 {
		t.Fatalf("len(items) = %d, want 1", len(body.Items))
	}
	if body.Items[0].WorkflowID != "wf-city-view" {
		t.Fatalf("items[0] = %+v, want rig workflow visible in city feed", body.Items[0])
	}
	if body.Items[0].ScopeKind != "rig" || body.Items[0].ScopeRef != "myrig" {
		t.Fatalf("scope = %s/%s, want rig/myrig", body.Items[0].ScopeKind, body.Items[0].ScopeRef)
	}
}

func TestHandleOrdersFeedCityScopeReportsPartialRigFailures(t *testing.T) {
	fs := newFakeState(t)
	fs.stores["alpha"] = failListStore{Store: beads.NewMemStore()}
	rigStore := fs.stores["myrig"]
	if rigStore == nil {
		t.Fatal("expected rig store")
	}

	_, err := rigStore.Create(beads.Bead{
		Title: "Cross-rig run",
		Ref:   "mol-adopt-pr-v2",
		Metadata: map[string]string{
			"gc.kind":             "workflow",
			"gc.formula_contract": "graph.v2",
			"gc.workflow_id":      "wf-city-view",
			"gc.run_target":       "myrig/codex",
			"gc.scope_kind":       "rig",
			"gc.scope_ref":        "myrig",
		},
	})
	if err != nil {
		t.Fatalf("create workflow root: %v", err)
	}

	srv := New(fs)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	writeWSJSON(t, conn, wsRequestEnvelope{
		Type:   "request",
		ID:     "test-1",
		Action: "orders.feed",
		Payload: map[string]any{
			"scope_kind": "city",
			"scope_ref":  "test-city",
		},
	})
	var resp wsResponseEnvelope
	readWSJSON(t, conn, &resp)

	if resp.Type != "response" || resp.ID != "test-1" {
		t.Fatalf("response = %#v, want correlated response", resp)
	}

	var body struct {
		Items         []monitorFeedItemResponse `json:"items"`
		Partial       bool                      `json:"partial"`
		PartialErrors []string                  `json:"partial_errors"`
	}
	if err := json.Unmarshal(resp.Result, &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(body.Items) != 1 {
		t.Fatalf("len(items) = %d, want 1", len(body.Items))
	}
	if !body.Partial {
		t.Fatalf("partial = false, want true")
	}
	if len(body.PartialErrors) != 1 || body.PartialErrors[0] != "rig:alpha store unavailable" {
		t.Fatalf("partial_errors = %v, want rig:alpha store unavailable", body.PartialErrors)
	}
}

func TestHandleOrderGet_Ambiguous(t *testing.T) {
	fs := newFakeState(t)
	fs.autos = []orders.Order{
		{Name: "health", Exec: "echo ok", Gate: "cooldown", Rig: "rig-a"},
		{Name: "health", Exec: "echo ok", Gate: "cooldown", Rig: "rig-b"},
	}
	srv := New(fs)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	// Bare name should return ambiguous error.
	writeWSJSON(t, conn, wsRequestEnvelope{
		Type:    "request",
		ID:      "test-1",
		Action:  "order.get",
		Payload: map[string]any{"name": "health"},
	})
	var errResp wsErrorEnvelope
	readWSJSON(t, conn, &errResp)

	if errResp.Type != "error" {
		t.Fatalf("type = %q, want error", errResp.Type)
	}
	if errResp.Code != "ambiguous" {
		t.Fatalf("code = %q, want ambiguous", errResp.Code)
	}
	if !strings.Contains(errResp.Message, "ambiguous") {
		t.Fatalf("message = %q, want ambiguous mention", errResp.Message)
	}

	// Scoped name should resolve unambiguously.
	writeWSJSON(t, conn, wsRequestEnvelope{
		Type:    "request",
		ID:      "test-2",
		Action:  "order.get",
		Payload: map[string]any{"name": "health:rig:rig-a"},
	})
	var resp wsResponseEnvelope
	readWSJSON(t, conn, &resp)

	if resp.Type != "response" || resp.ID != "test-2" {
		t.Fatalf("response = %#v, want correlated response", resp)
	}
	var body orderResponse
	if err := json.Unmarshal(resp.Result, &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body.Rig != "rig-a" {
		t.Errorf("rig = %q, want %q", body.Rig, "rig-a")
	}
}

func TestHandleOrderDisable_Ambiguous(t *testing.T) {
	fs := newFakeMutatorState(t)
	fs.autos = []orders.Order{
		{Name: "health", Exec: "echo ok", Gate: "cooldown", Rig: "rig-a"},
		{Name: "health", Exec: "echo ok", Gate: "cooldown", Rig: "rig-b"},
	}
	srv := New(fs)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	writeWSJSON(t, conn, wsRequestEnvelope{
		Type:    "request",
		ID:      "test-1",
		Action:  "order.disable",
		Payload: map[string]any{"name": "health"},
	})
	var errResp wsErrorEnvelope
	readWSJSON(t, conn, &errResp)

	if errResp.Type != "error" {
		t.Fatalf("type = %q, want error", errResp.Type)
	}
	if errResp.Code != "ambiguous" {
		t.Fatalf("code = %q, want ambiguous", errResp.Code)
	}
}

func TestHandleOrderDisable_NotFound(t *testing.T) {
	fs := newFakeMutatorState(t)
	srv := New(fs)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	writeWSJSON(t, conn, wsRequestEnvelope{
		Type:    "request",
		ID:      "test-1",
		Action:  "order.disable",
		Payload: map[string]any{"name": "nonexistent"},
	})
	var errResp wsErrorEnvelope
	readWSJSON(t, conn, &errResp)

	if errResp.Type != "error" {
		t.Fatalf("type = %q, want error", errResp.Type)
	}
	if errResp.Code != "not_found" {
		t.Fatalf("code = %q, want not_found", errResp.Code)
	}
}
