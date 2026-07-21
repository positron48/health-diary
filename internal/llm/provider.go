package llm

import "context"

type Event struct {
	ClientRef     string         `json:"client_ref"`
	Kind          string         `json:"kind"`
	OccurredAt    string         `json:"occurred_at"`
	TimePrecision string         `json:"time_precision"`
	Data          map[string]any `json:"data"`
}
type Result struct {
	Summary string  `json:"summary"`
	Events  []Event `json:"events"`
}
type Extractor interface {
	Extract(context.Context, string) (Result, error)
}
