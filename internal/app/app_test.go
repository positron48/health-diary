package app

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"health-diary/internal/config"
)

func TestHealthz(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	response := httptest.NewRecorder()
	New(config.Config{}, nil).Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK || response.Body.String() != "ok\n" {
		t.Fatalf("unexpected response: status=%d body=%q", response.Code, response.Body.String())
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
