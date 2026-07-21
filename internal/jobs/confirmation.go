package jobs

import (
	"fmt"
	"strings"
	"time"

	"health-diary/internal/llm"
)

func confirmationText(events []llm.Event) string {
	lines := make([]string, 0, len(events)+2)
	lines = append(lines, "Распознано:")
	for _, event := range events {
		lines = append(lines, "• "+eventDescription(event))
	}
	lines = append(lines, "Подтвердите весь список, только если всё верно.")
	return strings.Join(lines, "\n")
}

func eventDescription(event llm.Event) string {
	data := event.Data
	title := map[string]string{
		"pain_observation":  "Боль",
		"medication_intake": "Приём лекарства",
		"wellbeing":         "Самочувствие",
		"activity":          "Активность",
		"sleep":             "Сон",
		"food_drink":        "Еда или напиток",
		"measurement":       "Измерение",
		"note":              "Заметка",
	}[event.Kind]
	if event.Kind == "pain_observation" && stringValue(data["symptom_type"]) == "headache" {
		title = "Головная боль"
	}
	parts := []string{}
	if event.Kind == "medication_intake" {
		if name := firstNonEmpty(stringValue(data["name_raw"]), stringValue(data["normalized_name"])); name != "" {
			parts = append(parts, name)
		}
		if dose := stringValue(data["dose_value"]); dose != "" {
			parts = append(parts, strings.TrimSpace(dose+" "+stringValue(data["dose_unit"])))
		}
	}
	if event.Kind == "pain_observation" {
		if intensity := stringValue(data["intensity"]); intensity != "" {
			parts = append(parts, intensity+"/10")
		}
	}
	if event.OccurredAt != "" {
		if occurredAt, err := time.Parse(time.RFC3339, event.OccurredAt); err == nil {
			prefix := "в"
			if event.TimePrecision != "exact" {
				prefix = "около"
			}
			parts = append(parts, prefix+" "+occurredAt.UTC().Format("15:04"))
		}
	}
	if len(parts) == 0 {
		return title
	}
	return title + " — " + strings.Join(parts, ", ")
}

func stringValue(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case float64:
		return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.3f", typed), "0"), ".")
	case int:
		return fmt.Sprintf("%d", typed)
	default:
		return ""
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
