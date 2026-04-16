
package wscontract

type ApiSessionStreamSubscriptionPayload struct {
  // Resume from this cursor
  AfterCursor string `json:"after_cursor,omitempty"`
  // Stream format: 'text', 'raw', 'jsonl'
  Format string `json:"format,omitempty"`
  // Must be 'session.stream'
  Kind string `json:"kind,omitempty"`
  // Session ID or session name
  Target string `json:"target,omitempty"`
  // Most recent N turns (0=all)
  Turns int `json:"turns,omitempty"`
  AdditionalProperties map[string]interface{} `json:"-,omitempty"`
}