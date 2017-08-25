package types

type Container struct {
	Dns                    []string               `json:"dns,omitempty"`
	DnsSearch              []string               `json:"dns_search,omitempty"`
	EnvironmentUuid        string                 `json:"environment_uuid,omitempty"`
	ExitCode               int64                  `json:"exit_code,omitempty"`
	ExternalId             string                 `json:"external_id,omitempty"`
	HealthCheck            HealthcheckInfo        `json:"health_check,omitempty"`
	HealthCheckHosts       []HealthcheckState     `json:"health_check_hosts,omitempty"`
	HealthState            string                 `json:"health_state,omitempty"`
	HostId                 string                  `json:"host_id,omitempty"`
	Hostname               string                 `json:"hostname,omitempty"`
	Labels                 map[string]interface{} `json:"labels,omitempty"`
	Links                  []Link                 `json:"links,omitempty"`
	MemoryReservation      int64                  `json:"memory_reservation,omitempty"`
	MilliCpuReservation    int64                  `json:"milli_cpu_reservation,omitempty"`
	Name                   string                 `json:"name,omitempty"`
	NativeContainer        bool                   `json:"native_container,omitempty"`
	NetworkFromContainerId string                  `json:"network_from_container_id,omitempty"`
	NetworkId              string                  `json:"network_id,omitempty"`
	Ports                  []PublicEndpoint       `json:"ports,omitempty"`
	PrimaryIp              string                 `json:"primary_ip,omitempty"`
	PrimaryMacAddress      string                 `json:"primary_mac_address,omitempty"`
	ServiceId              string                  `json:"service_id,omitempty"`
	ServiceIds             []string                `json:"service_ids,omitempty"`
	ServiceIndex           int64                  `json:"service_index,omitempty"`
	ShouldRestart          bool                   `json:"should_restart,omitempty"`
	StackId                string                  `json:"stack_id,omitempty"`
	StartCount             int64                  `json:"start_count,omitempty"`
	State                  string                 `json:"state,omitempty"`
	Uuid                   string                 `json:"uuid,omitempty"`
}

type HealthcheckState struct {
	HealthState string `json:"health_state,omitempty"`
	HostId      string `json:"host_id,omitempty"`
}

type Link struct {
	Alias string `json:"alias,omitempty"`
	Name  string `json:"name,omitempty"`
}

type HealthcheckInfo struct {
	HealthyThreshold    int64  `json:"healthy_threshold,omitempty"`
	InitializingTimeout int64  `json:"initializing_timeout,omitempty"`
	Interval            int64  `json:"interval,omitempty"`
	Port                int64  `json:"port,omitempty"`
	RequestLine         string `json:"request_line,omitempty"`
	ResponseTimeout     int64  `json:"response_timeout,omitempty"`
	UnhealthyThreshold  int64  `json:"unhealthy_threshold,omitempty"`
}

type PublicEndpoint struct {
	AgentIpAddress string `json:"agent_ip_address,omitempty"`
	BindAll        bool   `json:"bind_all,omitempty"`
	BindIpAddress  string `json:"bind_ip_address,omitempty"`
	Fqdn           string `json:"fqdn,omitempty"`
	HostId         string `json:"host_id,omitempty"`
	InstanceId     string `json:"instance_id,omitempty"`
	IpAddress      string `json:"ip_address,omitempty"`
	PrivatePort    int64  `json:"private_port,omitempty"`
	Protocol       string `json:"protocol,omitempty"`
	PublicPort     int64  `json:"public_port,omitempty"`
	ServiceId      string `json:"service_id,omitempty"`
}

func (c *Container) GetEnvironmentUUID() string {
	return c.EnvironmentUuid
}

func (c *Container) GetServiceID() string {
	return c.ServiceId
}
