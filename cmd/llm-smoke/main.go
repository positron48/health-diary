package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"health-diary/internal/config"
	"health-diary/internal/llm"
)

type fixture struct {
	text  string
	kinds []string
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}
	if cfg.LLMAPIKey == "" {
		fmt.Fprintln(os.Stderr, "LLM_API_KEY is required")
		os.Exit(2)
	}
	models := strings.FieldsFunc(os.Getenv("LLM_SMOKE_MODELS"), func(r rune) bool { return r == ',' || r == ' ' })
	if len(models) == 0 {
		models = []string{cfg.LLMModel}
	}
	fixtures := []fixture{
		{"Сегодня в 15:00 болела голова 6/10, выпил ибупрофен 400 мг", []string{"pain_observation", "medication_intake"}},
		{"Тестовая запись: голова болит.", []string{"pain_observation"}},
		{"Тестовая запись: принял ибупрофен 200 мг.", []string{"medication_intake"}},
		{"Тестовая запись: заметка без симптомов.", []string{"note"}},
		{"Тестовая запись: спал с 23:30 до 07:10, просыпался один раз.", []string{"sleep"}},
		{"Тестовая запись: после прогулки 35 минут чувствую себя лучше.", []string{"activity"}},
		{"Тестовая запись: энергия 4 из 10, настроение 6 из 10.", []string{"wellbeing"}},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	failed := false
	for _, model := range models {
		extractor := llm.NewOpenAICompatible(cfg.LLMBaseURL, model, cfg.LLMAPIKey, nil)
		passed := 0
		for _, f := range fixtures {
			result, err := extractor.Extract(ctx, f.text)
			if err != nil {
				fmt.Printf("model=%s fixture=%q error=%s\n", model, f.kinds[0], errorCategory(err))
				continue
			}
			if containsKinds(result.Events, f.kinds) {
				passed++
			} else {
				fmt.Printf("model=%s fixture=%q error=unexpected_event_kind\n", model, f.kinds[0])
			}
		}
		fmt.Printf("model=%s passed=%d/%d\n", model, passed, len(fixtures))
		if passed != len(fixtures) {
			failed = true
		}
	}
	if failed {
		os.Exit(1)
	}
}

func errorCategory(err error) string {
	message := err.Error()
	if strings.HasPrefix(message, "provider returned ") {
		return message
	}
	for _, prefix := range []string{"invalid provider JSON", "provider returned no completion", "provider returned no events"} {
		if strings.HasPrefix(message, prefix) {
			return prefix
		}
	}
	return "transport_or_validation"
}

func containsKinds(events []llm.Event, expected []string) bool {
	seen := map[string]bool{}
	for _, event := range events {
		seen[event.Kind] = true
	}
	for _, kind := range expected {
		if !seen[kind] {
			return false
		}
	}
	return true
}
