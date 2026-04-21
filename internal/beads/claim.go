package beads

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

var ErrClaimConflict = errors.New("bead claim conflict")

func ClaimWithBD(ctx context.Context, dir, beadID, assignee string) error {
	cmd := exec.CommandContext(ctx, "bd", "update", beadID, "--claim", "--json")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "BEADS_ACTOR="+assignee)
	out, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}
	msg := strings.TrimSpace(string(out))
	if isClaimConflictMessage(msg) {
		return fmt.Errorf("%w: %s", ErrClaimConflict, msg)
	}
	return fmt.Errorf("bd claim %s: %w: %s", beadID, err, msg)
}

func IsClaimConflict(err error) bool {
	return errors.Is(err, ErrClaimConflict)
}

func isClaimConflictMessage(msg string) bool {
	msg = strings.ToLower(msg)
	return strings.Contains(msg, "already assigned") ||
		strings.Contains(msg, "already claimed") ||
		strings.Contains(msg, "claimed by") ||
		strings.Contains(msg, "claim conflict")
}
