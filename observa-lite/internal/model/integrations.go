package model

import "time"

// Profiles store environment variable names, never credentials.
type Integration struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	Kind             string `json:"kind"`
	Enabled          bool   `json:"enabled"`
	Region           string `json:"region,omitempty"`
	Scope            string `json:"scope,omitempty"`
	CredentialPrefix string `json:"credential_prefix,omitempty"`
	Endpoint         string `json:"endpoint,omitempty"`
	Target           string `json:"target,omitempty"`
	Module           string `json:"module,omitempty"`
	Auth             string `json:"auth,omitempty"`
}
type IntegrationResource struct {
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
