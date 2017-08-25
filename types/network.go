package types

type Network struct {
	DefaultPolicyAction string                 `json:"default_policy_action,omitempty"`
	EnvironmentUuid     string                 `json:"environment_uuid,omitempty"`
	HostPorts           bool                   `json:"host_ports,omitempty"`
	Metadata            map[string]interface{} `json:"metadata,omitempty"`
	Name                string                 `json:"name,omitempty"`
	Policy              interface{}            `json:"policy,omitempty"`
	Uuid                string                 `json:"uuid,omitempty"`
}

func (n *Network) GetEnvironmentUUID() string {
	return n.EnvironmentUuid
}
