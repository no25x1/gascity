
package wscontract

type ApiSocketAgentOutputPayload struct {
  Before string `json:"before,omitempty"`
  Name string `json:"name,omitempty"`
  Tail *int `json:"tail,omitempty"`
  AdditionalProperties map[string]interface{} `json:"-,omitempty"`
}