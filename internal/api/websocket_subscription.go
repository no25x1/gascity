package api

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/gastownhall/gascity/internal/events"
	"github.com/gastownhall/gascity/internal/runtime"
	"github.com/gastownhall/gascity/internal/session"
	"github.com/gastownhall/gascity/internal/telemetry"
	"github.com/gorilla/websocket"
)

const (
	socketPingInterval = 15 * time.Second
	socketPongWait     = 45 * time.Second

	subscriptionKindEventsStream      = "events.stream"
	subscriptionKindSessionStream     = "session.stream"
	subscriptionKindAgentOutputStream = "agent.output.stream"
)

type subscriptionKindPayload struct {
	Kind string `json:"kind"`
}

// SubscriptionStartPayload is the payload for subscription.start.
type SubscriptionStartPayload struct {
	Kind        string `json:"kind" description:"Subscription type: 'events.stream', 'session.stream', or 'agent.output.stream'"`
	AfterSeq    uint64 `json:"after_seq,omitempty" description:"Resume from this event sequence"`
	AfterCursor string `json:"after_cursor,omitempty" description:"Resume from this cursor"`
	Target      string `json:"target,omitempty" description:"Stream target identifier (session ID/name or agent name)"`
	Format      string `json:"format,omitempty" description:"Stream format: 'text', 'raw', 'jsonl'"`
	Turns       int    `json:"turns,omitempty" description:"Most recent N turns (0=all)"`
}

// EventsStreamSubscriptionPayload is the typed request payload for the
// route-faithful events.stream subscription kind.
type EventsStreamSubscriptionPayload struct {
	Kind        string `json:"kind" description:"Must be 'events.stream'"`
	AfterSeq    uint64 `json:"after_seq,omitempty" description:"Resume from this event sequence"`
	AfterCursor string `json:"after_cursor,omitempty" description:"Resume from this cursor"`
}

// ToSubscriptionStartPayload converts the typed payload to the runtime
// protocol payload for subscription.start.
func (p EventsStreamSubscriptionPayload) ToSubscriptionStartPayload() SubscriptionStartPayload {
	return SubscriptionStartPayload{
		Kind:        subscriptionKindEventsStream,
		AfterSeq:    p.AfterSeq,
		AfterCursor: p.AfterCursor,
	}
}

// SessionStreamSubscriptionPayload is the typed request payload for the
// route-faithful session.stream subscription kind.
type SessionStreamSubscriptionPayload struct {
	Kind        string `json:"kind" description:"Must be 'session.stream'"`
	AfterCursor string `json:"after_cursor,omitempty" description:"Resume from this cursor"`
	Target      string `json:"target" description:"Session ID or session name"`
	Format      string `json:"format,omitempty" description:"Stream format: 'text', 'raw', 'jsonl'"`
	Turns       int    `json:"turns,omitempty" description:"Most recent N turns (0=all)"`
}

// ToSubscriptionStartPayload converts the typed payload to the runtime
// protocol payload for subscription.start.
func (p SessionStreamSubscriptionPayload) ToSubscriptionStartPayload() SubscriptionStartPayload {
	return SubscriptionStartPayload{
		Kind:        subscriptionKindSessionStream,
		AfterCursor: p.AfterCursor,
		Target:      p.Target,
		Format:      p.Format,
		Turns:       p.Turns,
	}
}

// AgentOutputStreamSubscriptionPayload is the typed request payload for the
// route-faithful agent.output.stream subscription kind.
type AgentOutputStreamSubscriptionPayload struct {
	Kind        string `json:"kind" description:"Must be 'agent.output.stream'"`
	AfterCursor string `json:"after_cursor,omitempty" description:"Resume from this cursor"`
	Target      string `json:"target" description:"Agent name"`
}

// ToSubscriptionStartPayload converts the typed payload to the runtime
// protocol payload for subscription.start.
func (p AgentOutputStreamSubscriptionPayload) ToSubscriptionStartPayload() SubscriptionStartPayload {
	return SubscriptionStartPayload{
		Kind:        subscriptionKindAgentOutputStream,
		AfterCursor: p.AfterCursor,
		Target:      p.Target,
	}
}

// SubscriptionStopPayload is the payload for subscription.stop.
type SubscriptionStopPayload struct {
	SubscriptionID string `json:"subscription_id" description:"Subscription to stop"`
}

// EventEnvelope is sent by the server for subscription events.
type EventEnvelope struct {
	Type           string `json:"type" description:"Must be 'event'"`
	SubscriptionID string `json:"subscription_id" description:"Subscription that produced this event"`
	EventType      string `json:"event_type" description:"Event type (e.g. 'bead.created')"`
	Index          uint64 `json:"index,omitempty" description:"Event sequence number"`
	Cursor         string `json:"cursor,omitempty" description:"Resume cursor for reconnection"`
	Payload        any    `json:"payload,omitempty" description:"Event-specific payload"`
}

// EventsStreamPayload is the typed payload emitted by events.stream.
type EventsStreamPayload struct {
	events.Event
	City     string                   `json:"city,omitempty"`
	Workflow *workflowEventProjection `json:"workflow,omitempty"`
}

// EventsStreamEventEnvelope is the typed event envelope for events.stream.
type EventsStreamEventEnvelope struct {
	Type           string              `json:"type" description:"Must be 'event'"`
	SubscriptionID string              `json:"subscription_id" description:"Subscription that produced this event"`
	EventType      string              `json:"event_type" description:"Event type (e.g. 'bead.created')"`
	Index          uint64              `json:"index,omitempty" description:"Event sequence number"`
	Cursor         string              `json:"cursor,omitempty" description:"Resume cursor for reconnection"`
	Payload        EventsStreamPayload `json:"payload" description:"Events stream payload"`
}

// SessionStreamTurnEventEnvelope is the typed event envelope for turn events
// emitted by session.stream.
type SessionStreamTurnEventEnvelope struct {
	Type           string                    `json:"type" description:"Must be 'event'"`
	SubscriptionID string                    `json:"subscription_id" description:"Subscription that produced this event"`
	EventType      string                    `json:"event_type" description:"Must be 'turn'"`
	Index          uint64                    `json:"index,omitempty" description:"Event sequence number"`
	Cursor         string                    `json:"cursor,omitempty" description:"Resume cursor for reconnection"`
	Payload        sessionTranscriptResponse `json:"payload" description:"Session transcript payload"`
}

// SessionStreamMessageEventEnvelope is the typed event envelope for raw message
// events emitted by session.stream.
type SessionStreamMessageEventEnvelope struct {
	Type           string                       `json:"type" description:"Must be 'event'"`
	SubscriptionID string                       `json:"subscription_id" description:"Subscription that produced this event"`
	EventType      string                       `json:"event_type" description:"Must be 'message'"`
	Index          uint64                       `json:"index,omitempty" description:"Event sequence number"`
	Cursor         string                       `json:"cursor,omitempty" description:"Resume cursor for reconnection"`
	Payload        sessionRawTranscriptResponse `json:"payload" description:"Raw session transcript payload"`
}

// StreamActivityPayload is the typed payload emitted for activity updates.
type StreamActivityPayload struct {
	Activity string `json:"activity" description:"Session activity state"`
}

// SessionStreamActivityEventEnvelope is the typed event envelope for activity
// updates emitted by session.stream.
type SessionStreamActivityEventEnvelope struct {
	Type           string                `json:"type" description:"Must be 'event'"`
	SubscriptionID string                `json:"subscription_id" description:"Subscription that produced this event"`
	EventType      string                `json:"event_type" description:"Must be 'activity'"`
	Index          uint64                `json:"index,omitempty" description:"Event sequence number"`
	Payload        StreamActivityPayload `json:"payload" description:"Session activity payload"`
}

// SessionStreamPendingEventEnvelope is the typed event envelope for pending
// interactions emitted by session.stream.
type SessionStreamPendingEventEnvelope struct {
	Type           string                     `json:"type" description:"Must be 'event'"`
	SubscriptionID string                     `json:"subscription_id" description:"Subscription that produced this event"`
	EventType      string                     `json:"event_type" description:"Must be 'pending'"`
	Index          uint64                     `json:"index,omitempty" description:"Event sequence number"`
	Payload        runtime.PendingInteraction `json:"payload" description:"Pending interaction payload"`
}

// AgentOutputStreamTurnEventEnvelope is the typed event envelope for turn
// events emitted by agent.output.stream.
type AgentOutputStreamTurnEventEnvelope struct {
	Type           string              `json:"type" description:"Must be 'event'"`
	SubscriptionID string              `json:"subscription_id" description:"Subscription that produced this event"`
	EventType      string              `json:"event_type" description:"Must be 'turn'"`
	Index          uint64              `json:"index,omitempty" description:"Event sequence number"`
	Cursor         string              `json:"cursor,omitempty" description:"Resume cursor for reconnection"`
	Payload        agentOutputResponse `json:"payload" description:"Agent output payload"`
}

// Backward-compatible aliases.
type socketSubscriptionStartPayload = SubscriptionStartPayload
type socketSubscriptionStopPayload = SubscriptionStopPayload
type socketEventEnvelope = EventEnvelope

type socketSession struct {
	ctx       context.Context
	cancel    context.CancelFunc
	conn      *socketConn
	mu        sync.Mutex
	nextSubID uint64
	subs      map[string]context.CancelFunc
}

func newSocketSession(parent context.Context, conn *socketConn) *socketSession {
	ctx, cancel := context.WithCancel(parent)
	return &socketSession{
		ctx:    ctx,
		cancel: cancel,
		conn:   conn,
		subs:   make(map[string]context.CancelFunc),
	}
}

func (s *socketSession) close() {
	s.cancel()
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, cancel := range s.subs {
		cancel()
		delete(s.subs, id)
	}
}

func (s *socketSession) runPingLoop() {
	ticker := time.NewTicker(socketPingInterval)
	defer ticker.Stop()
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			if err := s.conn.writePing(); err != nil {
				s.cancel()
				return
			}
		}
	}
}

func (s *socketSession) newSubscriptionID() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextSubID++
	return fmt.Sprintf("sub-%d", s.nextSubID)
}

func (s *socketSession) registerSubscription(id string, cancel context.CancelFunc) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.subs[id] = cancel
}

func (s *socketSession) unregisterSubscription(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.subs, id)
}

func (s *socketSession) stopSubscription(id string) bool {
	s.mu.Lock()
	cancel, ok := s.subs[id]
	if ok {
		delete(s.subs, id)
	}
	s.mu.Unlock()
	if ok {
		cancel()
	}
	return ok
}

func (sc *socketConn) writePing() error {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	return sc.conn.WriteControl(websocket.PingMessage, []byte("ping"), time.Now().Add(5*time.Second))
}

func (s *Server) startSocketSubscription(ctx context.Context, sess *socketSession, req *socketRequestEnvelope) (socketActionResult, *socketErrorEnvelope) {
	// Validate city scope on per-city servers.
	if req.Scope != nil && req.Scope.City != "" {
		if cityName := s.state.CityName(); req.Scope.City != cityName {
			return socketActionResult{}, newSocketError(req.ID, "invalid",
				"scope.city "+req.Scope.City+" does not match this city "+cityName)
		}
	}
	var kindPayload subscriptionKindPayload
	if err := decodeSocketPayload(req.Payload, &kindPayload); err != nil {
		return socketActionResult{}, newSocketError(req.ID, "invalid", err.Error())
	}
	switch kindPayload.Kind {
	case subscriptionKindEventsStream:
		var payload EventsStreamSubscriptionPayload
		if err := decodeSocketPayload(req.Payload, &payload); err != nil {
			return socketActionResult{}, newSocketError(req.ID, "invalid", err.Error())
		}
		return s.startEventSubscription(ctx, sess, req, payload)
	case subscriptionKindSessionStream:
		var payload SessionStreamSubscriptionPayload
		if err := decodeSocketPayload(req.Payload, &payload); err != nil {
			return socketActionResult{}, newSocketError(req.ID, "invalid", err.Error())
		}
		return s.startSessionStreamSubscription(ctx, sess, req, payload)
	case subscriptionKindAgentOutputStream:
		var payload AgentOutputStreamSubscriptionPayload
		if err := decodeSocketPayload(req.Payload, &payload); err != nil {
			return socketActionResult{}, newSocketError(req.ID, "invalid", err.Error())
		}
		if payload.Target == "" {
			return socketActionResult{}, newSocketError(req.ID, "invalid", "target is required")
		}
		return s.startAgentOutputStreamSubscription(ctx, sess, req, payload)
	default:
		return socketActionResult{}, newSocketError(req.ID, "not_found", "unknown subscription kind: "+kindPayload.Kind)
	}
}

func (s *Server) stopSocketSubscription(sess *socketSession, req *socketRequestEnvelope) (socketActionResult, *socketErrorEnvelope) {
	return stopSocketSubscriptionImpl(sess, req)
}

func (sm *SupervisorMux) startSocketSubscription(ctx context.Context, sess *socketSession, req *socketRequestEnvelope) (socketActionResult, *socketErrorEnvelope) {
	var kindPayload subscriptionKindPayload
	if err := decodeSocketPayload(req.Payload, &kindPayload); err != nil {
		return socketActionResult{}, newSocketError(req.ID, "invalid", err.Error())
	}
	switch kindPayload.Kind {
	case subscriptionKindEventsStream:
		var payload EventsStreamSubscriptionPayload
		if err := decodeSocketPayload(req.Payload, &payload); err != nil {
			return socketActionResult{}, newSocketError(req.ID, "invalid", err.Error())
		}
		if req.Scope != nil && req.Scope.City != "" {
			cityName, apiErr := sm.resolveSocketCityTarget(req.Scope)
			if apiErr != nil {
				apiErr.ID = req.ID
				return socketActionResult{}, apiErr
			}
			state := sm.resolver.CityState(cityName)
			if state == nil {
				return socketActionResult{}, newSocketError(req.ID, "not_found", "city not found or not running: "+cityName)
			}
			cityReq := *req
			cityReq.Scope = nil
			srv := sm.getCityServer(cityName, state)
			result, apiErr := srv.startSocketSubscription(ctx, sess, &cityReq)
			if apiErr == nil {
				result.AfterWrite = sm.wrapAfterWriteWithCityWatch(ctx, cityName, sess, result)
			}
			return result, apiErr
		}
		return sm.startGlobalEventSubscription(ctx, sess, req, payload)
	case subscriptionKindSessionStream, subscriptionKindAgentOutputStream:
		switch kindPayload.Kind {
		case subscriptionKindSessionStream:
			typed := SessionStreamSubscriptionPayload{}
			if err := decodeSocketPayload(req.Payload, &typed); err != nil {
				return socketActionResult{}, newSocketError(req.ID, "invalid", err.Error())
			}
		case subscriptionKindAgentOutputStream:
			typed := AgentOutputStreamSubscriptionPayload{}
			if err := decodeSocketPayload(req.Payload, &typed); err != nil {
				return socketActionResult{}, newSocketError(req.ID, "invalid", err.Error())
			}
		}
		cityName, apiErr := sm.resolveSocketCityTarget(req.Scope)
		if apiErr != nil {
			apiErr.ID = req.ID
			return socketActionResult{}, apiErr
		}
		state := sm.resolver.CityState(cityName)
		if state == nil {
			return socketActionResult{}, newSocketError(req.ID, "not_found", "city not found or not running: "+cityName)
		}
		cityReq := *req
		cityReq.Scope = nil
		srv := sm.getCityServer(cityName, state)
		result, apiErr := srv.startSocketSubscription(ctx, sess, &cityReq)
		if apiErr == nil {
			result.AfterWrite = sm.wrapAfterWriteWithCityWatch(ctx, cityName, sess, result)
		}
		return result, apiErr
	default:
		return socketActionResult{}, newSocketError(req.ID, "not_found", "unknown subscription kind: "+kindPayload.Kind)
	}
}

func (sm *SupervisorMux) stopSocketSubscription(sess *socketSession, req *socketRequestEnvelope) (socketActionResult, *socketErrorEnvelope) {
	return stopSocketSubscriptionImpl(sess, req)
}

func stopSocketSubscriptionImpl(sess *socketSession, req *socketRequestEnvelope) (socketActionResult, *socketErrorEnvelope) {
	var payload socketSubscriptionStopPayload
	if err := decodeSocketPayload(req.Payload, &payload); err != nil {
		return socketActionResult{}, newSocketError(req.ID, "invalid", err.Error())
	}
	if payload.SubscriptionID == "" {
		return socketActionResult{}, newSocketError(req.ID, "invalid", "subscription_id is required")
	}
	if !sess.stopSubscription(payload.SubscriptionID) {
		return socketActionResult{}, newSocketError(req.ID, "not_found", "subscription not found: "+payload.SubscriptionID)
	}
	return socketActionResult{Result: map[string]string{"status": "ok", "subscription_id": payload.SubscriptionID}}, nil
}

func (s *Server) startEventSubscription(parent context.Context, sess *socketSession, req *socketRequestEnvelope, payload EventsStreamSubscriptionPayload) (socketActionResult, *socketErrorEnvelope) {
	ep := s.state.EventProvider()
	if ep == nil {
		return socketActionResult{}, newSocketError(req.ID, "unavailable", "events not enabled")
	}
	subID := sess.newSubscriptionID()
	subCtx, cancel := context.WithCancel(parent)
	watcher, err := ep.Watch(subCtx, payload.AfterSeq)
	if err != nil {
		cancel()
		return socketActionResult{}, newSocketError(req.ID, "internal", "failed to start event watcher: "+err.Error())
	}
	sess.registerSubscription(subID, cancel)
	log.Printf("api: ws subscription started id=%s kind=%s", subID, subscriptionKindEventsStream)
	telemetry.RecordWebSocketSubscription(context.Background(), 1)
	return socketActionResult{
		Result: map[string]string{"subscription_id": subID, "kind": subscriptionKindEventsStream},
		AfterWrite: func() {
			go func() {
				defer watcher.Close() //nolint:errcheck
				defer cancel()
				defer sess.unregisterSubscription(subID)
				defer log.Printf("api: ws subscription ended id=%s kind=%s", subID, subscriptionKindEventsStream)
				defer telemetry.RecordWebSocketSubscription(context.Background(), -1)
				for {
					event, err := watcher.Next()
					if err != nil {
						return
					}
					envelope := EventsStreamEventEnvelope{
						Type:           "event",
						SubscriptionID: subID,
						EventType:      event.Type,
						Index:          event.Seq,
						Payload: EventsStreamPayload{
							Event:    event,
							Workflow: projectWorkflowEvent(s.state, event),
						},
					}
					if err := sess.conn.writeJSON(envelope); err != nil {
						return
					}
				}
			}()
		},
	}, nil
}

func (sm *SupervisorMux) startGlobalEventSubscription(parent context.Context, sess *socketSession, req *socketRequestEnvelope, payload EventsStreamSubscriptionPayload) (socketActionResult, *socketErrorEnvelope) {
	subID := sess.newSubscriptionID()
	subCtx, cancel := context.WithCancel(parent)
	mw, err := sm.buildMultiplexer().Watch(subCtx, events.ParseCursor(payload.AfterCursor))
	if err != nil {
		cancel()
		return socketActionResult{}, newSocketError(req.ID, "internal", "failed to start global event watcher: "+err.Error())
	}
	sess.registerSubscription(subID, cancel)
	log.Printf("api: ws subscription started id=%s kind=%s", subID, subscriptionKindEventsStream)
	telemetry.RecordWebSocketSubscription(context.Background(), 1)
	cursors := events.ParseCursor(payload.AfterCursor)
	if cursors == nil {
		cursors = make(map[string]uint64)
	}
	return socketActionResult{
		Result: map[string]string{"subscription_id": subID, "kind": subscriptionKindEventsStream},
		AfterWrite: func() {
			go func() {
				defer mw.Close() //nolint:errcheck
				defer cancel()
				defer sess.unregisterSubscription(subID)
				defer log.Printf("api: ws subscription ended id=%s kind=%s", subID, subscriptionKindEventsStream)
				defer telemetry.RecordWebSocketSubscription(context.Background(), -1)
				for {
					tagged, err := mw.Next()
					if err != nil {
						return
					}
					cursors[tagged.City] = tagged.Seq
					var workflow *workflowEventProjection
					if state := sm.resolver.CityState(tagged.City); state != nil {
						workflow = projectWorkflowEvent(state, tagged.Event)
					}
					envelope := EventsStreamEventEnvelope{
						Type:           "event",
						SubscriptionID: subID,
						EventType:      tagged.Type,
						Cursor:         events.FormatCursor(cursors),
						Payload: EventsStreamPayload{
							Event:    tagged.Event,
							City:     tagged.City,
							Workflow: workflow,
						},
					}
					if err := sess.conn.writeJSON(envelope); err != nil {
						return
					}
				}
			}()
		},
	}, nil
}

func (s *Server) startSessionStreamSubscription(parent context.Context, sess *socketSession, req *socketRequestEnvelope, payload SessionStreamSubscriptionPayload) (socketActionResult, *socketErrorEnvelope) {
	store := s.state.CityBeadStore()
	if store == nil {
		return socketActionResult{}, newSocketError(req.ID, "unavailable", "no bead store configured")
	}
	if payload.Target == "" {
		return socketActionResult{}, newSocketError(req.ID, "invalid", "target is required")
	}
	id, err := s.resolveSessionIDAllowClosedWithConfig(store, payload.Target)
	if err != nil {
		return socketActionResult{}, newSocketError(req.ID, "not_found", err.Error())
	}

	mgr := s.sessionManager(store)
	info, err := mgr.Get(id)
	if err != nil {
		return socketActionResult{}, socketErrorFor(req.ID, err)
	}
	path, err := mgr.TranscriptPath(id, s.sessionLogPaths())
	if err != nil {
		return socketActionResult{}, socketErrorFor(req.ID, err)
	}
	sp := s.state.SessionProvider()
	running := info.State == session.StateActive && sp.IsRunning(info.SessionName)
	if path == "" && !running {
		return socketActionResult{}, newSocketError(req.ID, "not_found", "session "+id+" has no live output")
	}

	format, err := normalizeSessionTranscriptFormat(payload.Format)
	if err != nil {
		return socketActionResult{}, socketErrorFor(req.ID, err)
	}
	subID := sess.newSubscriptionID()
	subCtx, cancel := context.WithCancel(parent)
	sess.registerSubscription(subID, cancel)
	log.Printf("api: ws subscription started id=%s kind=session.stream target=%s", subID, payload.Target)
	telemetry.RecordWebSocketSubscription(context.Background(), 1)

	start := func() {
		go func() {
			defer cancel()
			defer sess.unregisterSubscription(subID)
			defer log.Printf("api: ws subscription ended id=%s kind=session.stream target=%s", subID, payload.Target)
			defer telemetry.RecordWebSocketSubscription(context.Background(), -1)
			emitter := newSocketSessionStreamEmitter(sess, subID)
			if info.Closed {
				if format == "raw" {
					s.emitClosedSessionSnapshotRawWithEmitter(emitter, info, path, payload.Turns, payload.AfterCursor)
				} else {
					s.emitClosedSessionSnapshotWithEmitter(emitter, info, path, payload.Turns, payload.AfterCursor)
				}
				return
			}
			switch {
			case path != "":
				if format == "raw" {
					s.streamSessionTranscriptLogRawWithEmitter(subCtx, emitter, info, path, payload.Turns, payload.AfterCursor)
				} else {
					s.streamSessionTranscriptLogWithEmitter(subCtx, emitter, info, path, payload.Turns, payload.AfterCursor)
				}
			case format == "raw":
				if running {
					s.streamSessionPeekRawWithEmitter(subCtx, emitter, info)
				} else {
					_ = emitter.emit("message", 1, sessionRawTranscriptResponse{
						ID:       info.ID,
						Template: info.Template,
						Format:   "raw",
						Messages: []sessionRawMessage{},
					})
				}
			default:
				s.streamSessionPeekWithEmitter(subCtx, emitter, info)
			}
		}()
	}

	return socketActionResult{
		Result:     map[string]string{"subscription_id": subID, "kind": subscriptionKindSessionStream},
		AfterWrite: start,
	}, nil
}

// cityWatcherHub manages one polling goroutine per watched city instead of
// one per subscription. When a city becomes unavailable, all subscriptions
// targeting that city are notified.
type cityWatcherHub struct {
	mu       sync.Mutex
	resolver CityResolver
	cities   map[string]*cityWatcherEntry
}

type cityWatcherEntry struct {
	cancel context.CancelFunc
	subs   map[string]cityWatchSub // keyed by subscription ID
}

type cityWatchSub struct {
	sess  *socketSession
	subID string
}

func newCityWatcherHub(resolver CityResolver) *cityWatcherHub {
	return &cityWatcherHub{
		resolver: resolver,
		cities:   make(map[string]*cityWatcherEntry),
	}
}

// watch registers a subscription for city availability notifications.
// Starts a watcher goroutine for the city if one isn't running.
func (h *cityWatcherHub) watch(cityName string, sess *socketSession, subID string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	entry, ok := h.cities[cityName]
	if !ok {
		ctx, cancel := context.WithCancel(context.Background())
		entry = &cityWatcherEntry{
			cancel: cancel,
			subs:   make(map[string]cityWatchSub),
		}
		h.cities[cityName] = entry
		go h.pollCity(ctx, cityName)
	}
	entry.subs[subID] = cityWatchSub{sess: sess, subID: subID}
}

// unwatch removes a subscription from city notifications.
// Stops the watcher goroutine if no subscriptions remain.
func (h *cityWatcherHub) unwatch(cityName, subID string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	entry, ok := h.cities[cityName]
	if !ok {
		return
	}
	delete(entry.subs, subID)
	if len(entry.subs) == 0 {
		entry.cancel()
		delete(h.cities, cityName)
	}
}

func (h *cityWatcherHub) pollCity(ctx context.Context, cityName string) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if h.resolver.CityState(cityName) == nil {
				h.notifyCityUnavailable(cityName)
				return
			}
		}
	}
}

func (h *cityWatcherHub) notifyCityUnavailable(cityName string) {
	h.mu.Lock()
	entry, ok := h.cities[cityName]
	if !ok {
		h.mu.Unlock()
		return
	}
	// Copy subs under lock, then notify outside.
	subs := make([]cityWatchSub, 0, len(entry.subs))
	for _, sub := range entry.subs {
		subs = append(subs, sub)
	}
	entry.cancel()
	delete(h.cities, cityName)
	h.mu.Unlock()

	for _, sub := range subs {
		log.Printf("api: ws subscription id=%s city %s became unavailable", sub.subID, cityName)
		_ = sub.sess.conn.writeJSON(socketEventEnvelope{
			Type:           "event",
			SubscriptionID: sub.subID,
			EventType:      "city.unavailable",
			Payload:        map[string]string{"city": cityName, "reason": "city stopped or unavailable"},
		})
		sub.sess.stopSubscription(sub.subID)
	}
}

// wrapAfterWriteWithCityWatch composes the original AfterWrite with a city
// availability watcher for supervisor-scoped city subscriptions.
func (sm *SupervisorMux) wrapAfterWriteWithCityWatch(ctx context.Context, cityName string, sess *socketSession, result socketActionResult) func() {
	origAfterWrite := result.AfterWrite
	return func() {
		if origAfterWrite != nil {
			origAfterWrite()
		}
		if m, ok := result.Result.(map[string]string); ok {
			if subID := m["subscription_id"]; subID != "" {
				sm.cityWatchers.watch(cityName, sess, subID)
				// Cleanup when subscription ends (context cancelled).
				go func() {
					<-ctx.Done()
					sm.cityWatchers.unwatch(cityName, subID)
				}()
			}
		}
	}
}
