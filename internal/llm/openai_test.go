package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) { return f(request) }

func TestOpenAICompatiblePromptRequiresSequentialClientRefs(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		body, err := io.ReadAll(request.Body)
		if err != nil {
			t.Fatal(err)
		}
		var payload struct {
			Messages []struct {
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatal(err)
		}
		prompt := payload.Messages[0].Content
		if !strings.Contains(prompt, "e1, e2, e3") {
			t.Fatal("system prompt must require deterministic sequential client_ref values")
		}
		if !strings.Contains(prompt, "exact, approximate, date_only, inferred_from_message") {
			t.Fatal("system prompt must enumerate allowed time precision values")
		}
		if !strings.Contains(prompt, "phase MUST be start, update, or end") {
			t.Fatal("system prompt must require headache phase rules")
		}
		if !strings.Contains(prompt, "name_raw") {
			t.Fatal("system prompt must require medication name_raw")
		}
		userPrompt := payload.Messages[1].Content
		if !strings.Contains(userPrompt, "Current UTC") || !strings.Contains(userPrompt, "entry") {
			t.Fatal("user prompt must include temporal context and diary text")
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"choices":[{"message":{"content":"{\"summary\":\"ok\",\"events\":[{\"client_ref\":\"e1\",\"kind\":\"note\",\"occurred_at\":\"2026-07-21T18:00:00Z\",\"time_precision\":\"exact\",\"data\":{}}]}"}}]}`)), Header: make(http.Header)}, nil
	})}
	result, err := NewOpenAICompatible("https://example.test", "test", "key", client).Extract(context.Background(), "entry")
	if err != nil {
		t.Fatal(err)
	}
	if result.Events[0].ClientRef != "e1" {
		t.Fatalf("client_ref = %q", result.Events[0].ClientRef)
	}
}
