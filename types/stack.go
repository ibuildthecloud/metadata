package types

type Stack struct {
	EnvironmentUuid string `json:"environment_uuid,omitempty"`
	HealthState     string `json:"health_state,omitempty"`
	Name            string `json:"name,omitempty"`
	Uuid            string `json:"uuid,omitempty"`
}

func (s *Stack) GetEnvironmentUUID() string {
	return s.EnvironmentUuid
}
