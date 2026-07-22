package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type OpenAICompatible struct {
	baseURL, model, key, timezone string
	client                        *http.Client
	now                           func() time.Time
}

func NewOpenAICompatible(baseURL, model, key string, client *http.Client) *OpenAICompatible {
	if client == nil {
		client = http.DefaultClient
	}
	return &OpenAICompatible{strings.TrimRight(baseURL, "/"), model, key, "Europe/Moscow", client, time.Now}
}

func (p *OpenAICompatible) WithTimezone(timezone string) *OpenAICompatible {
	if timezone != "" {
		p.timezone = timezone
	}
	return p
}

func (p *OpenAICompatible) Extract(ctx context.Context, input ExtractionRequest) (Result, error) {
	now := time.Now()
	if p.now != nil {
		now = p.now()
	}
	if !input.Reference.IsZero() {
		now = input.Reference
	}
	timezone := input.Timezone
	if timezone == "" {
		timezone = p.timezone
	}
	body := map[string]any{
		"model":           p.model,
		"temperature":     0,
		"response_format": map[string]string{"type": "json_object"},
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": BuildUserPrompt(input.Text, now, timezone)},
		},
	}
	raw, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/chat/completions", bytes.NewReader(raw))
	if err != nil {
		return Result{}, err
	}
	req.Header.Set("Authorization", "Bearer "+p.key)
	req.Header.Set("Content-Type", "application/json")
	resp, err := p.client.Do(req)
	if err != nil {
		return Result{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return Result{}, fmt.Errorf("provider returned %d", resp.StatusCode)
	}
	var wire struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&wire); err != nil {
		return Result{}, err
	}
	if len(wire.Choices) != 1 {
		return Result{}, fmt.Errorf("provider returned no completion")
	}
	var result Result
	content := strings.TrimSpace(wire.Choices[0].Message.Content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(strings.TrimSpace(content), "```")
	if err := json.Unmarshal([]byte(strings.TrimSpace(content)), &result); err != nil {
		return Result{}, fmt.Errorf("invalid provider JSON: %w", err)
	}
	if err := ValidateResult(result); err != nil {
		return Result{}, fmt.Errorf("invalid provider result: %w", err)
	}
	return result, nil
}
