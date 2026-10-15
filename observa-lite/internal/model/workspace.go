package model

import "time"

type FrontendCandidate struct {
	InjectionStatus string    `json:"injection_status,omitempty"`
	ID              string    `json:"id"`
	HostID          string    `json:"host_id"`
	HostName        string    `json:"host_name"`
	Service         string    `json:"service"`
	PID             int       `json:"pid"`
	Port            int       `json:"port"`
	Scheme          string    `json:"scheme"`
	Evidence        string    `json:"evidence"`
	ObservedAt      time.Time `json:"observed_at"`
}
type SoftwarePackage struct {
	Name      string `json:"name"`
	Version   string `json:"version"`
	Ecosystem string `json:"ecosystem"`
	Source    string `json:"source,omitempty"`
}
type NotebookCell struct {
	ID      string `json:"id"`
	Kind    string `json:"kind"`
	Content string `json:"content"`
}
type WorkflowStep struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Kind     string `json:"kind"`
	Query    string `json:"query,omitempty"`
	TargetID string `json:"target_id,omitempty"`
	MinRows  int    `json:"min_rows,omitempty"`
}
type WorkspaceObject struct {
	ID              string            `json:"id"`
	Name            string            `json:"name"`
	Kind            string            `json:"kind"`
	Cells           []NotebookCell    `json:"cells,omitempty"`
	Steps           []WorkflowStep    `json:"steps,omitempty"`
	Enabled         bool              `json:"enabled"`
	IntervalSeconds int               `json:"interval_seconds,omitempty"`
	Engine          string            `json:"engine,omitempty"`
	CredentialEnv   string            `json:"credential_env,omitempty"`
	Packages        []SoftwarePackage `json:"packages,omitempty"`
	UpdatedAt       time.Time         `json:"updated_at"`
}
type OperationRun struct {
	ID         string         `json:"id"`
	ObjectID   string         `json:"object_id"`
	Kind       string         `json:"kind"`
	StartedAt  time.Time      `json:"started_at"`
	FinishedAt time.Time      `json:"finished_at"`
	Status     string         `json:"status"`
	Error      string         `json:"error,omitempty"`
	Result     map[string]any `json:"result"`
}
