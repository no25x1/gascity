package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/gastownhall/gascity/internal/sessionlog"
)

// handleSessionAgentList returns subagent mappings for a session.
//
//	GET /v0/session/{id}/agents
//	Response: { "agents": [{ "agent_id": "...", "parent_tool_use_id": "..." }] }
func (s *Server) handleSessionAgentList(w http.ResponseWriter, r *http.Request) {
	store := s.state.CityBeadStore()
	if store == nil {
		writeError(w, http.StatusServiceUnavailable, "unavailable", "no bead store configured")
		return
	}

	id, err := s.resolveSessionIDAllowClosedWithConfig(store, r.PathValue("id"))
	if err != nil {
		writeResolveError(w, err)
		return
	}

	mgr := s.sessionManager(store)
	logPath, err := mgr.TranscriptPath(id, s.sessionLogPaths())
	if err != nil {
		writeSessionManagerError(w, err)
		return
	}
	if logPath == "" {
		writeJSON(w, http.StatusOK, map[string]any{"agents": []any{}})
		return
	}

	mappings, err := sessionlog.FindAgentMappings(logPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "failed to list agents")
		return
	}
	if mappings == nil {
		mappings = []sessionlog.AgentMapping{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"agents": mappings})
}

// handleSessionAgentGet returns the transcript and status of a subagent.
//
//	GET /v0/session/{id}/agents/{agentId}
//	Response: { "messages": [...], "status": "completed|running|pending|failed" }
func (s *Server) handleSessionAgentGet(w http.ResponseWriter, r *http.Request) {
	store := s.state.CityBeadStore()
	if store == nil {
		writeError(w, http.StatusServiceUnavailable, "unavailable", "no bead store configured")
		return
	}

	id, err := s.resolveSessionIDAllowClosedWithConfig(store, r.PathValue("id"))
	if err != nil {
		writeResolveError(w, err)
		return
	}

	agentID := r.PathValue("agentId")
	if agentID == "" {
		writeError(w, http.StatusBadRequest, "invalid", "agentId is required")
		return
	}

	if err := sessionlog.ValidateAgentID(agentID); err != nil {
		writeError(w, http.StatusBadRequest, "invalid", err.Error())
		return
	}

	mgr := s.sessionManager(store)
	logPath, err := mgr.TranscriptPath(id, s.sessionLogPaths())
	if err != nil {
		writeSessionManagerError(w, err)
		return
	}
	if logPath == "" {
		writeError(w, http.StatusNotFound, "not_found", "no transcript found for session "+id)
		return
	}

	agentSession, err := sessionlog.ReadAgentSession(logPath, agentID)
	if err != nil {
		if errors.Is(err, sessionlog.ErrAgentNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "agent not found")
		} else {
			writeError(w, http.StatusInternalServerError, "internal", "failed to read agent transcript")
		}
		return
	}

	// Build raw message array for API pass-through (same as raw transcript).
	rawMessages := make([]json.RawMessage, 0, len(agentSession.Messages))
	for _, entry := range agentSession.Messages {
		if len(entry.Raw) > 0 {
			rawMessages = append(rawMessages, entry.Raw)
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"messages": rawMessages,
		"status":   agentSession.Status,
	})
}

// --- Shared methods for WS dispatch ---

func (s *Server) listSessionAgents(target string) (any, error) {
	store := s.state.CityBeadStore()
	if store == nil {
		return nil, httpError{status: 503, code: "unavailable", message: "no bead store configured"}
	}
	id, err := s.resolveSessionIDAllowClosedWithConfig(store, target)
	if err != nil {
		return nil, err
	}
	mgr := s.sessionManager(store)
	logPath, err := mgr.TranscriptPath(id, s.sessionLogPaths())
	if err != nil {
		return nil, err
	}
	if logPath == "" {
		return map[string]any{"agents": []any{}}, nil
	}
	mappings, err := sessionlog.FindAgentMappings(logPath)
	if err != nil {
		return nil, httpError{status: 500, code: "internal", message: "failed to list agents"}
	}
	if mappings == nil {
		mappings = []sessionlog.AgentMapping{}
	}
	return map[string]any{"agents": mappings}, nil
}

func (s *Server) getSessionAgent(target, agentID string) (any, error) {
	store := s.state.CityBeadStore()
	if store == nil {
		return nil, httpError{status: 503, code: "unavailable", message: "no bead store configured"}
	}
	id, err := s.resolveSessionIDAllowClosedWithConfig(store, target)
	if err != nil {
		return nil, err
	}
	if agentID == "" {
		return nil, httpError{status: 400, code: "invalid", message: "agent_id is required"}
	}
	if err := sessionlog.ValidateAgentID(agentID); err != nil {
		return nil, httpError{status: 400, code: "invalid", message: err.Error()}
	}
	mgr := s.sessionManager(store)
	logPath, err := mgr.TranscriptPath(id, s.sessionLogPaths())
	if err != nil {
		return nil, err
	}
	if logPath == "" {
		return nil, httpError{status: 404, code: "not_found", message: "no transcript found for session " + id}
	}
	agentSession, err := sessionlog.ReadAgentSession(logPath, agentID)
	if err != nil {
		if errors.Is(err, sessionlog.ErrAgentNotFound) {
			return nil, httpError{status: 404, code: "not_found", message: "agent not found"}
		}
		return nil, httpError{status: 500, code: "internal", message: "failed to read agent transcript"}
	}
	rawMessages := make([]json.RawMessage, 0, len(agentSession.Messages))
	for _, entry := range agentSession.Messages {
		if len(entry.Raw) > 0 {
			rawMessages = append(rawMessages, entry.Raw)
		}
	}
	return map[string]any{"messages": rawMessages, "status": agentSession.Status}, nil
}
