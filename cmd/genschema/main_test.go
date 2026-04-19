package main

import (
	"testing"

	"github.com/gastownhall/gascity/internal/docgen"
)

func TestGeneratedCitySchemaIncludesLegacyOrderOverrideGateAlias(t *testing.T) {
	schema, err := docgen.GenerateCitySchema()
	if err != nil {
		t.Fatalf("GenerateCitySchema: %v", err)
	}

	orderOverride, ok := schema.Definitions["OrderOverride"]
	if !ok {
		t.Fatal("OrderOverride definition missing")
	}
	if orderOverride.Properties == nil {
		t.Fatal("OrderOverride properties missing")
	}
	gate, ok := orderOverride.Properties.Get("gate")
	if !ok {
		t.Fatal("legacy gate alias missing from generated OrderOverride schema")
	}
	deprecated, ok := gate.Extras["deprecated"]
	if !ok || deprecated != true {
		t.Fatalf("legacy gate alias should be marked deprecated in generated schema, got %#v", gate.Extras)
	}
}
