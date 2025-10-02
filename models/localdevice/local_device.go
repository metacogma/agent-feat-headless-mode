package localdevice

import (
	"agent/errors"
)

type Config struct {
	MachineId string                 `json:"machine_id,omitempty" bson:"machine_id,omitempty"`
	Config    map[string]interface{} `json:"config" bson:"config"`
	Active    bool                   `json:"active" bson:"active"`
}

func (t *Config) Validate() error {
	ve := errors.ValidationErrs()

	if t.MachineId == "" {
		ve.Add("machine id", "cannot be empty")
	}

	return ve.Err()
}
