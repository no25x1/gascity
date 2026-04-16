
package wscontract

type ApiSessionStreamMessageEventEnvelope struct {
  // Resume cursor for reconnection
  Cursor string `json:"cursor,omitempty"`
  // Must be 'message'
  EventType string `json:"event_type,omitempty"`
  // Event sequence number
  Index int `json:"index,omitempty"`
  Payload *ApiSessionTranscriptResult `json:"payload,omitempty"`
  // Subscription that produced this event
  SubscriptionId string `json:"subscription_id,omitempty"`
  // Must be 'event'
  ReservedType string `json:"type,omitempty"`
  AdditionalProperties map[string]interface{} `json:"-,omitempty"`
}