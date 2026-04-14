package api

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gastownhall/gascity/internal/beads"
	"github.com/gastownhall/gascity/internal/config"
	"github.com/gorilla/websocket"
)

// WS test types and helpers are in websocket_test.go.

// wsRoundTrip sends a request and reads the response, handling both success
// and error envelopes. Returns the response envelope if type is "response",
// or fails the test for unexpected types. For error checking use wsRoundTripRaw.
func wsRoundTrip(t *testing.T, conn *websocket.Conn, req wsRequestEnvelope) wsResponseEnvelope {
	t.Helper()
	writeWSJSON(t, conn, req)
	// Read raw message to determine type.
	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read ws message: %v", err)
	}
	var envelope struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(msg, &envelope); err != nil {
		t.Fatalf("unmarshal envelope type: %v", err)
	}
	if envelope.Type == "error" {
		var errEnv wsErrorEnvelope
		if err := json.Unmarshal(msg, &errEnv); err != nil {
			t.Fatalf("unmarshal error envelope: %v", err)
		}
		t.Fatalf("expected response, got error: code=%q message=%q", errEnv.Code, errEnv.Message)
	}
	var resp wsResponseEnvelope
	if err := json.Unmarshal(msg, &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	return resp
}

// wsRoundTripRaw sends a request and returns the raw message bytes.
func wsRoundTripRaw(t *testing.T, conn *websocket.Conn, req wsRequestEnvelope) []byte {
	t.Helper()
	writeWSJSON(t, conn, req)
	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read ws message: %v", err)
	}
	return msg
}

func connectWS(t *testing.T, state *fakeState) *websocket.Conn {
	t.Helper()
	srv := New(state)
	ts := httptest.NewServer(srv.handler())
	t.Cleanup(ts.Close)
	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	t.Cleanup(func() { conn.Close() })
	drainWSHello(t, conn)
	return conn
}

// --- Prefixed alias store (unchanged) ---

type prefixedAliasStore struct {
	prefix        string
	base          *beads.MemStore
	getCalls      int
	updateCalls   int
	closeCalls    int
	childrenCalls int
}

func newPrefixedAliasStore(prefix string) *prefixedAliasStore {
	return &prefixedAliasStore{
		prefix: prefix,
		base:   beads.NewMemStore(),
	}
}

func (s *prefixedAliasStore) aliasToBase(id string) string {
	if strings.HasPrefix(id, s.prefix) {
		return "gc" + strings.TrimPrefix(id, s.prefix)
	}
	return id
}

func (s *prefixedAliasStore) baseToAlias(id string) string {
	if strings.HasPrefix(id, "gc") {
		return s.prefix + strings.TrimPrefix(id, "gc")
	}
	return id
}

func (s *prefixedAliasStore) beadToAlias(b beads.Bead) beads.Bead {
	b.ID = s.baseToAlias(b.ID)
	if b.ParentID != "" {
		b.ParentID = s.baseToAlias(b.ParentID)
	}
	if len(b.Needs) > 0 {
		needs := make([]string, 0, len(b.Needs))
		for _, need := range b.Needs {
			depType, depID, ok := strings.Cut(need, ":")
			if ok && depType != "" && depID != "" {
				needs = append(needs, depType+":"+s.baseToAlias(depID))
				continue
			}
			needs = append(needs, s.baseToAlias(need))
		}
		b.Needs = needs
	}
	return b
}

func (s *prefixedAliasStore) depToAlias(dep beads.Dep) beads.Dep {
	dep.IssueID = s.baseToAlias(dep.IssueID)
	dep.DependsOnID = s.baseToAlias(dep.DependsOnID)
	return dep
}

func (s *prefixedAliasStore) Create(b beads.Bead) (beads.Bead, error) {
	if b.ParentID != "" {
		b.ParentID = s.aliasToBase(b.ParentID)
	}
	if len(b.Needs) > 0 {
		needs := make([]string, 0, len(b.Needs))
		for _, need := range b.Needs {
			depType, depID, ok := strings.Cut(need, ":")
			if ok && depType != "" && depID != "" {
				needs = append(needs, depType+":"+s.aliasToBase(depID))
				continue
			}
			needs = append(needs, s.aliasToBase(need))
		}
		b.Needs = needs
	}
	created, err := s.base.Create(b)
	if err != nil {
		return beads.Bead{}, err
	}
	return s.beadToAlias(created), nil
}

func (s *prefixedAliasStore) Get(id string) (beads.Bead, error) {
	s.getCalls++
	b, err := s.base.Get(s.aliasToBase(id))
	if err != nil {
		return beads.Bead{}, err
	}
	return s.beadToAlias(b), nil
}

func (s *prefixedAliasStore) Update(id string, opts beads.UpdateOpts) error {
	s.updateCalls++
	if opts.ParentID != nil {
		parentID := s.aliasToBase(*opts.ParentID)
		opts.ParentID = &parentID
	}
	return s.base.Update(s.aliasToBase(id), opts)
}

func (s *prefixedAliasStore) Close(id string) error {
	s.closeCalls++
	return s.base.Close(s.aliasToBase(id))
}

func (s *prefixedAliasStore) CloseAll(ids []string, metadata map[string]string) (int, error) {
	mapped := make([]string, 0, len(ids))
	for _, id := range ids {
		mapped = append(mapped, s.aliasToBase(id))
	}
	return s.base.CloseAll(mapped, metadata)
}

func (s *prefixedAliasStore) ListOpen(status ...string) ([]beads.Bead, error) {
	items, err := s.base.ListOpen(status...)
	if err != nil {
		return nil, err
	}
	out := make([]beads.Bead, 0, len(items))
	for _, item := range items {
		out = append(out, s.beadToAlias(item))
	}
	return out, nil
}

func (s *prefixedAliasStore) List(query beads.ListQuery) ([]beads.Bead, error) {
	if query.ParentID != "" {
		s.childrenCalls++
		query.ParentID = s.aliasToBase(query.ParentID)
	}
	if len(query.Metadata) > 0 {
		filters := make(map[string]string, len(query.Metadata))
		for k, v := range query.Metadata {
			switch k {
			case "gc.root_bead_id", "gc.workflow_id", "gc.source_bead_id":
				filters[k] = s.aliasToBase(v)
			default:
				filters[k] = v
			}
		}
		query.Metadata = filters
	}
	items, err := s.base.List(query)
	if err != nil {
		return nil, err
	}
	out := make([]beads.Bead, 0, len(items))
	for _, item := range items {
		out = append(out, s.beadToAlias(item))
	}
	return out, nil
}

func (s *prefixedAliasStore) Ready() ([]beads.Bead, error) {
	items, err := s.base.Ready()
	if err != nil {
		return nil, err
	}
	out := make([]beads.Bead, 0, len(items))
	for _, item := range items {
		out = append(out, s.beadToAlias(item))
	}
	return out, nil
}

func (s *prefixedAliasStore) Children(parentID string, opts ...beads.QueryOpt) ([]beads.Bead, error) {
	s.childrenCalls++
	items, err := s.base.Children(s.aliasToBase(parentID), opts...)
	if err != nil {
		return nil, err
	}
	out := make([]beads.Bead, 0, len(items))
	for _, item := range items {
		out = append(out, s.beadToAlias(item))
	}
	return out, nil
}

func (s *prefixedAliasStore) ListByLabel(label string, limit int, opts ...beads.QueryOpt) ([]beads.Bead, error) {
	items, err := s.base.ListByLabel(label, limit, opts...)
	if err != nil {
		return nil, err
	}
	out := make([]beads.Bead, 0, len(items))
	for _, item := range items {
		out = append(out, s.beadToAlias(item))
	}
	return out, nil
}

func (s *prefixedAliasStore) ListByAssignee(assignee, status string, limit int) ([]beads.Bead, error) {
	items, err := s.base.ListByAssignee(assignee, status, limit)
	if err != nil {
		return nil, err
	}
	out := make([]beads.Bead, 0, len(items))
	for _, item := range items {
		out = append(out, s.beadToAlias(item))
	}
	return out, nil
}

func (s *prefixedAliasStore) SetMetadata(id, key, value string) error {
	return s.base.SetMetadata(s.aliasToBase(id), key, value)
}

func (s *prefixedAliasStore) SetMetadataBatch(id string, kvs map[string]string) error {
	return s.base.SetMetadataBatch(s.aliasToBase(id), kvs)
}

func (s *prefixedAliasStore) Ping() error {
	return s.base.Ping()
}

func (s *prefixedAliasStore) DepAdd(issueID, dependsOnID, depType string) error {
	return s.base.DepAdd(s.aliasToBase(issueID), s.aliasToBase(dependsOnID), depType)
}

func (s *prefixedAliasStore) DepRemove(issueID, dependsOnID string) error {
	return s.base.DepRemove(s.aliasToBase(issueID), s.aliasToBase(dependsOnID))
}

func (s *prefixedAliasStore) DepList(id, direction string) ([]beads.Dep, error) {
	deps, err := s.base.DepList(s.aliasToBase(id), direction)
	if err != nil {
		return nil, err
	}
	out := make([]beads.Dep, 0, len(deps))
	for _, dep := range deps {
		out = append(out, s.depToAlias(dep))
	}
	return out, nil
}

func (s *prefixedAliasStore) ListByMetadata(filters map[string]string, limit int, opts ...beads.QueryOpt) ([]beads.Bead, error) {
	result, err := s.base.ListByMetadata(filters, limit, opts...)
	if err != nil {
		return nil, err
	}
	out := make([]beads.Bead, 0, len(result))
	for _, b := range result {
		out = append(out, s.beadToAlias(b))
	}
	return out, nil
}

func (s *prefixedAliasStore) Delete(id string) error {
	return s.base.Delete(s.aliasToBase(id))
}

func configureBeadRouteState(t *testing.T) (*fakeState, *prefixedAliasStore, *prefixedAliasStore) {
	t.Helper()

	state := newFakeState(t)
	state.cityPath = t.TempDir()
	state.cfg.Rigs = []config.Rig{
		{Name: "alpha", Path: "rigs/alpha"},
		{Name: "beta", Path: "rigs/beta"},
	}

	alphaStore := newPrefixedAliasStore("ga")
	betaStore := newPrefixedAliasStore("gb")
	state.stores = map[string]beads.Store{
		"alpha": alphaStore,
		"beta":  betaStore,
	}

	alphaPath := filepath.Join(state.cityPath, "rigs", "alpha")
	betaPath := filepath.Join(state.cityPath, "rigs", "beta")
	if err := os.MkdirAll(filepath.Join(alphaPath, ".beads"), 0o700); err != nil {
		t.Fatalf("MkdirAll(alpha .beads): %v", err)
	}
	if err := os.MkdirAll(betaPath, 0o755); err != nil {
		t.Fatalf("MkdirAll(beta): %v", err)
	}
	routes := `{"prefix":"ga","path":"."}` + "\n" + `{"prefix":"gb","path":"../beta"}`
	if err := os.WriteFile(filepath.Join(alphaPath, ".beads", "routes.jsonl"), []byte(routes), 0o644); err != nil {
		t.Fatalf("WriteFile(routes.jsonl): %v", err)
	}

	return state, alphaStore, betaStore
}

// --- WS-based bead tests ---

func TestBeadCRUD(t *testing.T) {
	state := newFakeState(t)
	conn := connectWS(t, state)

	// Create a bead.
	resp := wsRoundTrip(t, conn, wsRequestEnvelope{
		Type:   "request",
		ID:     "create-1",
		Action: "bead.create",
		Payload: map[string]any{
			"rig":   "myrig",
			"title": "Fix login bug",
			"type":  "task",
		},
	})
	if resp.Type != "response" {
		t.Fatalf("create type = %q, want response", resp.Type)
	}

	var created beads.Bead
	if err := json.Unmarshal(resp.Result, &created); err != nil {
		t.Fatalf("unmarshal created: %v", err)
	}
	if created.Title != "Fix login bug" {
		t.Errorf("Title = %q, want %q", created.Title, "Fix login bug")
	}
	if created.ID == "" {
		t.Fatal("created bead has no ID")
	}

	// Get the bead.
	resp = wsRoundTrip(t, conn, wsRequestEnvelope{
		Type:   "request",
		ID:     "get-1",
		Action: "bead.get",
		Payload: map[string]any{
			"id": created.ID,
		},
	})

	var got beads.Bead
	if err := json.Unmarshal(resp.Result, &got); err != nil {
		t.Fatalf("unmarshal got: %v", err)
	}
	if got.Title != "Fix login bug" {
		t.Errorf("Title = %q, want %q", got.Title, "Fix login bug")
	}

	// Close the bead.
	resp = wsRoundTrip(t, conn, wsRequestEnvelope{
		Type:   "request",
		ID:     "close-1",
		Action: "bead.close",
		Payload: map[string]any{
			"id": created.ID,
		},
	})
	if resp.Type != "response" {
		t.Fatalf("close type = %q, want response", resp.Type)
	}

	// Verify closed.
	resp = wsRoundTrip(t, conn, wsRequestEnvelope{
		Type:   "request",
		ID:     "get-2",
		Action: "bead.get",
		Payload: map[string]any{
			"id": created.ID,
		},
	})
	if err := json.Unmarshal(resp.Result, &got); err != nil {
		t.Fatalf("unmarshal got: %v", err)
	}
	if got.Status != "closed" {
		t.Errorf("Status = %q, want %q", got.Status, "closed")
	}
}

func TestBeadListFiltering(t *testing.T) {
	state := newFakeState(t)
	store := state.stores["myrig"]
	store.Create(beads.Bead{Title: "Open task", Type: "task"})                           //nolint:errcheck
	store.Create(beads.Bead{Title: "Message", Type: "message"})                          //nolint:errcheck
	store.Create(beads.Bead{Title: "Labeled", Type: "task", Labels: []string{"urgent"}}) //nolint:errcheck

	conn := connectWS(t, state)

	// Filter by type.
	resp := wsRoundTrip(t, conn, wsRequestEnvelope{
		Type:   "request",
		ID:     "list-type",
		Action: "beads.list",
		Payload: map[string]any{
			"type": "message",
		},
	})

	var listResp struct {
		Items []beads.Bead `json:"items"`
		Total int          `json:"total"`
	}
	if err := json.Unmarshal(resp.Result, &listResp); err != nil {
		t.Fatalf("unmarshal list: %v", err)
	}
	if listResp.Total != 1 {
		t.Errorf("type filter: Total = %d, want 1", listResp.Total)
	}

	// Filter by label.
	resp = wsRoundTrip(t, conn, wsRequestEnvelope{
		Type:   "request",
		ID:     "list-label",
		Action: "beads.list",
		Payload: map[string]any{
			"label": "urgent",
		},
	})

	if err := json.Unmarshal(resp.Result, &listResp); err != nil {
		t.Fatalf("unmarshal list: %v", err)
	}
	if listResp.Total != 1 {
		t.Errorf("label filter: Total = %d, want 1", listResp.Total)
	}
}

func TestBeadListCrossRig(t *testing.T) {
	state := newFakeState(t)
	store2 := beads.NewMemStore()
	state.stores["rig2"] = store2

	state.stores["myrig"].Create(beads.Bead{Title: "Bead from rig1"}) //nolint:errcheck
	store2.Create(beads.Bead{Title: "Bead from rig2"})                //nolint:errcheck

	conn := connectWS(t, state)

	resp := wsRoundTrip(t, conn, wsRequestEnvelope{
		Type:   "request",
		ID:     "list-cross",
		Action: "beads.list",
	})

	var listResp struct {
		Items []beads.Bead `json:"items"`
		Total int          `json:"total"`
	}
	if err := json.Unmarshal(resp.Result, &listResp); err != nil {
		t.Fatalf("unmarshal list: %v", err)
	}
	if listResp.Total != 2 {
		t.Errorf("cross-rig: Total = %d, want 2", listResp.Total)
	}
}

func TestBeadGetNotFound(t *testing.T) {
	state := newFakeState(t)
	conn := connectWS(t, state)

	raw := wsRoundTripRaw(t, conn, wsRequestEnvelope{
		Type:   "request",
		ID:     "get-nf",
		Action: "bead.get",
		Payload: map[string]any{
			"id": "nonexistent",
		},
	})

	var errResp wsErrorEnvelope
	if err := json.Unmarshal(raw, &errResp); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if errResp.Type != "error" {
		t.Fatalf("type = %q, want error", errResp.Type)
	}
	if errResp.Code != "not_found" {
		t.Errorf("code = %q, want not_found", errResp.Code)
	}
}

func TestBeadGetUsesRoutePrefixStore(t *testing.T) {
	state, alphaStore, betaStore := configureBeadRouteState(t)
	created, err := betaStore.Create(beads.Bead{Title: "Routed beta bead"})
	if err != nil {
		t.Fatalf("Create(beta): %v", err)
	}

	conn := connectWS(t, state)

	resp := wsRoundTrip(t, conn, wsRequestEnvelope{
		Type:   "request",
		ID:     "get-route",
		Action: "bead.get",
		Payload: map[string]any{
			"id": created.ID,
		},
	})

	var got beads.Bead
	if err := json.Unmarshal(resp.Result, &got); err != nil {
		t.Fatalf("Decode(): %v", err)
	}
	if got.Title != "Routed beta bead" {
		t.Fatalf("Title = %q, want %q", got.Title, "Routed beta bead")
	}
	if alphaStore.getCalls != 0 {
		t.Fatalf("alphaStore.getCalls = %d, want 0", alphaStore.getCalls)
	}
	if betaStore.getCalls != 1 {
		t.Fatalf("betaStore.getCalls = %d, want 1", betaStore.getCalls)
	}
}

func TestBeadReady(t *testing.T) {
	state := newFakeState(t)
	store := state.stores["myrig"]
	store.Create(beads.Bead{Title: "Open"}) //nolint:errcheck
	b2, _ := store.Create(beads.Bead{Title: "Closed"})
	store.Close(b2.ID) //nolint:errcheck

	conn := connectWS(t, state)

	resp := wsRoundTrip(t, conn, wsRequestEnvelope{
		Type:   "request",
		ID:     "ready-1",
		Action: "beads.ready",
	})

	var listResp struct {
		Items []beads.Bead `json:"items"`
		Total int          `json:"total"`
	}
	if err := json.Unmarshal(resp.Result, &listResp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if listResp.Total != 1 {
		t.Errorf("ready: Total = %d, want 1", listResp.Total)
	}
}

func TestBeadUpdate(t *testing.T) {
	state := newFakeState(t)
	store := state.stores["myrig"]
	b, _ := store.Create(beads.Bead{Title: "Test"})

	conn := connectWS(t, state)

	desc := "updated description"
	resp := wsRoundTrip(t, conn, wsRequestEnvelope{
		Type:   "request",
		ID:     "update-1",
		Action: "bead.update",
		Payload: map[string]any{
			"id":          b.ID,
			"description": desc,
			"labels":      []string{"new-label"},
		},
	})
	if resp.Type != "response" {
		t.Fatalf("update type = %q, want response", resp.Type)
	}

	// Verify update.
	got, _ := store.Get(b.ID)
	if got.Description != desc {
		t.Errorf("Description = %q, want %q", got.Description, desc)
	}
	if len(got.Labels) != 1 || got.Labels[0] != "new-label" {
		t.Errorf("Labels = %v, want [new-label]", got.Labels)
	}
}

func TestBeadUpdateUsesRoutePrefixStore(t *testing.T) {
	state, alphaStore, betaStore := configureBeadRouteState(t)
	created, err := betaStore.Create(beads.Bead{Title: "Routed beta bead"})
	if err != nil {
		t.Fatalf("Create(beta): %v", err)
	}

	conn := connectWS(t, state)

	resp := wsRoundTrip(t, conn, wsRequestEnvelope{
		Type:   "request",
		ID:     "update-route",
		Action: "bead.update",
		Payload: map[string]any{
			"id":          created.ID,
			"description": "updated via route",
		},
	})
	if resp.Type != "response" {
		t.Fatalf("update type = %q, want response", resp.Type)
	}

	got, err := betaStore.Get(created.ID)
	if err != nil {
		t.Fatalf("Get(beta): %v", err)
	}
	if got.Description != "updated via route" {
		t.Fatalf("Description = %q, want %q", got.Description, "updated via route")
	}
	if alphaStore.updateCalls != 0 {
		t.Fatalf("alphaStore.updateCalls = %d, want 0", alphaStore.updateCalls)
	}
	if betaStore.updateCalls != 1 {
		t.Fatalf("betaStore.updateCalls = %d, want 1", betaStore.updateCalls)
	}
}

func TestBeadDepsUsesRoutePrefixStore(t *testing.T) {
	state, alphaStore, betaStore := configureBeadRouteState(t)
	parent, err := betaStore.Create(beads.Bead{Title: "Parent"})
	if err != nil {
		t.Fatalf("Create(parent): %v", err)
	}
	child, err := betaStore.Create(beads.Bead{Title: "Child", ParentID: parent.ID})
	if err != nil {
		t.Fatalf("Create(child): %v", err)
	}

	conn := connectWS(t, state)

	resp := wsRoundTrip(t, conn, wsRequestEnvelope{
		Type:   "request",
		ID:     "deps-route",
		Action: "bead.deps",
		Payload: map[string]any{
			"id": parent.ID,
		},
	})

	var depsResp struct {
		Children []beads.Bead `json:"children"`
	}
	if err := json.Unmarshal(resp.Result, &depsResp); err != nil {
		t.Fatalf("Decode(): %v", err)
	}
	if len(depsResp.Children) != 1 || depsResp.Children[0].ID != child.ID {
		t.Fatalf("children = %#v, want [%s]", depsResp.Children, child.ID)
	}
	if alphaStore.childrenCalls != 0 {
		t.Fatalf("alphaStore.childrenCalls = %d, want 0", alphaStore.childrenCalls)
	}
	if betaStore.childrenCalls != 1 {
		t.Fatalf("betaStore.childrenCalls = %d, want 1", betaStore.childrenCalls)
	}
}

func TestBeadDepsIncludesMetadataAttachments(t *testing.T) {
	state, _, betaStore := configureBeadRouteState(t)
	parent, err := betaStore.Create(beads.Bead{Title: "Parent"})
	if err != nil {
		t.Fatalf("Create(parent): %v", err)
	}
	attached, err := betaStore.Create(beads.Bead{Title: "Attached", Type: "molecule"})
	if err != nil {
		t.Fatalf("Create(attached): %v", err)
	}
	if err := betaStore.SetMetadata(parent.ID, "molecule_id", attached.ID); err != nil {
		t.Fatalf("SetMetadata(molecule_id): %v", err)
	}

	conn := connectWS(t, state)

	resp := wsRoundTrip(t, conn, wsRequestEnvelope{
		Type:   "request",
		ID:     "deps-meta",
		Action: "bead.deps",
		Payload: map[string]any{
			"id": parent.ID,
		},
	})

	var depsResp struct {
		Children []beads.Bead `json:"children"`
	}
	if err := json.Unmarshal(resp.Result, &depsResp); err != nil {
		t.Fatalf("Decode(): %v", err)
	}
	if len(depsResp.Children) != 1 || depsResp.Children[0].ID != attached.ID {
		t.Fatalf("children = %#v, want [%s]", depsResp.Children, attached.ID)
	}
	if betaStore.getCalls < 2 {
		t.Fatalf("betaStore.getCalls = %d, want at least 2 (parent + attachment)", betaStore.getCalls)
	}
}

func TestBeadPatchAlias(t *testing.T) {
	state := newFakeState(t)
	store := state.stores["myrig"]
	b, _ := store.Create(beads.Bead{Title: "Test"})

	conn := connectWS(t, state)

	desc := "patched"
	resp := wsRoundTrip(t, conn, wsRequestEnvelope{
		Type:   "request",
		ID:     "patch-1",
		Action: "bead.update",
		Payload: map[string]any{
			"id":          b.ID,
			"description": desc,
		},
	})
	if resp.Type != "response" {
		t.Fatalf("PATCH type = %q, want response", resp.Type)
	}

	got, _ := store.Get(b.ID)
	if got.Description != desc {
		t.Errorf("Description = %q, want %q", got.Description, desc)
	}
}

func TestBeadUpdatePriority(t *testing.T) {
	state := newFakeState(t)
	store := state.stores["myrig"]
	b, _ := store.Create(beads.Bead{Title: "Test"})

	conn := connectWS(t, state)

	resp := wsRoundTrip(t, conn, wsRequestEnvelope{
		Type:   "request",
		ID:     "update-pri",
		Action: "bead.update",
		Payload: map[string]any{
			"id":       b.ID,
			"priority": 1,
		},
	})
	if resp.Type != "response" {
		t.Fatalf("update type = %q, want response", resp.Type)
	}

	got, _ := store.Get(b.ID)
	if got.Priority == nil || *got.Priority != 1 {
		t.Fatalf("Priority = %v, want 1", got.Priority)
	}
}

func TestBeadUpdateRejectsNullPriority(t *testing.T) {
	state := newFakeState(t)
	store := state.stores["myrig"]
	priority := 1
	b, _ := store.Create(beads.Bead{Title: "Test", Priority: &priority})

	conn := connectWS(t, state)

	raw := wsRoundTripRaw(t, conn, wsRequestEnvelope{
		Type:   "request",
		ID:     "update-null-pri",
		Action: "bead.update",
		Payload: map[string]any{
			"id":       b.ID,
			"priority": nil,
		},
	})

	var errResp wsErrorEnvelope
	if err := json.Unmarshal(raw, &errResp); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if errResp.Type != "error" {
		t.Fatalf("type = %q, want error", errResp.Type)
	}

	got, _ := store.Get(b.ID)
	if got.Priority == nil || *got.Priority != 1 {
		t.Fatalf("Priority = %v, want unchanged 1", got.Priority)
	}
}

func TestBeadReopen(t *testing.T) {
	state := newFakeState(t)
	store := state.stores["myrig"]
	b, _ := store.Create(beads.Bead{Title: "Closed task"})
	store.Close(b.ID) //nolint:errcheck

	conn := connectWS(t, state)

	// Reopen the closed bead.
	resp := wsRoundTrip(t, conn, wsRequestEnvelope{
		Type:   "request",
		ID:     "reopen-1",
		Action: "bead.reopen",
		Payload: map[string]any{
			"id": b.ID,
		},
	})
	if resp.Type != "response" {
		t.Fatalf("reopen type = %q, want response", resp.Type)
	}

	// Verify reopened.
	got, _ := store.Get(b.ID)
	if got.Status != "open" {
		t.Errorf("Status = %q, want %q", got.Status, "open")
	}
}

func TestBeadReopenNotClosed(t *testing.T) {
	state := newFakeState(t)
	store := state.stores["myrig"]
	b, _ := store.Create(beads.Bead{Title: "Open task"})

	conn := connectWS(t, state)

	raw := wsRoundTripRaw(t, conn, wsRequestEnvelope{
		Type:   "request",
		ID:     "reopen-nc",
		Action: "bead.reopen",
		Payload: map[string]any{
			"id": b.ID,
		},
	})

	var errResp wsErrorEnvelope
	if err := json.Unmarshal(raw, &errResp); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if errResp.Type != "error" {
		t.Fatalf("type = %q, want error", errResp.Type)
	}
	if errResp.Code != "conflict" {
		t.Errorf("code = %q, want conflict", errResp.Code)
	}
}

func TestBeadAssign(t *testing.T) {
	state := newFakeState(t)
	store := state.stores["myrig"]
	b, _ := store.Create(beads.Bead{Title: "Task"})

	conn := connectWS(t, state)

	resp := wsRoundTrip(t, conn, wsRequestEnvelope{
		Type:   "request",
		ID:     "assign-1",
		Action: "bead.assign",
		Payload: map[string]any{
			"id":       b.ID,
			"assignee": "worker-1",
		},
	})
	if resp.Type != "response" {
		t.Fatalf("assign type = %q, want response", resp.Type)
	}

	got, _ := store.Get(b.ID)
	if got.Assignee != "worker-1" {
		t.Errorf("Assignee = %q, want %q", got.Assignee, "worker-1")
	}
}

func TestBeadDelete(t *testing.T) {
	state := newFakeState(t)
	store := state.stores["myrig"]
	b, _ := store.Create(beads.Bead{Title: "To delete"})

	conn := connectWS(t, state)

	resp := wsRoundTrip(t, conn, wsRequestEnvelope{
		Type:   "request",
		ID:     "delete-1",
		Action: "bead.delete",
		Payload: map[string]any{
			"id": b.ID,
		},
	})
	if resp.Type != "response" {
		t.Fatalf("delete type = %q, want response", resp.Type)
	}

	// Verify closed (soft delete).
	got, _ := store.Get(b.ID)
	if got.Status != "closed" {
		t.Errorf("Status = %q, want %q", got.Status, "closed")
	}
}

func TestBeadDeleteNotFound(t *testing.T) {
	state := newFakeState(t)
	conn := connectWS(t, state)

	raw := wsRoundTripRaw(t, conn, wsRequestEnvelope{
		Type:   "request",
		ID:     "delete-nf",
		Action: "bead.delete",
		Payload: map[string]any{
			"id": "nonexistent",
		},
	})

	var errResp wsErrorEnvelope
	if err := json.Unmarshal(raw, &errResp); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if errResp.Type != "error" {
		t.Fatalf("type = %q, want error", errResp.Type)
	}
	if errResp.Code != "not_found" {
		t.Errorf("code = %q, want not_found", errResp.Code)
	}
}

func TestBeadCreateValidation(t *testing.T) {
	state := newFakeState(t)
	conn := connectWS(t, state)

	// Missing title.
	raw := wsRoundTripRaw(t, conn, wsRequestEnvelope{
		Type:   "request",
		ID:     "create-val",
		Action: "bead.create",
		Payload: map[string]any{
			"rig": "myrig",
		},
	})

	var errResp wsErrorEnvelope
	if err := json.Unmarshal(raw, &errResp); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if errResp.Type != "error" {
		t.Fatalf("type = %q, want error", errResp.Type)
	}
}

func TestPackList(t *testing.T) {
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

	writeWSJSON(t, conn, wsRequestEnvelope{Type: "request", ID: "pack-list", Action: "packs.list"})
	var wsResp wsResponseEnvelope
	readWSJSON(t, conn, &wsResp)
	if wsResp.Type != "response" {
		t.Fatalf("type = %q, want response", wsResp.Type)
	}
	var result struct {
		Packs []packResponse `json:"packs"`
	}
	json.Unmarshal(wsResp.Result, &result) //nolint:errcheck
	if len(result.Packs) != 1 {
		t.Fatalf("packs count = %d, want 1", len(result.Packs))
	}
	if result.Packs[0].Name != "gastown" {
		t.Errorf("Name = %q, want %q", result.Packs[0].Name, "gastown")
	}
}

func TestPackListEmpty(t *testing.T) {
	state := newFakeState(t)
	srv := New(state)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()
	conn := dialWebSocket(t, ts.URL+"/v0/ws")
	defer conn.Close()
	drainWSHello(t, conn)

	writeWSJSON(t, conn, wsRequestEnvelope{Type: "request", ID: "pack-empty", Action: "packs.list"})
	var wsResp wsResponseEnvelope
	readWSJSON(t, conn, &wsResp)
	var result struct {
		Packs []packResponse `json:"packs"`
	}
	json.Unmarshal(wsResp.Result, &result) //nolint:errcheck
	if len(result.Packs) != 0 {
		t.Errorf("packs count = %d, want 0", len(result.Packs))
	}
}
