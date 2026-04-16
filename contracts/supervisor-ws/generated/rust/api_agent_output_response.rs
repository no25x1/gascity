// ApiAgentOutputResponse represents a ApiAgentOutputResponse model.
#[derive(Clone, Debug, Deserialize, Serialize)]
pub struct ApiAgentOutputResponse {
    #[serde(rename="agent", skip_serializing_if = "Option::is_none")]
    pub agent: Option<String>,
    #[serde(rename="format", skip_serializing_if = "Option::is_none")]
    pub format: Option<String>,
    #[serde(rename="pagination", skip_serializing_if = "Option::is_none")]
    pub pagination: Option<Box<crate::SessionlogPaginationInfo>>,
    #[serde(rename="turns", skip_serializing_if = "Option::is_none")]
    pub turns: Option<Vec<crate::ApiOutputTurn>>,
    #[serde(rename="additionalProperties", skip_serializing_if = "Option::is_none")]
    pub additional_properties: Option<std::collections::HashMap<String, serde_json::Value>>,
}
