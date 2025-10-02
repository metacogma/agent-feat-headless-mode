package testlab

import (
	"os"

	"agent/models/machineprofile"
)

type TestLabConfig interface {
	GetName() string
	WriteConfigToFile(*os.File) error
	SetMachine(machineprofile.MachineProfile)
	Validate() error
}

type TestLabDetails struct {
}
