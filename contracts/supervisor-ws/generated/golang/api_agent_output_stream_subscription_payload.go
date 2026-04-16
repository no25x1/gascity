
package wscontract

type ApiAgentOutputStreamSubscriptionPayload struct {
  // Resume from this cursor
  AfterCursor string `json:"after_cursor,omitempty"`
  // Must be 'agent.output.stream'
  Kind string `json:"kind,omitempty"`
  // Agent name
  Target string `json:"target,omitempty"`
  AdditionalProperties map[string]interface{} `json:"-,omitempty"`
}