//go:build acceptance_a

// CLI basics acceptance tests.
//
// These exercise the foundational gc commands that every user touches:
// version, help, hook, and stop. These are the first things a new user
// runs and must work correctly.
package acceptance_test

import (
	"strings"
	"testing"

	helpers "github.com/gastownhall/gascity/test/acceptance/helpers"
)

// --- gc version ---

func TestVersion_PrintsVersion(t *testing.T) {
	out, err := helpers.RunGC(testEnv, "", "version")
	if err != nil {
		t.Fatalf("gc version: %v\n%s", err, out)
	}
	out = strings.TrimSpace(out)
	if out == "" {
		t.Fatal("gc version produced empty output")
	}
}

func TestVersion_Long_IncludesCommitInfo(t *testing.T) {
	out, err := helpers.RunGC(testEnv, "", "version", "--long")
	if err != nil {
		t.Fatalf("gc version --long: %v\n%s", err, out)
	}
	if !strings.Contains(out, "commit:") {
		t.Errorf("expected 'commit:' in long version output, got:\n%s", out)
	}
}

// --- gc help ---

func TestHelp_ListsSubcommands(t *testing.T) {
	out, err := helpers.RunGC(testEnv, "", "help")
	if err != nil {
		t.Fatalf("gc help: %v\n%s", err, out)
	}
	for _, cmd := range []string{"init", "start", "stop", "status", "rig", "config"} {
		if !strings.Contains(out, cmd) {
			t.Errorf("gc help should mention %q subcommand, got:\n%s", cmd, out)
		}
	}
}

// --- gc hook ---

func TestHook_NoAgent_ReturnsError(t *testing.T) {
	c := helpers.NewCity(t, testEnv)
	c.Init("claude")

	out, err := c.GC("hook")
	if err == nil {
		t.Fatal("expected error for 'gc hook' without agent, got success")
	}
	if !strings.Contains(out, "agent not specified") {
		t.Errorf("expected 'agent not specified' message, got:\n%s", out)
	}
}

func TestHook_UnknownAgent_ReturnsError(t *testing.T) {
	c := helpers.NewCity(t, testEnv)
	c.Init("claude")

	out, err := c.GC("hook", "nonexistent-agent-xyz")
	if err == nil {
		t.Fatal("expected error for unknown agent, got success")
	}
	if !strings.Contains(out, "not found") {
		t.Errorf("expected 'not found' message, got:\n%s", out)
	}
}

func TestHook_Inject_NoAgent_ExitsZero(t *testing.T) {
	c := helpers.NewCity(t, testEnv)
	c.Init("claude")

	// --inject should always exit 0, even without an agent.
	_, err := c.GC("hook", "--inject")
	if err != nil {
		t.Fatalf("gc hook --inject should exit 0, got: %v", err)
	}
}

// --- gc stop ---

func TestStop_NotInitialized_ReturnsError(t *testing.T) {
	emptyDir := t.TempDir()
	_, err := helpers.RunGC(testEnv, emptyDir, "stop", emptyDir)
	if err == nil {
		t.Fatal("expected error for stop on non-city directory, got success")
	}
}

func TestStop_InitializedNeverStarted_Succeeds(t *testing.T) {
	c := helpers.NewCity(t, testEnv)
	c.Init("claude")

	// Stop on a city that was never started should not error.
	_, err := c.GC("stop", c.Dir)
	if err != nil {
		t.Fatalf("gc stop on never-started city should succeed, got: %v", err)
	}
}
