package session

import (
	"encoding/json"
	"os"
	"strconv"
	"strings"

	"agent/errors"
	"agent/models/machineprofile"
	apxconstants "agent/utils/constants"
)

type Config struct {
	MachineName    string     `json:"machineName" bson:"machine_name"`
	MachineId      string     `json:"machineId" bson:"machine_id"`
	Browser        string     `json:"browser" bson:"browser"`
	BrowserVersion string     `json:"browserVersion" bson:"browser_version"`
	OS             string     `json:"os" bson:"os"`
	OSVersion      string     `json:"osVersion" bson:"os_version"`
	Name           string     `json:"name" bson:"name"`
	Resolution     string     `json:"resolution" bson:"resolution"`
	ConsoleLogs    string     `json:"consoleLogs" bson:"console_logs"`
	NetworkLogs    bool       `json:"networkLogs" bson:"network_logs"`
	Debug          bool       `json:"debug" bson:"debug"`
	Project        string     `json:"project" bson:"project"`
	Platform       string     `json:"platform" bson:"platform"`
	Headless       bool       `json:"headless" bson:"headless"`
	Workers        int        `json:"workers" bson:"workers"`
	EDCDetails     EDCDetails `json:"edcDetails" bson:"edc_details"`
}

func (t *Config) GetWidth() int {
	splitResolution := strings.Split(t.Resolution, "x")
	width, _ := strconv.Atoi(splitResolution[0])
	return width
}

func (t *Config) GetHeight() int {
	splitResolution := strings.Split(t.Resolution, "x")
	height, _ := strconv.Atoi(splitResolution[1])
	return height
}

func (t *Config) Validate() error {
	ve := errors.ValidationErrs()
	if t.Browser == "" {
		ve.Add("browser ", "cannot be empty")
	}
	if t.BrowserVersion == "" {
		ve.Add("browser version", "cannot be empty")
	}

	if t.MachineName == "" {
		t.MachineName = "ADHOC"
	}
	if t.MachineId == "" {
		t.MachineId = "ADHOC"
	}
	if !t.Headless {
		t.Headless = true
	}
	return ve.Err()
}

func (t *Config) WriteConfigToFile(file *os.File) error {
	//TODO
	configs := make([]Config, 0)
	configs = append(configs, *t)
	data, err := json.Marshal(configs)
	if err != nil {
		return err
	}
	_, err = file.Write(data)
	if err != nil {
		return err
	}
	return nil
}

func (t *Config) SetMachine(profile machineprofile.MachineProfile) {
	//TODO check for browser name
	t.Browser = profile.BrowserName
	t.BrowserVersion = profile.BrowserVersion
	t.Platform = profile.OS + " " + profile.OSVersion
	t.MachineId = profile.ID
	t.MachineName = profile.Name
	t.Resolution = profile.Resolution
}

func (t *Config) GetName() string {
	return apxconstants.Local
}

type Configs struct {
	Projects   []Project
	Configs    []Config
	EDCDetails EDCDetails
	Workers    int
}

type EDCDetails struct {
	Site           string   `json:"site"`
	SubjectNumbers []string `json:"subjectNumbers"`
}

type Project struct {
	Name      string `json:"name"`
	TestMatch string `json:"testMatch,omitempty"`
	Use       Use    `json:"use,omitempty"`
}
type Use struct {
	Viewport    Viewport `json:"viewport,omitempty"`
	BrowserName string   `json:"browserName,omitempty"`
	Headless    bool     `json:"headless"`
}

type Viewport struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

func (t *Configs) Validate() error {
	ve := errors.ValidationErrs()
	if len(t.Configs) == 0 {
		ve.Add("configs", "cannot be empty")
	}
	return ve.Err()
}

func (t *Configs) WriteConfigToFile(file *os.File) error {
	data, err := json.Marshal(t.Configs)
	if err != nil {
		return err
	}
	_, err = file.Write(data)
	if err != nil {
		return err
	}
	return nil
}

func (t *Configs) SetMachine(profile machineprofile.MachineProfile) {
	//TODO
}

func (t *Configs) GetName() string {
	return apxconstants.Local
}

func SetLocalTestcaseBrowsers(c *Config) *Config {

	if c.Browser == "Chrome" {
		c.Browser = "chromium"
	} else if c.Browser == "Firefox" {
		c.Browser = "firefox"
	} else if c.Browser == "Edge" {
		c.Browser = "edge"
	} else if c.Browser == "Safari" {
		c.Browser = "webkit"
	}
	return c
}
