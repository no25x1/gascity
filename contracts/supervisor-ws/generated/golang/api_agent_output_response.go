
package wscontract

type ApiAgentOutputResponse struct {
  Agent string `json:"agent,omitempty"`
  Format string `json:"format,omitempty"`
  Pagination *SessionlogPaginationInfo `json:"pagination,omitempty"`
  Turns []ApiOutputTurn `json:"turns,omitempty"`
  AdditionalProperties map[string]interface{} `json:"-,omitempty"`
}