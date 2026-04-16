package api

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"strings"
	"time"

	"github.com/gastownhall/gascity/internal/config"
	"github.com/gastownhall/gascity/internal/sessionlog"
	"github.com/gastownhall/gascity/internal/telemetry"
	workdirutil "github.com/gastownhall/gascity/internal/workdir"
)

// outputTurn is a single conversation turn in the unified output response.
type outputTurn struct {
	Role      string `json:"role"`
	Text      string `json:"text"`
	Timestamp string `json:"timestamp,omitempty"`
}

// agentOutputResponse is the WS replacement payload for the old
// GET /v0/agent/{name}/output endpoint.
type agentOutputResponse struct {
	Agent      string                     `json:"agent"`
	Format     string                     `json:"format"` // "conversation" or "text"
	Turns      []outputTurn               `json:"turns"`
	Pagination *sessionlog.PaginationInfo `json:"pagination,omitempty"`
}

type agentOutputQuery struct {
	Tail   int
	Before string
}

// entryToTurn converts a sessionlog Entry to a human-readable outputTurn.
func entryToTurn(e *sessionlog.Entry) outputTurn {
	turn := outputTurn{
		Role: e.Type,
	}
	if !e.Timestamp.IsZero() {
		turn.Timestamp = e.Timestamp.Format("2006-01-02T15:04:05Z07:00")
	}

	// Try plain string content (message is a JSON object with string content).
	if text := e.TextContent(); text != "" {
		turn.Text = text
		return turn
	}

	// Try structured content blocks — extract human-readable text.
	if blocks := e.ContentBlocks(); len(blocks) > 0 {
		var parts []string
		for _, b := range blocks {
			switch b.Type {
			case "text":
				if b.Text != "" {
					parts = append(parts, b.Text)
				}
			case "tool_use":
				if b.Name != "" {
					parts = append(parts, "["+b.Name+"]")
				}
			case "tool_result":
				text := extractToolResultText(b.Content)
				if text != "" {
					if len(text) > 500 {
						text = text[:500] + "…"
					}
					parts = append(parts, "[result] "+text)
				}
			case "thinking":
				// Redact thinking blocks — internal model reasoning
				// should not be surfaced to the UI.
				parts = append(parts, "[thinking]")
			}
		}
		turn.Text = strings.Join(parts, "\n")
		return turn
	}

	// Claude JSONL double-encodes the message field as a JSON string
	// containing JSON. Unwrap and try again.
	turn.Text = unwrapDoubleEncoded(e.Message)
	return turn
}

// extractToolResultText extracts human-readable text from a tool_result
// Content field (json.RawMessage). The content can be a plain string or
// an array of content blocks (e.g., [{type:"text", text:"..."}]).
func extractToolResultText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	// Try plain string.
	var s string
	if json.Unmarshal(raw, &s) == nil && s != "" {
		return s
	}
	// Try array of content blocks.
	var blocks []sessionlog.ContentBlock
	if json.Unmarshal(raw, &blocks) == nil {
		var parts []string
		for _, b := range blocks {
			if b.Type == "text" && b.Text != "" {
				parts = append(parts, b.Text)
			}
		}
		return strings.Join(parts, "\n")
	}
	return ""
}

// outputStreamPollInterval controls how often stream implementations check for
// new output. Used by the log file watcher and session stream emitters.
const outputStreamPollInterval = 2 * time.Second

func normalizeAgentOutputQuery(tail *int, before string) agentOutputQuery {
	query := agentOutputQuery{
		Tail:   1,
		Before: before,
	}
	if tail != nil && *tail >= 0 {
		query.Tail = *tail
	}
	return query
}

func (s *Server) getAgentOutput(name string, query agentOutputQuery) (agentOutputResponse, error) {
	cfg := s.state.Config()
	agentCfg, ok := findAgent(cfg, name)
	if !ok {
		return agentOutputResponse{}, httpError{status: 404, code: "not_found", message: "agent " + name + " not found"}
	}

	resp, err := s.trySessionLogOutput(query, name, agentCfg)
	if err != nil {
		return agentOutputResponse{}, httpError{status: 500, code: "internal", message: "reading session log: " + err.Error()}
	}
	if resp != nil {
		return *resp, nil
	}

	return s.peekFallbackOutput(name, cfg)
}

// trySessionLogOutput attempts to read structured conversation data from an
// agent transcript file. It returns (nil, nil) when no transcript exists.
func (s *Server) trySessionLogOutput(query agentOutputQuery, name string, agentCfg config.Agent) (*agentOutputResponse, error) {
	cfg := s.state.Config()
	workDir := s.resolveAgentWorkDir(agentCfg, name)
	if workDir == "" {
		return nil, nil
	}
	provider := strings.TrimSpace(agentCfg.Provider)
	if provider == "" && cfg != nil {
		provider = strings.TrimSpace(cfg.Workspace.Provider)
	}

	path := sessionlog.FindSessionFileForProvider(s.sessionLogPaths(), provider, workDir)
	if path == "" {
		return nil, nil
	}

	var sess *sessionlog.Session
	var err error
	if query.Before != "" {
		sess, err = sessionlog.ReadProviderFileOlder(provider, path, query.Tail, query.Before)
	} else {
		sess, err = sessionlog.ReadProviderFile(provider, path, query.Tail)
	}
	if err != nil {
		return nil, err
	}

	turns := make([]outputTurn, 0, len(sess.Messages))
	for _, e := range sess.Messages {
		turn := entryToTurn(e)
		if turn.Text == "" {
			continue
		}
		turns = append(turns, turn)
	}

	return &agentOutputResponse{
		Agent:      name,
		Format:     "conversation",
		Turns:      turns,
		Pagination: sess.Pagination,
	}, nil
}

func (s *Server) peekFallbackOutput(name string, cfg *config.City) (agentOutputResponse, error) {
	sp := s.state.SessionProvider()
	sessionName := agentSessionName(s.state.CityName(), name, cfg.Workspace.SessionTemplate)

	if !sp.IsRunning(sessionName) {
		return agentOutputResponse{}, httpError{status: 404, code: "not_found", message: "agent " + name + " not running"}
	}

	output, err := sp.Peek(sessionName, 100)
	if err != nil {
		return agentOutputResponse{}, httpError{status: 500, code: "internal", message: err.Error()}
	}

	turns := []outputTurn{}
	if output != "" {
		turns = append(turns, outputTurn{Role: "output", Text: output})
	}

	return agentOutputResponse{
		Agent:  name,
		Format: "text",
		Turns:  turns,
	}, nil
}

func (s *Server) resolveAgentWorkDir(a config.Agent, qualifiedName string) string {
	cfg := s.state.Config()
	return workdirutil.ResolveWorkDirPath(
		s.state.CityPath(),
		workdirutil.CityName(s.state.CityPath(), cfg),
		qualifiedName,
		a,
		cfg.Rigs,
	)
}

func (s *Server) startAgentOutputStreamSubscription(parent context.Context, sess *socketSession, req *socketRequestEnvelope, payload AgentOutputStreamSubscriptionPayload) (socketActionResult, *socketErrorEnvelope) {
	cfg := s.state.Config()
	agentCfg, ok := findAgent(cfg, payload.Target)
	if !ok {
		return socketActionResult{}, newSocketError(req.ID, "not_found", "agent "+payload.Target+" not found")
	}
	workDir := s.resolveAgentWorkDir(agentCfg, payload.Target)
	provider := strings.TrimSpace(agentCfg.Provider)
	if provider == "" && cfg != nil {
		provider = strings.TrimSpace(cfg.Workspace.Provider)
	}
	logPath := ""
	if workDir != "" {
		logPath = sessionlog.FindSessionFileForProvider(s.sessionLogPaths(), provider, workDir)
	}

	sp := s.state.SessionProvider()
	sessionName := agentSessionName(s.state.CityName(), payload.Target, cfg.Workspace.SessionTemplate)
	running := sp.IsRunning(sessionName)
	if logPath == "" && !running {
		return socketActionResult{}, newSocketError(req.ID, "not_found", "agent "+payload.Target+" not running")
	}

	subID := sess.newSubscriptionID()
	subCtx, cancel := context.WithCancel(parent)
	sess.registerSubscription(subID, cancel)
	log.Printf("api: ws subscription started id=%s kind=%s target=%s", subID, payload.Kind, payload.Target)
	telemetry.RecordWebSocketSubscription(context.Background(), 1)
	return socketActionResult{
		Result: map[string]string{"subscription_id": subID, "kind": payload.Kind},
		AfterWrite: func() {
			go func() {
				defer cancel()
				defer sess.unregisterSubscription(subID)
				defer log.Printf("api: ws subscription ended id=%s kind=%s target=%s", subID, payload.Kind, payload.Target)
				defer telemetry.RecordWebSocketSubscription(context.Background(), -1)
				emitter := newSocketAgentOutputStreamEmitter(sess, subID)
				switch {
				case logPath != "":
					s.streamAgentOutputLogWithEmitter(subCtx, emitter, payload.Target, provider, logPath, payload.AfterCursor)
				default:
					s.streamAgentPeekOutputWithEmitter(subCtx, emitter, payload.Target, cfg)
				}
			}()
		},
	}, nil
}

func (s *Server) streamAgentOutputLogWithEmitter(ctx context.Context, emitter sessionStreamEmitter, name, provider, logPath, afterCursor string) {
	lw := newLogFileWatcher(logPath)
	defer lw.Close()

	var lastSize int64
	var lastSentUUID string
	var resumeCursor = afterCursor
	var seq uint64
	sentUUIDs := make(map[string]struct{})
	lw.onReset = func() { lastSize = 0 }

	readAndEmit := func() {
		info, err := os.Stat(logPath)
		if err != nil {
			return
		}
		if info.Size() == lastSize {
			return
		}

		sess, err := sessionlog.ReadProviderFile(provider, logPath, 1)
		if err != nil {
			return
		}
		lastSize = info.Size()

		turns := make([]outputTurn, 0, len(sess.Messages))
		uuids := make([]string, 0, len(sess.Messages))
		for _, e := range sess.Messages {
			turn := entryToTurn(e)
			if turn.Text == "" {
				continue
			}
			turns = append(turns, turn)
			uuids = append(uuids, e.UUID)
		}
		if len(turns) == 0 {
			return
		}

		var toSend []outputTurn
		emittedCursor := ""
		if lastSentUUID == "" && resumeCursor == "" {
			toSend = turns
			if len(toSend) > 0 {
				emittedCursor = uuids[len(uuids)-1]
			}
		} else if lastSentUUID == "" {
			found := false
			for i, uuid := range uuids {
				if uuid == resumeCursor {
					toSend = turns[i+1:]
					found = true
					break
				}
			}
			if !found {
				log.Printf("agent stream: cursor %s lost, waiting for new turns", resumeCursor)
			} else if len(toSend) > 0 {
				emittedCursor = uuids[len(uuids)-1]
			}
			resumeCursor = ""
		} else {
			found := false
			for i, uuid := range uuids {
				if uuid == lastSentUUID {
					toSend = turns[i+1:]
					found = true
					break
				}
			}
			if !found {
				log.Printf("agent stream: cursor %s lost, emitting only new turns", lastSentUUID)
				for i, uuid := range uuids {
					if _, seen := sentUUIDs[uuid]; !seen {
						toSend = append(toSend, turns[i])
					}
				}
			}
			if len(toSend) > 0 {
				emittedCursor = uuids[len(uuids)-1]
			}
		}

		if len(toSend) == 0 {
			lastSentUUID = uuids[len(uuids)-1]
			for _, uuid := range uuids {
				sentUUIDs[uuid] = struct{}{}
			}
			return
		}

		seq++
		_ = emitter.emitWithCursor("turn", seq, emittedCursor, agentOutputResponse{
			Agent:  name,
			Format: "conversation",
			Turns:  toSend,
		})
		lastSentUUID = uuids[len(uuids)-1]
		for _, uuid := range uuids {
			sentUUIDs[uuid] = struct{}{}
		}
	}

	lw.Run(ctx, readAndEmit, func() {})
}

func (s *Server) streamAgentPeekOutputWithEmitter(ctx context.Context, emitter sessionStreamEmitter, name string, cfg *config.City) {
	sp := s.state.SessionProvider()
	sessionName := agentSessionName(s.state.CityName(), name, cfg.Workspace.SessionTemplate)

	poll := time.NewTicker(outputStreamPollInterval)
	defer poll.Stop()

	var lastOutput string
	var seq uint64

	emitPeek := func() {
		if !sp.IsRunning(sessionName) {
			return
		}
		output, err := sp.Peek(sessionName, 100)
		if err != nil || output == lastOutput {
			return
		}
		lastOutput = output
		seq++

		turns := []outputTurn{}
		if output != "" {
			turns = append(turns, outputTurn{Role: "output", Text: output})
		}
		_ = emitter.emit("turn", seq, agentOutputResponse{
			Agent:  name,
			Format: "text",
			Turns:  turns,
		})
	}

	emitPeek()
	for {
		select {
		case <-ctx.Done():
			return
		case <-poll.C:
			emitPeek()
		}
	}
}

// unwrapDoubleEncoded handles Claude's double-encoded message format
// where the "message" field is a JSON string containing a JSON object.
// Returns the human-readable content text, or "" if not parseable.
func unwrapDoubleEncoded(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}
	// Try to unwrap: raw might be a JSON string like "{\"role\":...}"
	var inner string
	if err := json.Unmarshal(raw, &inner); err != nil {
		return ""
	}
	// Now inner is the JSON object as a string. Parse it.
	var mc sessionlog.MessageContent
	if err := json.Unmarshal([]byte(inner), &mc); err != nil {
		return ""
	}
	// Try string content.
	var s string
	if err := json.Unmarshal(mc.Content, &s); err == nil && s != "" {
		return s
	}
	// Try array of content blocks.
	var blocks []sessionlog.ContentBlock
	if err := json.Unmarshal(mc.Content, &blocks); err == nil {
		var parts []string
		for _, b := range blocks {
			if b.Type == "text" && b.Text != "" {
				parts = append(parts, b.Text)
			}
		}
		return strings.Join(parts, "\n")
	}
	return ""
}
