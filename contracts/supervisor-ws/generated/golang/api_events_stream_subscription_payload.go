
package wscontract

type ApiEventsStreamSubscriptionPayload struct {
  // Resume from this cursor
  AfterCursor string `json:"after_cursor,omitempty"`
  // Resume from this event sequence
  AfterSeq int `json:"after_seq,omitempty"`
  // Must be 'events.stream'
  Kind string `json:"kind,omitempty"`
  AdditionalProperties map[string]interface{} `json:"-,omitempty"`
}