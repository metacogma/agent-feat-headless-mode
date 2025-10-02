package config

import (
	"os"
	// Local Packages

	apxerrors "agent/errors"
	"agent/models/localdevice"
)

var DefaultConfig = []byte(`
application: "agent"

cors:
  allowed_origins:
  - "http://*.apxor.com"
  - "https://*.apxor.com"
  - "https://internal.cors.com"
  - "https://localhost"
  - "https://localhost:3000"
  - "http://localhost:3000"

logger:
  level: "info"

listen: ":5000"

prefix: "/agent"

server_domain: "http://localhost:5476/aurora-dev/v1"

execution_service_domain: "http://localhost:9123/executor/v1"

dashboard_domain: "https://autotest.apxor.com"
`)

type ApxConfig struct {
	Application            string `koanf:"application" json:"application"`
	Logger                 Logger `koanf:"logger" json:"logger"`
	Listen                 string `koanf:"listen" json:"listen"`
	ServerDomain           string `koanf:"server_domain" json:"server_domain"`
	ExecutionServiceDomain string `koanf:"execution_service_domain" json:"execution_service_domain"`
	DashboardDomain        string `koanf:"dashboard_domain" json:"dashboard_domain"`
	Prefix                 string `koanf:"prefix" json:"prefix"`
	Hostname               string `koanf:"hostname" json:"hostname"`
	MachineId              string `koanf:"machine_id" json:"machine_id"`
	Cors                   CORS   `koanf:"cors" json:"cors"`
	ProjectId              string `koanf:"project_id" json:"project_id"`
	OrgId                  string `koanf:"org_id" json:"org_id"`
}

type CORS struct {
	AllowedOrigins []string `koanf:"allowed_origins"`
}

func (c *ApxConfig) ToLocalDevice() *localdevice.Config {
	return &localdevice.Config{
		MachineId: c.MachineId,
		Active:    false,
		Config:    map[string]interface{}{},
	}
}

type Logger struct {
	Level    string `koanf:"level"`
	HostName string `koanf:"host_name"`
}

// Validate validates the configuration
func (c *ApxConfig) Validate() error {
	ve := apxerrors.ValidationErrs()

	if c.Application == "" {
		c.Application = "bahya-go"
	}
	if c.Listen == "" {
		ve.Add("listen", "cannot be empty")
	}
	if c.Logger.Level == "" {
		ve.Add("logger.level", "cannot be empty")
	}

	if c.DashboardDomain == "" {
		ve.Add("dashboard_domain", "cannot be empty")
	}

	if c.ServerDomain == "" {
		ve.Add("server_domain", "cannot be empty")
	}
	if c.ExecutionServiceDomain == "" {
		ve.Add("execution_service_domain", "cannot be empty")
	}

	if c.Prefix == "" {
		ve.Add("prefix", "cannot be empty")
	}

	if host, err := os.Hostname(); err != nil {
		ve.Add("hostname", "invalid")
	} else {
		c.Logger.HostName = host
	}

	return ve.Err()
}
