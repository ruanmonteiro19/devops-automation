package model

// Credentials are encrypted separately from the public device profile.
type SNMPCredentials struct {
	Community       string `json:"community,omitempty"`
	Username        string `json:"username,omitempty"`
	SecurityLevel   string `json:"security_level,omitempty"`
	AuthProtocol    string `json:"auth_protocol,omitempty"`
	AuthPassword    string `json:"auth_password,omitempty"`
	PrivacyProtocol string `json:"privacy_protocol,omitempty"`
	PrivacyPassword string `json:"privacy_password,omitempty"`
	ContextName     string `json:"context_name,omitempty"`
}
type NetworkItem struct {
	Change     string  `json:"change,omitempty"`
	Name       string  `json:"name"`
	Key        string  `json:"key"`
	OID        string  `json:"oid"`
	Units      string  `json:"units,omitempty"`
	Multiplier float64 `json:"multiplier"`
	Supported  bool    `json:"supported"`
	Reason     string  `json:"reason,omitempty"`
}
type NetworkCondition struct {
	Macro   string `json:"macro"`
	Pattern string `json:"pattern"`
	Negate  bool   `json:"negate"`
}
type NetworkDiscovery struct {
	FilterMode string             `json:"filter_mode,omitempty"`
	Conditions []NetworkCondition `json:"conditions,omitempty"`
	Name       string             `json:"name"`
	Columns    map[string]string  `json:"columns"`
	Items      []NetworkItem      `json:"items"`
	Supported  bool               `json:"supported"`
	Reason     string             `json:"reason,omitempty"`
}
type NetworkTemplate struct {
	ID          string             `json:"id"`
	Name        string             `json:"name"`
	Version     string             `json:"version,omitempty"`
	SourceSHA   string             `json:"source_sha,omitempty"`
	Items       []NetworkItem      `json:"items"`
	Discovery   []NetworkDiscovery `json:"discovery"`
	Warnings    []string           `json:"warnings"`
	Supported   int                `json:"supported"`
	Unsupported int                `json:"unsupported"`
	Builtin     bool               `json:"builtin"`
}
