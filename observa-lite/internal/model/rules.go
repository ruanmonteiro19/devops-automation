package model

import "time"

type APMRule struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	ServiceID     string  `json:"service_id"`
	Metric        string  `json:"metric"`
	Threshold     float64 `json:"threshold"`
	MinRequests   int     `json:"min_requests"`
	WindowMinutes int     `json:"window_minutes"`
	Severity      string  `json:"severity"`
	Enabled       bool    `json:"enabled"`
}
type RuleTransition struct {
	At      time.Time `json:"at"`
	State   string    `json:"state"`
	Value   float64   `json:"value"`
	Summary string    `json:"summary"`
}
type RuleProblem struct {
	ID              string           `json:"id"`
	RuleID          string           `json:"rule_id"`
	ServiceID       string           `json:"service_id"`
	Name            string           `json:"name"`
	Severity        string           `json:"severity"`
	Status          string           `json:"status"`
	OpenedAt        time.Time        `json:"opened_at"`
	UpdatedAt       time.Time        `json:"updated_at"`
	ResolvedAt      *time.Time       `json:"resolved_at,omitempty"`
	Value           float64          `json:"value"`
	Threshold       float64          `json:"threshold"`
	Metric          string           `json:"metric"`
	WindowMinutes   int              `json:"window_minutes"`
	Evidence        []string         `json:"evidence"`
	ExampleTraceIDs []string         `json:"example_trace_ids"`
	Timeline        []RuleTransition `json:"timeline"`
}
