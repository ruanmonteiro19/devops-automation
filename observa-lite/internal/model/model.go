package model

import "time"

// Discovery is a point-in-time passive observation. All relationships remain
// observations until resolved by the inventory/correlation engine.
type Discovery struct {
	HostID      string       `json:"host_id"`
	Hostname    string       `json:"hostname"`
	OS          string       `json:"os"`
	Addresses   []string     `json:"addresses"`
	CapturedAt  time.Time    `json:"captured_at"`
	HostGroup string `json:"host_group,omitempty"`
	NetworkZone string `json:"network_zone,omitempty"`
	MonitoringMode string `json:"monitoring_mode,omitempty"`
	Processes   []Process    `json:"processes"`
	Connections []Connection `json:"connections"`
}
type Process struct {
	PID      int    `json:"pid"`
	Name     string `json:"name"`
	Service  string `json:"service"`
	Runtime  string `json:"runtime"`
	RSSBytes uint64 `json:"rss_bytes"`
}
type Connection struct {
	PID        int    `json:"pid"`
	LocalIP    string `json:"local_ip"`
	LocalPort  int    `json:"local_port"`
	RemoteIP   string `json:"remote_ip"`
	RemotePort int    `json:"remote_port"`
	State      string `json:"state"`
}
type Host struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	OS          string       `json:"os"`
	Addresses   []string     `json:"addresses"`
	LastSeen    time.Time    `json:"last_seen"`
	HostGroup string `json:"host_group,omitempty"`
	NetworkZone string `json:"network_zone,omitempty"`
	MonitoringMode string `json:"monitoring_mode,omitempty"`
	Processes   []Process    `json:"processes"`
	Connections []Connection `json:"connections"`
}
type Node struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	Host      string `json:"host,omitempty"`
	Runtime   string `json:"runtime,omitempty"`
	Status    string `json:"status"`
	RSSBytes  uint64 `json:"rss_bytes,omitempty"`
	Instances int    `json:"instances,omitempty"`
}
type Edge struct {
	ID         string `json:"id"`
	From       string `json:"from"`
	To         string `json:"to"`
	Kind       string `json:"kind"`
	Count      int    `json:"count"`
	Evidence   string `json:"evidence"`
	Confidence string `json:"confidence"`
}
type Map struct {
	Nodes []Node `json:"nodes"`
	Edges []Edge `json:"edges"`
}
type Event struct {
	ID       string    `json:"id"`
	At       time.Time `json:"at"`
	Severity string    `json:"severity"`
	Entity   string    `json:"entity"`
	Summary  string    `json:"summary"`
	Evidence []string  `json:"evidence"`
	Status   string    `json:"status"`
}
type Template struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	Kind            string    `json:"kind"`
	Target          string    `json:"target"`
	IntervalSeconds int       `json:"interval_seconds"`
	TimeoutSeconds  int       `json:"timeout_seconds"`
	ExpectedStatus  int       `json:"expected_status,omitempty"`
	Enabled         bool      `json:"enabled"`
	LastRun         time.Time `json:"last_run,omitempty"`
	LastValue       float64   `json:"last_value"`
	LastError       string    `json:"last_error,omitempty"`
	Status          string    `json:"status"`
}
type DataSource struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	URL       string `json:"url"`
	SecretEnv string `json:"secret_env,omitempty"`
}
type Panel struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Kind     string `json:"kind"`
	SourceID string `json:"source_id,omitempty"`
	Query    string `json:"query,omitempty"`
	HTML     string `json:"html,omitempty"`
	CSS      string `json:"css,omitempty"`
	JS       string `json:"js,omitempty"`
}
type Dashboard struct {
	ID     string  `json:"id"`
	Title  string  `json:"title"`
	Panels []Panel `json:"panels"`
}

// Snapshot is a safe, derived projection for the frontend and MCP.
type Snapshot struct {
	GeneratedAt time.Time    `json:"generated_at"`
	Hosts       []Host       `json:"hosts"`
	ServiceMap  Map          `json:"service_map"`
	ProcessMap  Map          `json:"process_map"`
	NetworkMap  Map          `json:"network_map"`
	Templates   []Template   `json:"templates"`
	DataSources []DataSource `json:"datasources"`
	Dashboards  []Dashboard  `json:"dashboards"`
	Events      []Event      `json:"events"`
}

 // GatewayEndpoint is configured by an administrator. Never accept destinations from untrusted agents.
 type GatewayEndpoint struct {
   ID string `json:"id"`
   NetworkZone string `json:"network_zone"`
   URL string `json:"url"`
   Priority int `json:"priority"`
   Enabled bool `json:"enabled"`
 }
 type ConnectionInfo struct {
   NetworkZone string `json:"network_zone"`
   Endpoints []string `json:"endpoints"`
   DefaultZoneFallback bool `json:"default_zone_fallback"`
   ValidForSeconds int `json:"valid_for_seconds"`
 }
