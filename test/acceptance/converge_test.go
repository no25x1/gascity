//go:build acceptance_a

// Converge command acceptance tests.
//
// These exercise gc converge as a black box. Convergence loops are
// bounded iterative refinement cycles (root bead + formula + gate).
// Most mutating operations (create, approve, iterate, stop) require a
// running controller, so Tier A tests focus on list, status, flag
// validation, and error paths.
package acceptance_test

import (
	"strings"
	"testing"

	helpers "github.com/gastownhall/gascity/test/acceptance/helpers"
)

// --- gc converge list ---

func TestConvergeList_EmptyCity_ShowsNone(t *testing.T) {
	c := helpers.NewCity(t, testEnv)
	c.Init("claude")

	out, err := c.GC("converge", "list")
	if err != nil {
		t.Fatalf("gc converge list: %v\n%s", err, out)
	}
	if !strings.Contains(out, "No convergence loops found") {
		t.Errorf("expected 'No convergence loops found' on empty city, got:\n%s", out)
	}
}

func TestConvergeList_JSON_ReturnsArray(t *testing.T) {
	c := helpers.NewCity(t, testEnv)
	c.Init("claude")

	out, err := c.GC("converge", "list", "--json")
	if err != nil {
		t.Fatalf("gc converge list --json: %v\n%s", err, out)
	}
	trimmed := strings.TrimSpace(out)
	// Empty JSON array or null is expected on a fresh city.
	if trimmed != "[]" && trimmed != "null" {
		t.Errorf("expected empty JSON array on fresh city, got:\n%s", out)
	}
}

// --- gc converge create (flag validation) ---

func TestConvergeCreate_MissingFormula_ReturnsError(t *testing.T) {
	c := helpers.NewCity(t, testEnv)
	c.Init("claude")

	// --formula and --target are required. Missing --formula triggers cobra error.
	_, err := c.GC("converge", "create", "--target", "some-agent")
	if err == nil {
		t.Fatal("expected error for missing --formula, got success")
	}
}

func TestConvergeCreate_MissingTarget_ReturnsError(t *testing.T) {
	c := helpers.NewCity(t, testEnv)
	c.Init("claude")

	_, err := c.GC("converge", "create", "--formula", "some-formula")
	if err == nil {
		t.Fatal("expected error for missing --target, got success")
	}
}

// --- gc converge status (error paths) ---

func TestConvergeStatus_NonexistentBead_ReturnsError(t *testing.T) {
	c := helpers.NewCity(t, testEnv)
	c.Init("claude")

	_, err := c.GC("converge", "status", "gc-99999")
	if err == nil {
		t.Fatal("expected error for nonexistent bead, got success")
	}
}

func TestConvergeStatus_MissingID_ReturnsError(t *testing.T) {
	c := helpers.NewCity(t, testEnv)
	c.Init("claude")

	// cobra.ExactArgs(1) handles this.
	_, err := c.GC("converge", "status")
	if err == nil {
		t.Fatal("expected error for missing bead ID, got success")
	}
}

// --- gc converge test-gate (error paths) ---

func TestConvergeTestGate_NonexistentBead_ReturnsError(t *testing.T) {
	c := helpers.NewCity(t, testEnv)
	c.Init("claude")

	_, err := c.GC("converge", "test-gate", "gc-99999")
	if err == nil {
		t.Fatal("expected error for nonexistent bead, got success")
	}
}

// --- gc converge approve/iterate/stop (missing args) ---

func TestConvergeApprove_MissingID_ReturnsError(t *testing.T) {
	c := helpers.NewCity(t, testEnv)
	c.Init("claude")

	_, err := c.GC("converge", "approve")
	if err == nil {
		t.Fatal("expected error for missing bead ID, got success")
	}
}

func TestConvergeIterate_MissingID_ReturnsError(t *testing.T) {
	c := helpers.NewCity(t, testEnv)
	c.Init("claude")

	_, err := c.GC("converge", "iterate")
	if err == nil {
		t.Fatal("expected error for missing bead ID, got success")
	}
}

func TestConvergeStop_MissingID_ReturnsError(t *testing.T) {
	c := helpers.NewCity(t, testEnv)
	c.Init("claude")

	_, err := c.GC("converge", "stop")
	if err == nil {
		t.Fatal("expected error for missing bead ID, got success")
	}
}
