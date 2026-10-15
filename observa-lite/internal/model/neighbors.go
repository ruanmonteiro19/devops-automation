package model

// DeviceNeighbor records the neighbor advertised by LLDP; port number is not assumed to be ifIndex.
type DeviceNeighbor struct {
	LocalPort    string `json:"local_port"`
	LocalIfIndex string `json:"local_if_index,omitempty"`
	RemoteName   string `json:"remote_name,omitempty"`
	DNSName      string `json:"dns_name,omitempty"`
	RemoteIP     string `json:"remote_ip,omitempty"`
	RemotePort   string `json:"remote_port,omitempty"`
	ChassisID    string `json:"chassis_id,omitempty"`
	Protocol     string `json:"protocol"`
}
