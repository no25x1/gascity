
package wscontract

type ApiStreamActivityPayload struct {
  // Session activity state
  Activity string `json:"activity,omitempty"`
  AdditionalProperties map[string]interface{} `json:"-,omitempty"`
}