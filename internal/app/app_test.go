package app

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"health-diary/internal/config"
)

func TestHealthz(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	response := httptest.NewRecorder()
	New(config.Config{}, nil).Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK || response.Body.String() != "ok\n" {
		t.Fatalf("unexpected response: status=%d body=%q", response.Code, response.Body.String())
	}
	if response.Header().Get("Content-Security-Policy") == "" || response.Header().Get("X-Frame-Options") != "DENY" {
		t.Fatalf("security headers were not set: %#v", response.Header())
	}
}

func TestUserLocationFallsBackToProductDefault(t *testing.T) {
	if got := userLocation("").String(); got != "Europe/Moscow" {
		t.Fatalf("empty timezone resolved to %q", got)
	}
	if got := userLocation("not/a-timezone").String(); got != "Europe/Moscow" {
		t.Fatalf("invalid timezone resolved to %q", got)
	}
	if got := userLocation("UTC"); got != time.UTC {
		t.Fatalf("valid timezone resolved to %q", got)
	}
}

func TestReadyzWithoutDatabaseIsUnavailable(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	response := httptest.NewRecorder()
	New(config.Config{}, nil).Handler().ServeHTTP(response, request)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d, want %d", response.Code, http.StatusServiceUnavailable)
	}
}

func TestSPARouteFallsBackToIndex(t *testing.T) {
	handler := New(config.Config{}, nil).Handler()
	for _, path := range []string{"/calendar", "/calendar?day=2026-07-19", "/calendar/2026-07", "/events/some-id/edit", "/analytics"} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("%s: status=%d, want %d", path, response.Code, http.StatusOK)
		}
		if contentType := response.Header().Get("Content-Type"); contentType != "text/html; charset=utf-8" {
			t.Fatalf("%s: content type=%q", path, contentType)
		}
	}
}

func TestRootAPIAliasesAreRemoved(t *testing.T) {
	handler := New(config.Config{}, nil).Handler()
	for _, path := range []string{"/calendar", "/events", "/analytics/summary", "/exports", "/batches/pending"} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if contentType := response.Header().Get("Content-Type"); !strings.Contains(contentType, "text/html") {
			t.Fatalf("%s should be SPA HTML, got content-type %q status=%d", path, contentType, response.Code)
		}
	}
}
