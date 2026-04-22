package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gastownhall/gascity/internal/beads"
	"github.com/gastownhall/gascity/internal/config"
	"github.com/gastownhall/gascity/internal/doctor"
	"github.com/gastownhall/gascity/internal/session"
)

func TestOrphanedPoolAssignmentsDoctorCheckReportsFreeableSessionOwner(t *testing.T) {
	cityPath := t.TempDir()
	rigPath := filepath.Join(cityPath, "repo")

	cityStore := beads.NewMemStore()
	rigStore := beads.NewMemStore()

	_, err := cityStore.Create(beads.Bead{
		Type:   session.BeadType,
		Labels: []string{session.LabelSession},
		Status: "open",
		Title:  "dog session",
		Metadata: map[string]string{
			"session_name":         "dog-gc-dead",
			"template":             "dog",
			"agent_name":           "dog",
			"state":                "drained",
			poolManagedMetadataKey: boolMetadata(true),
		},
	})
	if err != nil {
		t.Fatalf("create session bead: %v", err)
	}

	_, err = rigStore.Create(beads.Bead{
		Type:     "task",
		Status:   "in_progress",
		Title:    "stuck dog work",
		Assignee: "dog-gc-dead",
		Metadata: map[string]string{
			"gc.routed_to": "dog",
		},
	})
	if err != nil {
		t.Fatalf("create work bead: %v", err)
	}

	check := &orphanedPoolAssignmentsDoctorCheck{
		cfg: &config.City{
			Agents: []config.Agent{{Name: "dog", MaxActiveSessions: intPtr(2)}},
			Rigs:   []config.Rig{{Name: "repo", Path: rigPath}},
		},
		cityPath: cityPath,
		newStore: func(path string) (beads.Store, error) {
			switch path {
			case cityPath:
				return cityStore, nil
			case rigPath:
				return rigStore, nil
			default:
				return nil, filepath.ErrBadPattern
			}
		},
	}

	got := check.Run(&doctor.CheckContext{CityPath: cityPath, Verbose: true})
	if got.Status != doctor.StatusWarning {
		t.Fatalf("status = %v, want warning", got.Status)
	}
	if !strings.Contains(got.Message, "stuck-pool-owner") {
		t.Fatalf("message = %q, want stuck-pool-owner summary", got.Message)
	}
	if len(got.Details) != 1 {
		t.Fatalf("details = %v, want 1 finding", got.Details)
	}
	if !strings.Contains(got.Details[0], "stuck-pool-owner") {
		t.Fatalf("detail = %q, want stuck-pool-owner marker", got.Details[0])
	}
	if !strings.Contains(got.Details[0], "repo:") {
		t.Fatalf("detail = %q, want rig scope prefix", got.Details[0])
	}
}

func TestOrphanedPoolAssignmentsDoctorCheckSkipsActiveSessionOwner(t *testing.T) {
	cityPath := t.TempDir()

	cityStore := beads.NewMemStore()
	_, err := cityStore.Create(beads.Bead{
		Type:   session.BeadType,
		Labels: []string{session.LabelSession},
		Status: "open",
		Title:  "dog session",
		Metadata: map[string]string{
			"session_name":         "dog-gc-live",
			"template":             "dog",
			"agent_name":           "dog",
			"state":                "active",
			poolManagedMetadataKey: boolMetadata(true),
		},
	})
	if err != nil {
		t.Fatalf("create session bead: %v", err)
	}

	_, err = cityStore.Create(beads.Bead{
		Type:     "task",
		Status:   "in_progress",
		Title:    "active dog work",
		Assignee: "dog-gc-live",
		Metadata: map[string]string{
			"gc.routed_to": "dog",
		},
	})
	if err != nil {
		t.Fatalf("create work bead: %v", err)
	}

	check := &orphanedPoolAssignmentsDoctorCheck{
		cfg: &config.City{
			Agents: []config.Agent{{Name: "dog", MaxActiveSessions: intPtr(2)}},
		},
		cityPath: cityPath,
		newStore: func(path string) (beads.Store, error) {
			if path != cityPath {
				return nil, filepath.ErrBadPattern
			}
			return cityStore, nil
		},
	}

	got := check.Run(&doctor.CheckContext{CityPath: cityPath})
	if got.Status != doctor.StatusOK {
		t.Fatalf("status = %v, want OK", got.Status)
	}
}

func TestDoDoctorReportsOrphanedPoolAssignments(t *testing.T) {
	cityPath, store := newPhase0DoctorCityWithConfig(t, `[workspace]
name = "test-city"

[beads]
provider = "file"

[[agent]]
name = "dog"
start_command = "true"
max_active_sessions = 2
`)

	_, err := store.Create(beads.Bead{
		Type:   session.BeadType,
		Labels: []string{session.LabelSession},
		Status: "open",
		Title:  "dog session",
		Metadata: map[string]string{
			"session_name":         "dog-gc-dead",
			"template":             "dog",
			"agent_name":           "dog",
			"state":                "drained",
			poolManagedMetadataKey: boolMetadata(true),
		},
	})
	if err != nil {
		t.Fatalf("create session bead: %v", err)
	}

	_, err = store.Create(beads.Bead{
		Type:     "task",
		Status:   "in_progress",
		Title:    "stuck dog work",
		Assignee: "dog-gc-dead",
		Metadata: map[string]string{
			"gc.routed_to": "dog",
		},
	})
	if err != nil {
		t.Fatalf("create work bead: %v", err)
	}

	t.Setenv("GC_CITY", cityPath)
	var stdout, stderr bytes.Buffer
	_ = doDoctor(false, true, &stdout, &stderr)

	out := stdout.String() + stderr.String()
	if !strings.Contains(out, "orphaned-pool-assignments") {
		t.Fatalf("doctor output missing orphaned-pool-assignments check:\n%s", out)
	}
	if !strings.Contains(out, "stuck-pool-owner") {
		t.Fatalf("doctor output missing stuck-pool-owner detail:\n%s", out)
	}
}
