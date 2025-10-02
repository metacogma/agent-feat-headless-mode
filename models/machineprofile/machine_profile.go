package machineprofile

import (
	"time"

	"agent/errors"
)

type MachineProfile struct {
	ID             string    `json:"id" bson:"id,omitempty"`
	Name           string    `json:"name" bson:"name,omitempty"`
	TestLab        string    `json:"testLab" bson:"test_lab,omitempty"`
	OS             string    `json:"os" bson:"os,omitempty"`
	OSVersion      string    `json:"osVersion" bson:"os_version,omitempty"`
	BrowserName    string    `json:"browserName" bson:"browser_name,omitempty"`
	BrowserVersion string    `json:"browserVersion" bson:"browser_version,omitempty"`
	Resolution     string    `json:"resolution" bson:"resolution,omitempty"`
	CreatedAt      time.Time `json:"createdAt" bson:"created_at,omitempty"`
	CreatedBy      string    `json:"createdBy" bson:"created_by,omitempty"`
}

func (t *MachineProfile) Validate() error {
	ve := errors.ValidationErrs()
	if t.ID == "" {
		ve.Add("id", "cannot be empty")
	}
	if t.Name == "" {
		ve.Add("name", "cannot be empty")
	}
	if t.TestLab == "" {
		ve.Add("testlab", "cannot be empty")
	}
	if t.OS == "" {
		ve.Add("os", "cannot be emtpy")
	}
	if t.OSVersion == "" {
		ve.Add("os version", "cannot be empty")
	}
	if t.BrowserName == "" {
		ve.Add("browser name", "cannot be empty")
	}
	if t.BrowserVersion == "" {
		ve.Add("browser version", "cannot be empty")
	}
	if t.Resolution == "" {
		ve.Add("resolution", "cannot be empty")
	}
	return ve.Err()
}
