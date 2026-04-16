
package wscontract

type ApiEventsStreamPayload struct {
  Actor string `json:"actor,omitempty"`
  City string `json:"city,omitempty"`
  Message string `json:"message,omitempty"`
  Payload interface{} `json:"payload,omitempty"`
  Seq int `json:"seq,omitempty"`
  Subject string `json:"subject,omitempty"`
  Ts string `json:"ts,omitempty"`
  ReservedType string `json:"type,omitempty"`
  Workflow *ApiWorkflowEventProjection `json:"workflow,omitempty"`
  AdditionalProperties map[string]interface{} `json:"-,omitempty"`
}