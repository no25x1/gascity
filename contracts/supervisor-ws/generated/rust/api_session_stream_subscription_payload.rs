// ApiSessionStreamSubscriptionPayload represents a ApiSessionStreamSubscriptionPayload model.
#[derive(Clone, Debug, Deserialize, Serialize)]
pub struct ApiSessionStreamSubscriptionPayload {
    #[serde(rename="after_cursor", skip_serializing_if = "Option::is_none")]
    pub after_cursor: Option<String>,
    #[serde(rename="format", skip_serializing_if = "Option::is_none")]
    pub format: Option<String>,
    #[serde(rename="kind", skip_serializing_if = "Option::is_none")]
    pub kind: Option<String>,
    #[serde(rename="target", skip_serializing_if = "Option::is_none")]
    pub target: Option<String>,
    #[serde(rename="turns", skip_serializing_if = "Option::is_none")]
    pub turns: Option<i32>,
    #[serde(rename="additionalProperties", skip_serializing_if = "Option::is_none")]
    pub additional_properties: Option<std::collections::HashMap<String, serde_json::Value>>,
}
