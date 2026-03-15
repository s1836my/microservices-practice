package health_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yourorg/micromart/pkg/health"
)

// ── モック Checker ─────────────────────────────────────────────────────────────

type mockChecker struct {
	name string
	err  error
}

func (m *mockChecker) Name() string                       { return m.name }
func (m *mockChecker) Check(_ context.Context) error      { return m.err }

// ── Handler テスト ─────────────────────────────────────────────────────────────

func TestHandler_AllHealthy(t *testing.T) {
	h := health.Handler("1.0.0",
		&mockChecker{name: "db", err: nil},
		&mockChecker{name: "redis", err: nil},
	)
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	h(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}

	var resp health.Response
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Status != health.StatusOK {
		t.Errorf("status = %q, want %q", resp.Status, health.StatusOK)
	}
	if resp.Checks["db"] != "ok" {
		t.Errorf("db check = %q, want %q", resp.Checks["db"], "ok")
	}
	if resp.Version != "1.0.0" {
		t.Errorf("version = %q, want %q", resp.Version, "1.0.0")
	}
}

func TestHandler_OneDegraded(t *testing.T) {
	h := health.Handler("1.0.0",
		&mockChecker{name: "db", err: nil},
		&mockChecker{name: "kafka", err: fmt.Errorf("connection refused")},
	)
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	h(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", w.Code)
	}

	var resp health.Response
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Status != health.StatusDegraded {
		t.Errorf("status = %q, want %q", resp.Status, health.StatusDegraded)
	}
	if resp.Checks["kafka"] == "" {
		t.Error("kafka error message should be present")
	}
}

func TestHandler_NoCheckers(t *testing.T) {
	h := health.Handler("2.0.0")
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	h(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

func TestLivenessHandler(t *testing.T) {
	h := health.LivenessHandler()
	req := httptest.NewRequest(http.MethodGet, "/livez", nil)
	w := httptest.NewRecorder()
	h(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}

func TestNewMux_Routes(t *testing.T) {
	mux := health.NewMux("1.0.0", &mockChecker{name: "db"})

	routes := []struct {
		path       string
		wantStatus int
	}{
		{"/health", http.StatusOK},
		{"/ready", http.StatusOK},
		{"/livez", http.StatusOK},
	}

	for _, tc := range routes {
		t.Run(tc.path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)
			if w.Code != tc.wantStatus {
				t.Errorf("%s: status = %d, want %d", tc.path, w.Code, tc.wantStatus)
			}
		})
	}
}

// ── CheckerFunc テスト ────────────────────────────────────────────────────────

func TestCheckerFunc(t *testing.T) {
	called := false
	c := health.NewCheckerFunc("custom", func(_ context.Context) error {
		called = true
		return nil
	})

	if c.Name() != "custom" {
		t.Errorf("Name() = %q, want %q", c.Name(), "custom")
	}

	if err := c.Check(context.Background()); err != nil {
		t.Errorf("Check() = %v, want nil", err)
	}
	if !called {
		t.Error("expected check function to be called")
	}
}

func TestCheckerFunc_Error(t *testing.T) {
	want := fmt.Errorf("redis unreachable")
	c := health.NewCheckerFunc("redis", func(_ context.Context) error {
		return want
	})

	if err := c.Check(context.Background()); err != want {
		t.Errorf("Check() = %v, want %v", err, want)
	}
}

// ── PingChecker テスト ─────────────────────────────────────────────────────────

type mockPinger struct{ err error }

func (m *mockPinger) PingContext(_ context.Context) error { return m.err }

func TestPingChecker_Healthy(t *testing.T) {
	c := health.NewPingChecker("postgres", &mockPinger{err: nil})
	if c.Name() != "postgres" {
		t.Errorf("Name() = %q, want postgres", c.Name())
	}
	if err := c.Check(context.Background()); err != nil {
		t.Errorf("Check() = %v, want nil", err)
	}
}

func TestPingChecker_Unhealthy(t *testing.T) {
	want := fmt.Errorf("connection refused")
	c := health.NewPingChecker("postgres", &mockPinger{err: want})
	if err := c.Check(context.Background()); err != want {
		t.Errorf("Check() = %v, want %v", err, want)
	}
}
