// ApiWorkflowEventProjection represents a ApiWorkflowEventProjection model.
#[derive(Clone, Debug, Deserialize, Serialize)]
pub struct ApiWorkflowEventProjection {
    #[serde(rename="attempt_summary", skip_serializing_if = "Option::is_none")]
    pub attempt_summary: Option<std::collections::HashMap<String, serde_json::Value>>,
    #[serde(rename="bead", skip_serializing_if = "Option::is_none")]
    pub bead: Option<Box<crate::ApiWorkflowBeadResponse>>,
    #[serde(rename="changed_fields", skip_serializing_if = "Option::is_none")]
    pub changed_fields: Option<Vec<String>>,
    #[serde(rename="event_seq", skip_serializing_if = "Option::is_none")]
    pub event_seq: Option<i32>,
    #[serde(rename="event_ts", skip_serializing_if = "Option::is_none")]
    pub event_ts: Option<String>,
    #[serde(rename="event_type", skip_serializing_if = "Option::is_none")]
    pub event_type: Option<String>,
    #[serde(rename="logical_node_id", skip_serializing_if = "Option::is_none")]
    pub logical_node_id: Option<String>,
    #[serde(rename="requires_resync", skip_serializing_if = "Option::is_none")]
    pub requires_resync: Option<bool>,
    #[serde(rename="root_bead_id", skip_serializing_if = "Option::is_none")]
    pub root_bead_id: Option<String>,
    #[serde(rename="root_store_ref", skip_serializing_if = "Option::is_none")]
    pub root_store_ref: Option<String>,
    #[serde(rename="scope_kind", skip_serializing_if = "Option::is_none")]
    pub scope_kind: Option<String>,
    #[serde(rename="scope_ref", skip_serializing_if = "Option::is_none")]
    pub scope_ref: Option<String>,
    #[serde(rename="type", skip_serializing_if = "Option::is_none")]
    pub reserved_type: Option<String>,
    #[serde(rename="watch_generation", skip_serializing_if = "Option::is_none")]
    pub watch_generation: Option<String>,
    #[serde(rename="workflow_id", skip_serializing_if = "Option::is_none")]
    pub workflow_id: Option<String>,
    #[serde(rename="workflow_seq", skip_serializing_if = "Option::is_none")]
    pub workflow_seq: Option<i32>,
    #[serde(rename="additionalProperties", skip_serializing_if = "Option::is_none")]
    pub additional_properties: Option<std::collections::HashMap<String, serde_json::Value>>,
}
