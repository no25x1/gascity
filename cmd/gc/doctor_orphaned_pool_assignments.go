package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/gastownhall/gascity/internal/beads"
	"github.com/gastownhall/gascity/internal/config"
	"github.com/gastownhall/gascity/internal/doctor"
	"github.com/gastownhall/gascity/internal/session"
)

type orphanedPoolAssignmentsDoctorCheck struct {
	cfg      *config.City
	cityPath string
	newStore func(string) (beads.Store, error)
}

func newOrphanedPoolAssignmentsDoctorCheck(
	cityPath string,
	cfg *config.City,
	newStore func(string) (beads.Store, error),
) *orphanedPoolAssignmentsDoctorCheck {
	return &orphanedPoolAssignmentsDoctorCheck{
		cfg:      cfg,
		cityPath: cityPath,
		newStore: newStore,
	}
}

func (*orphanedPoolAssignmentsDoctorCheck) Name() string { return "orphaned-pool-assignments" }

func (*orphanedPoolAssignmentsDoctorCheck) CanFix() bool { return false }

func (*orphanedPoolAssignmentsDoctorCheck) Fix(_ *doctor.CheckContext) error { return nil }

func (c *orphanedPoolAssignmentsDoctorCheck) Run(_ *doctor.CheckContext) *doctor.CheckResult {
	r := &doctor.CheckResult{
		Name:    c.Name(),
		Status:  doctor.StatusOK,
		Message: "no pool-routed work is stuck on terminal session beads",
	}
	if c == nil || c.cfg == nil || c.newStore == nil {
		return r
	}

	cityStore, err := c.newStore(c.cityPath)
	if err != nil {
		r.Status = doctor.StatusWarning
		r.Message = fmt.Sprintf("orphaned pool assignment diagnostics skipped: %v", err)
		return r
	}

	cityBeads, err := listDoctorStoreBeads(cityStore)
	if err != nil {
		r.Status = doctor.StatusWarning
		r.Message = fmt.Sprintf("orphaned pool assignment diagnostics skipped: %v", err)
		return r
	}

	owners := terminalPoolSessionOwnersByIdentifier(cityBeads)
	if len(owners) == 0 {
		return r
	}

	findings := collectOrphanedPoolAssignmentFindings(c.cfg, "city", owners, cityBeads)
	for _, rig := range c.cfg.Rigs {
		if rig.Suspended || strings.TrimSpace(rig.Path) == "" {
			continue
		}
		rigStore, err := c.newStore(rig.Path)
		if err != nil {
			continue
		}
		rigBeads, err := listDoctorStoreBeads(rigStore)
		if err != nil {
			continue
		}
		findings = append(findings, collectOrphanedPoolAssignmentFindings(c.cfg, rig.Name, owners, rigBeads)...)
	}
	if len(findings) == 0 {
		return r
	}

	sort.Strings(findings)
	r.Status = doctor.StatusWarning
	r.Message = summarizeOrphanedPoolAssignmentFindings(findings)
	r.Details = findings
	r.FixHint = `run "gc doctor --verbose" to list the affected work beads, then clear or reassign them manually`
	return r
}

func listDoctorStoreBeads(store beads.Store) ([]beads.Bead, error) {
	if store == nil {
		return nil, fmt.Errorf("store unavailable")
	}
	return store.List(beads.ListQuery{
		AllowScan:     true,
		IncludeClosed: true,
		Sort:          beads.SortCreatedAsc,
	})
}

func terminalPoolSessionOwnersByIdentifier(all []beads.Bead) map[string]beads.Bead {
	owners := make(map[string]beads.Bead)
	for _, b := range all {
		if b.Status == "closed" || !session.IsSessionBeadOrRepairable(b) {
			continue
		}
		if !isPoolManagedSessionBead(b) || !isPoolSessionSlotFreeable(b) {
			continue
		}
		for _, ident := range []string{
			b.ID,
			b.Metadata["session_name"],
			b.Metadata["configured_named_identity"],
		} {
			ident = strings.TrimSpace(ident)
			if ident == "" {
				continue
			}
			if _, exists := owners[ident]; !exists {
				owners[ident] = b
			}
		}
	}
	return owners
}

func collectOrphanedPoolAssignmentFindings(
	cfg *config.City,
	scope string,
	owners map[string]beads.Bead,
	all []beads.Bead,
) []string {
	if len(owners) == 0 {
		return nil
	}

	scope = strings.TrimSpace(scope)
	if scope == "" {
		scope = "city"
	}

	var findings []string
	for _, b := range all {
		if session.IsSessionBeadOrRepairable(b) {
			continue
		}
		if b.Status != "open" && b.Status != "in_progress" {
			continue
		}
		assignee := strings.TrimSpace(b.Assignee)
		if assignee == "" {
			continue
		}
		template := strings.TrimSpace(b.Metadata["gc.routed_to"])
		if template == "" {
			continue
		}
		agentCfg := findAgentByTemplate(cfg, template)
		if agentCfg == nil || !agentCfg.SupportsGenericEphemeralSessions() {
			continue
		}
		if assigneePreservesNamedSessionRoute(cfg, template, assignee) {
			continue
		}
		owner, ok := owners[assignee]
		if !ok {
			continue
		}
		findings = append(findings, fmt.Sprintf(
			"stuck-pool-owner: %s:%s routed_to %q is assigned to %q, but session bead %s (%s) is open with %s and should be freeable",
			scope,
			b.ID,
			template,
			assignee,
			owner.ID,
			orphanedPoolAssignmentOwnerLabel(owner),
			orphanedPoolAssignmentOwnerState(owner),
		))
	}
	return findings
}

func orphanedPoolAssignmentOwnerLabel(owner beads.Bead) string {
	if sessionName := strings.TrimSpace(owner.Metadata["session_name"]); sessionName != "" {
		return sessionName
	}
	if named := strings.TrimSpace(owner.Metadata["configured_named_identity"]); named != "" {
		return named
	}
	return owner.Title
}

func orphanedPoolAssignmentOwnerState(owner beads.Bead) string {
	state := strings.TrimSpace(owner.Metadata["state"])
	if state == "" {
		state = "state=unknown"
	} else {
		state = "state=" + state
	}
	if reason := strings.TrimSpace(owner.Metadata["sleep_reason"]); reason != "" {
		return state + " sleep_reason=" + reason
	}
	return state
}

func summarizeOrphanedPoolAssignmentFindings(findings []string) string {
	switch len(findings) {
	case 0:
		return ""
	case 1:
		return findings[0]
	default:
		return fmt.Sprintf("%s (and %d more)", findings[0], len(findings)-1)
	}
}
