
package wscontract

type ApiSessionStreamPendingEventEnvelope struct {
  // Must be 'pending'
  EventType string `json:"event_type,omitempty"`
  // Event sequence number
  Index int `json:"index,omitempty"`
  Payload *RuntimePendingInteraction `json:"payload,omitempty"`
  // Subscription that produced this event
  SubscriptionId string `json:"subscription_id,omitempty"`
  // Must be 'event'
  ReservedType string `json:"type,omitempty"`
  AdditionalProperties map[string]interface{} `json:"-,omitempty"`
}