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
	if event.Kind == "pain_observation" {
		switch stringValue(data["phase"]) {
		case "start":
			parts = append(parts, "началась")
		case "update":
			parts = append(parts, "наблюдение")
		case "end":
			parts = append(parts, "прошла")
		}
		if intensity := stringValue(data["intensity"]); intensity != "" {
			parts = append(parts, intensity+"/10")
		}
		if locs := locationLabels(data["locations"]); locs != "" {
			parts = append(parts, locs)
		}
		if laterality := lateralityLabel(stringValue(data["laterality"])); laterality != "" {
			parts = append(parts, laterality)
		}
	}
	if event.Kind == "medication_intake" {
		if name := firstNonEmpty(stringValue(data["name_raw"]), stringValue(data["normalized_name"]), stringValue(data["name"])); name != "" {
			parts = append(parts, name)
		}
		if dose := stringValue(data["dose_value"]); dose != "" {
			parts = append(parts, strings.TrimSpace(dose+" "+stringValue(data["dose_unit"])))
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

func locationLabels(value any) string {
	items := []string{}
	switch typed := value.(type) {
	case []string:
		items = typed
	case []any:
		for _, item := range typed {
			if text, ok := item.(string); ok {
				items = append(items, text)
			}
		}
	}
	labels := make([]string, 0, len(items))
	for _, item := range items {
		labels = append(labels, locationLabel(item))
	}
	return strings.Join(labels, ", ")
}

func locationLabel(value string) string {
	switch value {
	case "top_of_head", "upper_head", "head_top":
		return "верхняя часть головы"
	case "occiput_neck", "occiput", "neck":
		return "затылок/шея"
	case "temple", "temporal":
		return "висок"
	case "forehead", "frontal":
		return "лоб"
	case "right_side":
		return "правая сторона"
	case "left_side":
		return "левая сторона"
	case "head":
		return "голова"
	default:
		return value
	}
}

func lateralityLabel(value string) string {
	switch value {
	case "left":
		return "слева"
	case "right":
		return "справа"
	case "bilateral":
		return "с обеих сторон"
	case "center":
		return "по центру"
	default:
		return ""
	}
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
