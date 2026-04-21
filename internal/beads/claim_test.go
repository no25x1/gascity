package beads

import (
	"errors"
	"testing"
)

func TestIsClaimConflictMessage(t *testing.T) {
	for _, msg := range []string{
		"issue already claimed by alice",
		"bead already assigned to worker",
		"claim conflict: stale assignee",
	} {
		if !isClaimConflictMessage(msg) {
			t.Fatalf("isClaimConflictMessage(%q) = false, want true", msg)
		}
	}
	if isClaimConflictMessage("assignee column missing from database") {
		t.Fatal("generic assignee error should not be classified as conflict")
	}
}

func TestIsClaimConflict(t *testing.T) {
	if !IsClaimConflict(ErrClaimConflict) {
		t.Fatal("ErrClaimConflict should classify as claim conflict")
	}
	if IsClaimConflict(errors.New("bd not found")) {
		t.Fatal("hard bd error should not classify as claim conflict")
	}
}
