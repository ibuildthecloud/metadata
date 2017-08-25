package types

type Host struct {
	AgentIp         string                 `json:"agent_ip,omitempty"`
	AgentState      string                 `json:"agent_state,omitempty"`
	EnvironmentUuid string                 `json:"environment_uuid,omitempty"`
	Hostname        string                 `json:"hostname,omitempty"`
	Labels          map[string]interface{} `json:"labels,omitempty"`
	Memory          int64                  `json:"memory,omitempty"`
	MilliCpu        int64                  `json:"milli_cpu,omitempty"`
	Name            string                 `json:"name,omitempty"`
	Ports           []PublicEndpoint       `json:"ports,omitempty"`
	State           string                 `json:"state,omitempty"`
	Uuid            string                 `json:"uuid,omitempty"`
}

func (h *Host) GetEnvironmentUUID() string {
	return h.EnvironmentUuid
}
