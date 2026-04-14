package api

import "context"

type socketExtMsgBindingsPayload struct {
	SessionID string `json:"session_id"`
}

func init() {
	RegisterAction("extmsg.inbound", ActionDef{
		Description:       "Process inbound external message",
		IsMutation:        true,
		RequiresCityScope: true,
	}, func(ctx context.Context, s *Server, payload extmsgInboundRequest) (any, error) {
		return s.processExtMsgInbound(ctx, payload)
	})

	RegisterAction("extmsg.outbound", ActionDef{
		Description:       "Process outbound external message",
		IsMutation:        true,
		RequiresCityScope: true,
	}, func(ctx context.Context, s *Server, payload extmsgOutboundRequest) (any, error) {
		return s.processExtMsgOutbound(ctx, payload)
	})

	RegisterAction("extmsg.bindings.list", ActionDef{
		Description:       "List external message bindings",
		RequiresCityScope: true,
	}, func(ctx context.Context, s *Server, payload socketExtMsgBindingsPayload) (any, error) {
		return s.listExtMsgBindings(ctx, payload.SessionID)
	})

	RegisterAction("extmsg.bind", ActionDef{
		Description:       "Bind external message channel",
		IsMutation:        true,
		RequiresCityScope: true,
	}, func(ctx context.Context, s *Server, payload extmsgBindRequest) (any, error) {
		return s.processExtMsgBind(ctx, payload)
	})

	RegisterAction("extmsg.unbind", ActionDef{
		Description:       "Unbind external message channel",
		IsMutation:        true,
		RequiresCityScope: true,
	}, func(ctx context.Context, s *Server, payload extmsgUnbindRequest) (any, error) {
		return s.processExtMsgUnbind(ctx, payload)
	})

	RegisterAction("extmsg.groups.lookup", ActionDef{
		Description:       "Look up external message group",
		RequiresCityScope: true,
	}, func(ctx context.Context, s *Server, payload extmsgGroupLookupRequest) (any, error) {
		return s.lookupExtMsgGroup(ctx, payload)
	})

	RegisterAction("extmsg.groups.ensure", ActionDef{
		Description:       "Ensure external message group exists",
		IsMutation:        true,
		RequiresCityScope: true,
	}, func(ctx context.Context, s *Server, payload extmsgGroupEnsureRequest) (any, error) {
		return s.ensureExtMsgGroup(ctx, payload)
	})

	RegisterAction("extmsg.participant.upsert", ActionDef{
		Description:       "Upsert external message participant",
		IsMutation:        true,
		RequiresCityScope: true,
	}, func(ctx context.Context, s *Server, payload extmsgParticipantUpsertRequest) (any, error) {
		return s.upsertExtMsgParticipant(ctx, payload)
	})

	RegisterAction("extmsg.participant.remove", ActionDef{
		Description:       "Remove external message participant",
		IsMutation:        true,
		RequiresCityScope: true,
	}, func(ctx context.Context, s *Server, payload extmsgParticipantRemoveRequest) (any, error) {
		return s.removeExtMsgParticipant(ctx, payload)
	})

	RegisterAction("extmsg.transcript.list", ActionDef{
		Description:       "List external message transcript",
		RequiresCityScope: true,
	}, func(ctx context.Context, s *Server, payload extmsgTranscriptListRequest) (any, error) {
		return s.listExtMsgTranscript(ctx, payload)
	})

	RegisterAction("extmsg.transcript.ack", ActionDef{
		Description:       "Acknowledge external message transcript",
		IsMutation:        true,
		RequiresCityScope: true,
	}, func(ctx context.Context, s *Server, payload extmsgTranscriptAckRequest) (any, error) {
		return s.ackExtMsgTranscript(ctx, payload)
	})

	RegisterVoidAction("extmsg.adapters.list", ActionDef{
		Description:       "List external message adapters",
		RequiresCityScope: true,
	}, func(_ context.Context, s *Server) (any, error) {
		return s.listExtMsgAdapters()
	})

	RegisterAction("extmsg.adapters.register", ActionDef{
		Description:       "Register external message adapter",
		IsMutation:        true,
		RequiresCityScope: true,
	}, func(_ context.Context, s *Server, payload extmsgAdapterRegisterRequest) (any, error) {
		return s.registerExtMsgAdapter(payload)
	})

	RegisterAction("extmsg.adapters.unregister", ActionDef{
		Description:       "Unregister external message adapter",
		IsMutation:        true,
		RequiresCityScope: true,
	}, func(_ context.Context, s *Server, payload extmsgAdapterUnregisterRequest) (any, error) {
		return s.unregisterExtMsgAdapter(payload)
	})
}
