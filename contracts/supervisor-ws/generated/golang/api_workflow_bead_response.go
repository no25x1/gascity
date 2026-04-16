
package wscontract

type ApiWorkflowBeadResponse struct {
  Assignee string `json:"assignee,omitempty"`
  Attempt *int `json:"attempt,omitempty"`
  Id string `json:"id,omitempty"`
  Kind string `json:"kind,omitempty"`
  LogicalBeadId string `json:"logical_bead_id,omitempty"`
  Metadata map[string]string `json:"metadata,omitempty"`
  ScopeRef string `json:"scope_ref,omitempty"`
  Status string `json:"status,omitempty"`
  StepRef string `json:"step_ref,omitempty"`
  Title string `json:"title,omitempty"`
  AdditionalProperties map[string]interface{} `json:"-,omitempty"`
}