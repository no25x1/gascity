package api

import (
	"testing"
)

// TestAgentCreateSpecMarksFieldsRequired verifies that the OpenAPI spec
// marks name and provider as required fields (Phase 2 Fix 2: no more
// omitempty bypass hiding required fields).
func TestAgentCreateSpecMarksFieldsRequired(t *testing.T) {
	spec := readCommittedOpenAPISpec(t)

	// Walk to the request body schema for POST /v0/city/{cityName}/agents.
	paths, _ := spec["paths"].(map[string]any)
	agentsPath, _ := paths["/v0/city/{cityName}/agents"].(map[string]any)
	post, _ := agentsPath["post"].(map[string]any)
	reqBody, _ := post["requestBody"].(map[string]any)
	content, _ := reqBody["content"].(map[string]any)
	appJSON, _ := content["application/json"].(map[string]any)
	schema, _ := appJSON["schema"].(map[string]any)

	// Schema is usually a $ref; resolve it.
	if ref, ok := schema["$ref"].(string); ok {
		// "#/components/schemas/FooRequest" → FooRequest
		name := ref[len("#/components/schemas/"):]
		components, _ := spec["components"].(map[string]any)
		schemas, _ := components["schemas"].(map[string]any)
		resolved, ok := schemas[name].(map[string]any)
		if !ok {
			t.Fatalf("could not resolve $ref %s", ref)
		}
		schema = resolved
	}

	required, _ := schema["required"].([]any)
	reqMap := make(map[string]bool)
	for _, r := range required {
		if s, ok := r.(string); ok {
			reqMap[s] = true
		}
	}

	if !reqMap["name"] {
		t.Errorf("agent create schema does not mark name as required; required=%v", required)
	}
	if !reqMap["provider"] {
		t.Errorf("agent create schema does not mark provider as required; required=%v", required)
	}
}

func TestOrderResponseSpecKeepsLegacyGateDeprecatedAndTriggerOptional(t *testing.T) {
	spec := readCommittedOpenAPISpec(t)

	components, _ := spec["components"].(map[string]any)
	schemas, _ := components["schemas"].(map[string]any)
	schema, ok := schemas["OrderResponse"].(map[string]any)
	if !ok {
		t.Fatal("OrderResponse schema not found")
	}

	required, _ := schema["required"].([]any)
	for _, r := range required {
		if s, ok := r.(string); ok && s == "trigger" {
			t.Fatalf("OrderResponse.trigger should remain optional for old-server compatibility; required=%v", required)
		}
	}

	properties, _ := schema["properties"].(map[string]any)
	gate, ok := properties["gate"].(map[string]any)
	if !ok {
		t.Fatal("OrderResponse.gate property not found")
	}
	if deprecated, _ := gate["deprecated"].(bool); !deprecated {
		t.Fatalf("OrderResponse.gate should be marked deprecated; property=%v", gate)
	}
}
