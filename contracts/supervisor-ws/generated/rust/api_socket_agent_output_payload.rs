// ApiSocketAgentOutputPayload represents a ApiSocketAgentOutputPayload model.
#[derive(Clone, Debug, Deserialize, Serialize)]
pub struct ApiSocketAgentOutputPayload {
    #[serde(rename="before", skip_serializing_if = "Option::is_none")]
    pub before: Option<String>,
    #[serde(rename="name", skip_serializing_if = "Option::is_none")]
    pub name: Option<String>,
    #[serde(rename="tail", skip_serializing_if = "Option::is_none")]
    pub tail: Option<i32>,
    #[serde(rename="additionalProperties", skip_serializing_if = "Option::is_none")]
    pub additional_properties: Option<std::collections::HashMap<String, serde_json::Value>>,
}
