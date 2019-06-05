package client

type RegistryStatus struct {
	Status string `json:"status"`
}

type QuayConfig struct {
	Config map[string]interface{} `json:"config"`
}

type QuayStatusResponse struct {
	Status bool   `json:"status,omitempty"`
	Reason string `json:"reason,omitempty"`
}

type SetupDatabaseResponse struct {
	Logs []LogMessage `json:"logs"`
}

type LogMessage struct {
	Message string `json:"message"`
	Level   string `json:"level"`
}

type QuayCreateSuperuserRequest struct {
	Username        string `json:"username"`
	Password        string `json:"password"`
	ConfirmPassword string `json:"confirmpassword"`
	Email           string `json:"email"`
}

type StringValue struct {
	Value string
}
