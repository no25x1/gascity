
package wscontract

type ApiWorkflowEventProjection struct {
  AttemptSummary map[string]interface{} `json:"attempt_summary,omitempty"`
  Bead *ApiWorkflowBeadResponse `json:"bead,omitempty"`
  ChangedFields []string `json:"changed_fields,omitempty"`
  EventSeq int `json:"event_seq,omitempty"`
  EventTs string `json:"event_ts,omitempty"`
  EventType string `json:"event_type,omitempty"`
  LogicalNodeId string `json:"logical_node_id,omitempty"`
  RequiresResync bool `json:"requires_resync,omitempty"`
  RootBeadId string `json:"root_bead_id,omitempty"`
  RootStoreRef string `json:"root_store_ref,omitempty"`
  ScopeKind string `json:"scope_kind,omitempty"`
  ScopeRef string `json:"scope_ref,omitempty"`
  ReservedType string `json:"type,omitempty"`
  WatchGeneration string `json:"watch_generation,omitempty"`
  WorkflowId string `json:"workflow_id,omitempty"`
  WorkflowSeq int `json:"workflow_seq,omitempty"`
  AdditionalProperties map[string]interface{} `json:"-,omitempty"`
}