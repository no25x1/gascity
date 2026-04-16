
package wscontract

type ApiEventsStreamEventEnvelope struct {
  // Resume cursor for reconnection
  Cursor string `json:"cursor,omitempty"`
  // Event type (e.g. 'bead.created')
  EventType string `json:"event_type,omitempty"`
  // Event sequence number
  Index int `json:"index,omitempty"`
  Payload *ApiEventsStreamPayload `json:"payload,omitempty"`
  // Subscription that produced this event
  SubscriptionId string `json:"subscription_id,omitempty"`
  // Must be 'event'
  ReservedType string `json:"type,omitempty"`
  AdditionalProperties map[string]interface{} `json:"-,omitempty"`
}