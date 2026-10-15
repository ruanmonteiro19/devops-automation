package model

import "time"

// Public profiles contain connection metadata; SNMP credentials are stored separately as ciphertext.
type Integration struct {
	NeighborDiscovery bool   `json:"neighbor_discovery,omitempty"`
	IntervalSeconds   int    `json:"interval_seconds,omitempty"`
	SNMPRevision      string `json:"snmp_revision,omitempty"`
	SNMPVersion       string `json:"snmp_version,omitempty"`
	SNMPPort          uint16 `json:"snmp_port,omitempty"`
	TemplateID        string `json:"template_id,omitempty"`
	DeviceType        string `json:"device_type,omitempty"`
	HostGroup         string `json:"host_group,omitempty"`
	ID                string `json:"id"`
	Name              string `json:"name"`
	Kind              string `json:"kind"`
	Enabled           bool   `json:"enabled"`
	Region            string `json:"region,omitempty"`
	Scope             string `json:"scope,omitempty"`
	CredentialPrefix  string `json:"credential_prefix,omitempty"`
	Endpoint          string `json:"endpoint,omitempty"`
	Target            string `json:"target,omitempty"`
	Module            string `json:"module,omitempty"`
	Auth              string `json:"auth,omitempty"`
}
type IntegrationResource struct {
	Neighbors  []DeviceNeighbor   `json:"neighbors,omitempty"`
	ID         string             `json:"id"`
	Name       string             `json:"name"`
	Kind       string             `json:"kind"`
	Location   string             `json:"location,omitempty"`
	Status     string             `json:"status"`
	Attributes map[string]string  `json:"attributes,omitempty"`
	Metrics    map[string]float64 `json:"metrics,omitempty"`
	Interfaces []DeviceInterface  `json:"interfaces,omitempty"`
}
type DeviceInterface struct {
	Index       string             `json:"index"`
	Name        string             `json:"name"`
	Description string             `json:"description,omitempty"`
	Metrics     map[string]float64 `json:"metrics"`
}
type IntegrationResult struct {
	SuccessAt time.Time             `json:"success_at"`
	At        time.Time             `json:"at"`
	Status    string                `json:"status"`
	Error     string                `json:"error,omitempty"`
	Truncated bool                  `json:"truncated"`
	Resources []IntegrationResource `json:"resources"`
}
