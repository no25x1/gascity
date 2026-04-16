// ApiAgentOutputStreamSubscriptionPayload represents a ApiAgentOutputStreamSubscriptionPayload model.
#[derive(Clone, Debug, Deserialize, Serialize)]
pub struct ApiAgentOutputStreamSubscriptionPayload {
    #[serde(rename="after_cursor", skip_serializing_if = "Option::is_none")]
    pub after_cursor: Option<String>,
    #[serde(rename="kind", skip_serializing_if = "Option::is_none")]
    pub kind: Option<String>,
    #[serde(rename="target", skip_serializing_if = "Option::is_none")]
    pub target: Option<String>,
    #[serde(rename="additionalProperties", skip_serializing_if = "Option::is_none")]
    pub additional_properties: Option<std::collections::HashMap<String, serde_json::Value>>,
}
