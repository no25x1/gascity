// ApiEventsStreamPayload represents a ApiEventsStreamPayload model.
#[derive(Clone, Debug, Deserialize, Serialize)]
pub struct ApiEventsStreamPayload {
    #[serde(rename="actor", skip_serializing_if = "Option::is_none")]
    pub actor: Option<String>,
    #[serde(rename="city", skip_serializing_if = "Option::is_none")]
    pub city: Option<String>,
    #[serde(rename="message", skip_serializing_if = "Option::is_none")]
    pub message: Option<String>,
    #[serde(rename="payload", skip_serializing_if = "Option::is_none")]
    pub payload: Option<serde_json::Value>,
    #[serde(rename="seq", skip_serializing_if = "Option::is_none")]
    pub seq: Option<i32>,
    #[serde(rename="subject", skip_serializing_if = "Option::is_none")]
    pub subject: Option<String>,
    #[serde(rename="ts", skip_serializing_if = "Option::is_none")]
    pub ts: Option<String>,
    #[serde(rename="type", skip_serializing_if = "Option::is_none")]
    pub reserved_type: Option<String>,
    #[serde(rename="workflow", skip_serializing_if = "Option::is_none")]
    pub workflow: Option<Box<crate::ApiWorkflowEventProjection>>,
    #[serde(rename="additionalProperties", skip_serializing_if = "Option::is_none")]
    pub additional_properties: Option<std::collections::HashMap<String, serde_json::Value>>,
}
