package model

import "time"

// SLOPolicy evaluates the retained request sample, not wall-clock uptime.
type SLOPolicy struct {
	ID                 string  `json:"id"`
	Name               string  `json:"name"`
	ServiceID          string  `json:"service_id"`
	Kind               string  `json:"kind"`
	TargetPercent      float64 `json:"target_percent"`
	LatencyThresholdMS float64 `json:"latency_threshold_ms,omitempty"`
}

type SLOEvaluation struct {
	SLOPolicy
	From                        time.Time `json:"from"`
	To                          time.Time `json:"to"`
	Total                       int       `json:"total"`
	Good                        int       `json:"good"`
	Bad                         int       `json:"bad"`
	SuccessPercent              *float64  `json:"success_percent"`
	ErrorBudgetRemainingPercent *float64  `json:"error_budget_remaining_percent"`
	BurnRate                    *float64  `json:"burn_rate"`
	Status                      string    `json:"status"`
	Basis                       string    `json:"basis"`
}
