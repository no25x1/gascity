package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/gastownhall/gascity/internal/beads"
	"github.com/spf13/cobra"
)

func newWorkCmd(stdout, stderr io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "work",
		Short: "Manage runnable work for agents",
	}
	cmd.AddCommand(newWorkClaimNextCmd(stdout, stderr))
	return cmd
}

func newWorkClaimNextCmd(stdout, stderr io.Writer) *cobra.Command {
	var template string
	var assignee string
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "claim-next",
		Short: "Atomically claim the next routed work item",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if strings.TrimSpace(template) == "" {
				template = os.Getenv("GC_TEMPLATE")
			}
			if strings.TrimSpace(assignee) == "" {
				assignee = firstNonEmptyWorkEnv(os.Getenv("GC_SESSION_ID"), os.Getenv("GC_SESSION_NAME"), os.Getenv("GC_ALIAS"), os.Getenv("GC_AGENT"))
			}
			if strings.TrimSpace(template) == "" || strings.TrimSpace(assignee) == "" {
				return fmt.Errorf("claim-next requires --template and --assignee (or GC_TEMPLATE/GC_SESSION_ID env)")
			}
			cityPath, err := resolveCity()
			if err != nil {
				return err
			}
			store, err := openCityStoreAt(cityPath)
			if err != nil {
				return err
			}
			result, err := claimNextWork(cmd.Context(), store, cityPath, template, assignee)
			if err != nil {
				return err
			}
			if jsonOut {
				return writePreLaunchClaimResult(stdout, result)
			}
			if result.Bead.ID == "" {
				fmt.Fprintln(stdout, "no work") //nolint:errcheck
				return nil
			}
			fmt.Fprintf(stdout, "%s\n", result.Bead.ID) //nolint:errcheck
			_ = stderr
			return nil
		},
	}
	cmd.Flags().StringVar(&template, "template", "", "agent template/routing target")
	cmd.Flags().StringVar(&assignee, "assignee", "", "session identity to claim as")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "emit pre_launch JSON")
	return cmd
}

type claimNextResult struct {
	Bead   beads.Bead
	Reason string
}

func claimNextWork(ctx context.Context, store beads.Store, cityPath, template, assignee string) (claimNextResult, error) {
	existing, err := store.List(beads.ListQuery{Assignee: assignee, Status: "in_progress", Limit: 1})
	if err != nil {
		return claimNextResult{}, err
	}
	if len(existing) > 0 {
		return claimNextResult{Bead: existing[0], Reason: "existing_assignment"}, nil
	}
	candidates, err := store.List(beads.ListQuery{
		Metadata: map[string]string{"gc.routed_to": template},
		Status:   "open",
		Limit:    10,
	})
	if err != nil {
		return claimNextResult{}, err
	}
	for _, candidate := range candidates {
		if strings.TrimSpace(candidate.Assignee) != "" {
			continue
		}
		if err := claimWork(ctx, cityPath, candidate.ID, assignee); err != nil {
			if beads.IsClaimConflict(err) {
				continue
			}
			return claimNextResult{}, err
		}
		claimed, err := store.Get(candidate.ID)
		if err != nil {
			return claimNextResult{}, err
		}
		if claimed.Assignee == assignee {
			return claimNextResult{Bead: claimed, Reason: "claimed"}, nil
		}
	}
	return claimNextResult{Reason: "no_work"}, nil
}

var claimWork = beads.ClaimWithBD

func writePreLaunchClaimResult(w io.Writer, result claimNextResult) error {
	type response struct {
		Action      string            `json:"action"`
		Reason      string            `json:"reason,omitempty"`
		Env         map[string]string `json:"env,omitempty"`
		NudgeAppend string            `json:"nudge_append,omitempty"`
		Metadata    map[string]string `json:"metadata,omitempty"`
	}
	if result.Bead.ID == "" {
		return json.NewEncoder(w).Encode(response{Action: "drain", Reason: result.Reason})
	}
	return json.NewEncoder(w).Encode(response{
		Action:      "continue",
		Reason:      result.Reason,
		Env:         map[string]string{"GC_WORK_BEAD": result.Bead.ID},
		NudgeAppend: "\n\nClaimed work bead: " + result.Bead.ID,
		Metadata:    map[string]string{"pre_launch.user.claimed_work_bead": result.Bead.ID},
	})
}

func firstNonEmptyWorkEnv(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
