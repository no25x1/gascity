// ApiStreamActivityPayload represents a ApiStreamActivityPayload model.
#[derive(Clone, Debug, Deserialize, Serialize)]
pub struct ApiStreamActivityPayload {
    #[serde(rename="activity", skip_serializing_if = "Option::is_none")]
    pub activity: Option<String>,
    #[serde(rename="additionalProperties", skip_serializing_if = "Option::is_none")]
    pub additional_properties: Option<std::collections::HashMap<String, serde_json::Value>>,
}
