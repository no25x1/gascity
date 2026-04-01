//go:build acceptance_a

// Session management acceptance tests.
//
// These exercise gc session subcommands as a black box. Sessions are
// the primary mechanism for interactive agent chat — testing their CLI
// is high-value because every user interacts with sessions.
package acceptance_test

import (
	"strings"
	"testing"

	helpers "github.com/gastownhall/gascity/test/acceptance/helpers"
)

// --- gc session (bare command) ---

func TestSession_NoSubcommand_ReturnsError(t *testing.T) {
	c := helpers.NewCity(t, testEnv)
	c.Init("claude")

	out, err := c.GC("session")
	if err == nil {
		t.Fatal("expected error for bare 'gc session', got success")
	}
	if !strings.Contains(out, "missing subcommand") {
		t.Errorf("expected 'missing subcommand' message, got:\n%s", out)
	}
}

func TestSession_UnknownSubcommand_ReturnsError(t *testing.T) {
	c := helpers.NewCity(t, testEnv)
	c.Init("claude")

	out, err := c.GC("session", "bogus")
	if err == nil {
		t.Fatal("expected error for 'gc session bogus', got success")
	}
	if !strings.Contains(out, "unknown subcommand") {
		t.Errorf("expected 'unknown subcommand' message, got:\n%s", out)
	}
}

// --- gc session list ---

func TestSessionList_EmptyCity_ShowsNoSessions(t *testing.T) {
	c := helpers.NewCity(t, testEnv)
	c.Init("claude")

	out, err := c.GC("session", "list")
	if err != nil {
		t.Fatalf("gc session list: %v\n%s", err, out)
	}
	if !strings.Contains(out, "No sessions") {
		t.Errorf("expected 'No sessions' message on fresh city, got:\n%s", out)
	}
}

func TestSessionList_JSON_ReturnsValidOutput(t *testing.T) {
	c := helpers.NewCity(t, testEnv)
	c.Init("claude")

	out, err := c.GC("session", "list", "--json")
	if err != nil {
		t.Fatalf("gc session list --json: %v\n%s", err, out)
	}
	trimmed := strings.TrimSpace(out)
	// Empty list should be a JSON array.
	if trimmed != "[]" && trimmed != "null" {
		// At minimum it should start with [ (array) or be "null".
		if !strings.HasPrefix(trimmed, "[") {
			t.Errorf("expected JSON array for empty session list, got:\n%s", out)
		}
	}
}

// --- gc session prune ---

func TestSessionPrune_EmptyCity_ShowsNone(t *testing.T) {
	c := helpers.NewCity(t, testEnv)
	c.Init("claude")

	out, err := c.GC("session", "prune")
	if err != nil {
		t.Fatalf("gc session prune: %v\n%s", err, out)
	}
	// With no sessions, prune should indicate nothing to do.
	if !strings.Contains(strings.ToLower(out), "no sessions") && !strings.Contains(out, "0") {
		t.Errorf("expected indication of no sessions to prune, got:\n%s", out)
	}
}

// --- gc session new ---

func TestSessionNew_NonexistentTemplate_ReturnsError(t *testing.T) {
	c := helpers.NewCity(t, testEnv)
	c.Init("claude")

	_, err := c.GC("session", "new", "nonexistent-agent-xyz")
	if err == nil {
		t.Fatal("expected error for nonexistent template, got success")
	}
}

func TestSessionNew_MissingTemplate_ReturnsError(t *testing.T) {
	c := helpers.NewCity(t, testEnv)
	c.Init("claude")

	// cobra.ExactArgs(1) handles the missing argument — just verify it errors.
	_, err := c.GC("session", "new")
	if err == nil {
		t.Fatal("expected error for 'gc session new' without template, got success")
	}
}

// --- gc session close/kill/wake on nonexistent sessions ---

func TestSessionClose_NonexistentSession_ReturnsError(t *testing.T) {
	c := helpers.NewCity(t, testEnv)
	c.Init("claude")

	_, err := c.GC("session", "close", "nonexistent-session")
	if err == nil {
		t.Fatal("expected error for closing nonexistent session, got success")
	}
}

func TestSessionKill_NonexistentSession_ReturnsError(t *testing.T) {
	c := helpers.NewCity(t, testEnv)
	c.Init("claude")

	_, err := c.GC("session", "kill", "nonexistent-session")
	if err == nil {
		t.Fatal("expected error for killing nonexistent session, got success")
	}
}

func TestSessionWake_NonexistentSession_ReturnsError(t *testing.T) {
	c := helpers.NewCity(t, testEnv)
	c.Init("claude")

	_, err := c.GC("session", "wake", "nonexistent-session")
	if err == nil {
		t.Fatal("expected error for waking nonexistent session, got success")
	}
}

func TestSessionPeek_NonexistentSession_ReturnsError(t *testing.T) {
	c := helpers.NewCity(t, testEnv)
	c.Init("claude")

	_, err := c.GC("session", "peek", "nonexistent-session")
	if err == nil {
		t.Fatal("expected error for peeking nonexistent session, got success")
	}
}

func TestSessionRename_NonexistentSession_ReturnsError(t *testing.T) {
	c := helpers.NewCity(t, testEnv)
	c.Init("claude")

	_, err := c.GC("session", "rename", "nonexistent-session", "new-name")
	if err == nil {
		t.Fatal("expected error for renaming nonexistent session, got success")
	}
}
