package llm

import (
	"context"
	"time"
)

type Event struct {
	ClientRef     string         `json:"client_ref"`
	Kind          string         `json:"kind"`
	OccurredAt    string         `json:"occurred_at"`
	EndedAt       *string        `json:"ended_at,omitempty"`
	TimePrecision string         `json:"time_precision"`
	Data          map[string]any `json:"data"`
}
type Result struct {
	Summary string  `json:"summary"`
	Events  []Event `json:"events"`
}
type ExtractionRequest struct {
	Text      string
	Timezone  string
	Reference time.Time
}
type Extractor interface {
	Extract(context.Context, ExtractionRequest) (Result, error)
}
