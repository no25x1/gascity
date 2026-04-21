package main

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/gastownhall/gascity/internal/beads"
)

func TestWritePreLaunchClaimResultDrain(t *testing.T) {
	var out bytes.Buffer
	if err := writePreLaunchClaimResult(&out, claimNextResult{Reason: "no_work"}); err != nil {
		t.Fatalf("writePreLaunchClaimResult: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if got["action"] != "drain" || got["reason"] != "no_work" {
		t.Fatalf("response = %#v, want drain/no_work", got)
	}
}

func TestWritePreLaunchClaimResultContinue(t *testing.T) {
	var out bytes.Buffer
	if err := writePreLaunchClaimResult(&out, claimNextResult{
		Reason: "claimed",
		Bead:   beads.Bead{ID: "ga-123"},
	}); err != nil {
		t.Fatalf("writePreLaunchClaimResult: %v", err)
	}
	var got struct {
		Action      string            `json:"action"`
		Reason      string            `json:"reason"`
		Env         map[string]string `json:"env"`
		NudgeAppend string            `json:"nudge_append"`
		Metadata    map[string]string `json:"metadata"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if got.Action != "continue" || got.Env["GC_WORK_BEAD"] != "ga-123" {
		t.Fatalf("response = %#v, want continue with GC_WORK_BEAD", got)
	}
	if got.Metadata["pre_launch.user.claimed_work_bead"] != "ga-123" {
		t.Fatalf("metadata = %#v, want claimed work bead", got.Metadata)
	}
}
