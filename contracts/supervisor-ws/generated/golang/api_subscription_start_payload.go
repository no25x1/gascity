
package wscontract

type ApiSubscriptionStartPayload struct {
  // Resume from this cursor
  AfterCursor string `json:"after_cursor,omitempty"`
  // Resume from this event sequence
  AfterSeq int `json:"after_seq,omitempty"`
  // Stream format: 'text', 'raw', 'jsonl'
  Format string `json:"format,omitempty"`
  // Subscription type: 'events.stream', 'session.stream', or 'agent.output.stream'
  Kind string `json:"kind,omitempty"`
  // Stream target identifier (session ID/name or agent name)
  Target string `json:"target,omitempty"`
  // Most recent N turns (0=all)
  Turns int `json:"turns,omitempty"`
  AdditionalProperties map[string]interface{} `json:"-,omitempty"`
}