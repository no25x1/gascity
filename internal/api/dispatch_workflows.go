package api

import (
	"context"
	"encoding/json"
)

type socketWorkflowGetPayload struct {
	ID        string `json:"id"`
	ScopeKind string `json:"scope_kind,omitempty"`
	ScopeRef  string `json:"scope_ref,omitempty"`
}

type socketWorkflowDeletePayload struct {
	ID        string `json:"id"`
	ScopeKind string `json:"scope_kind,omitempty"`
	ScopeRef  string `json:"scope_ref,omitempty"`
	Delete    bool   `json:"delete,omitempty"`
}

func init() {
	// workflow.get needs the dispatch index for snapshot consistency.
	// Uses raw actionHandler to access req.dispatchIndex directly.
	registerRawAction("workflow.get", ActionDef{
		Description:       "Get workflow snapshot",
		RequiresCityScope: true,
	}, func(s *Server, req *socketRequestEnvelope) (socketActionResult, *socketErrorEnvelope) {
		var payload socketWorkflowGetPayload
		if len(req.Payload) > 0 {
			if err := json.Unmarshal(req.Payload, &payload); err != nil {
				return socketActionResult{}, newSocketError(req.ID, "invalid", err.Error())
			}
		}
		if payload.ID == "" {
			return socketActionResult{}, newSocketError(req.ID, "invalid", "id is required")
		}
		if payload.ScopeKind != "" && payload.ScopeKind != "rig" && payload.ScopeKind != "city" {
			return socketActionResult{}, newSocketError(req.ID, "invalid", "scope_kind must be 'rig' or 'city'")
		}
		snapshot, err := s.buildWorkflowSnapshot(payload.ID, payload.ScopeKind, payload.ScopeRef, req.dispatchIndex)
		if err != nil {
			return socketActionResult{}, socketErrorFor(req.ID, err)
		}
		return socketActionResult{Result: snapshot}, nil
	})

	RegisterAction("workflow.delete", ActionDef{
		Description:       "Delete a workflow",
		IsMutation:        true,
		RequiresCityScope: true,
	}, func(_ context.Context, s *Server, payload socketWorkflowDeletePayload) (any, error) {
		if payload.ID == "" {
			return nil, httpError{status: 400, code: "invalid", message: "id is required"}
		}
		return s.deleteWorkflow(payload.ID, payload.ScopeKind, payload.ScopeRef, payload.Delete)
	})
}
