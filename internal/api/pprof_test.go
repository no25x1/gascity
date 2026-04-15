package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestServerPprofEndpointSurvives(t *testing.T) {
	state := newFakeState(t)
	ts := httptest.NewServer(New(state).handler())
	defer ts.Close()

	assertPprofEndpoint(t, ts.URL)
}

func TestSupervisorPprofEndpointSurvives(t *testing.T) {
	sm := newTestSupervisorMux(t, map[string]*fakeState{})
	ts := httptest.NewServer(sm.Handler())
	defer ts.Close()

	assertPprofEndpoint(t, ts.URL)
}

func assertPprofEndpoint(t *testing.T, baseURL string) {
	t.Helper()

	resp, err := http.Get(baseURL + "/debug/pprof/")
	if err != nil {
		t.Fatalf("GET /debug/pprof/: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("/debug/pprof/ status = %d, want 200", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read /debug/pprof/ body: %v", err)
	}
	text := string(body)
	if !strings.Contains(text, "Types of profiles available:") && !strings.Contains(text, "profile?seconds=") {
		t.Fatalf("/debug/pprof/ body missing pprof index content")
	}
}
