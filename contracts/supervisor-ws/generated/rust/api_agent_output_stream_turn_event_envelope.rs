// ApiAgentOutputStreamTurnEventEnvelope represents a ApiAgentOutputStreamTurnEventEnvelope model.
#[derive(Clone, Debug, Deserialize, Serialize)]
pub struct ApiAgentOutputStreamTurnEventEnvelope {
    #[serde(rename="cursor", skip_serializing_if = "Option::is_none")]
    pub cursor: Option<String>,
    #[serde(rename="event_type", skip_serializing_if = "Option::is_none")]
    pub event_type: Option<String>,
    #[serde(rename="index", skip_serializing_if = "Option::is_none")]
    pub index: Option<i32>,
    #[serde(rename="payload", skip_serializing_if = "Option::is_none")]
    pub payload: Option<Box<crate::ApiAgentOutputResponse>>,
    #[serde(rename="subscription_id", skip_serializing_if = "Option::is_none")]
    pub subscription_id: Option<String>,
    #[serde(rename="type", skip_serializing_if = "Option::is_none")]
    pub reserved_type: Option<String>,
    #[serde(rename="additionalProperties", skip_serializing_if = "Option::is_none")]
    pub additional_properties: Option<std::collections::HashMap<String, serde_json::Value>>,
}
