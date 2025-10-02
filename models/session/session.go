package session

import (
	"agent/models/executionstatus"
	apxconstants "agent/utils/constants"
	"time"
)

type Session struct {
	Testlab            string   `json:"testlab,omitempty" bson:"testlab,omitempty"`
	OrgId              string   `json:"org_id,omitempty" bson:"org_id,omitempty"`
	ProjectId          string   `json:"project_id,omitempty" bson:"project_id,omitempty"`
	AppId              string   `json:"app_id,omitempty" bson:"app_id,omitempty"`
	TestcaseId         string   `json:"testcase_id,omitempty" bson:"testcase_id,omitempty"`
	TestsuiteId        string   `json:"testsuite_id,omitempty" bson:"testsuite_id,omitempty"`
	TestplanId         string   `json:"testplan_id,omitempty" bson:"testplan_id,omitempty"`
	MachineId          string   `json:"machine_id,omitempty" bson:"machine_id,omitempty"`
	IsAdhoc            bool     `json:"is_adhoc,omitempty" bson:"is_adhoc,omitempty"`
	ExecutionId        string   `json:"execution_id,omitempty" bson:"execution_id,omitempty"`
	CreatedBy          string   `json:"created_by,omitempty" bson:"created_by,omitempty"`
	Name               string   `json:"name,omitempty" bson:"name,omitempty"`
	Duration           *int64   `json:"duration,omitempty" bson:"duration,omitempty"`
	OS                 string   `json:"os,omitempty" bson:"os,omitempty"`
	OSVersion          string   `json:"os_version,omitempty" bson:"os_version,omitempty"`
	BrowserVersion     string   `json:"browser_version,omitempty" bson:"browser_version,omitempty"`
	Browser            string   `json:"browser,omitempty" bson:"browser,omitempty"`
	Device             *string  `json:"device,omitempty" bson:"device,omitempty"`
	Status             string   `json:"status,omitempty" bson:"status,omitempty"`
	Reason             *string  `json:"reason,omitempty" bson:"reason,omitempty"`
	CommandRunning     bool     `json:"command_running" bson:"command_running"`
	ProjectName        string   `json:"project_name,omitempty" bson:"project_name,omitempty"`
	TestPriority       *int     `json:"test_priority,omitempty" bson:"test_priority,omitempty"`
	Logs               string   `json:"logs,omitempty" bson:"logs,omitempty"`
	CreatedAt          string   `json:"created_at,omitempty" bson:"created_at,omitempty"`
	VideoURL           string   `json:"video_url,omitempty" bson:"video_url,omitempty"`
	Screenshots        []string `json:"screenshots,omitempty" bson:"screenshots,omitempty"`
	StepCount          string   `json:"step_count,omitempty" bson:"step_count,omitempty"`
	Resolution         string   `json:"resolution,omitempty" bson:"resolution,omitempty"`
	PreRequisiteResult *Session `json:"pre_requisite_result,omitempty" bson:"pre_requisite_result,omitempty"`
	IsPreRequisite     bool     `json:"is_prerequisite,omitempty" bson:"is_pre_requisite,omitempty"`
	ParentTestCaseId   string   `json:"parent_testcase_id,omitempty" bson:"parent_test_case_id,omitempty"`
	MachineName        string   `json:"machineName,omitempty" bson:"machine_name,omitempty"`
	Config             *Config  `json:"config,omitempty" bson:"config,omitempty"`
	RunName            string   `json:"run_name,omitempty" bson:"run_name,omitempty"`
	FileName           string   `json:"file_name,omitempty" bson:"file_name,omitempty"`
	OutputDir          string   `json:"output_dir,omitempty" bson:"output_dir,omitempty"`
}

func NewSession(createdBy, testcaseId, testsuiteId string, status executionstatus.ExecutionStatus, config Config) *Session {
	//TODO 0 index is taken as platform
	return &Session{
		Testlab:        "local",
		OrgId:          status.OrgId,
		ProjectId:      status.ProjectId,
		AppId:          status.AppId,
		CreatedBy:      createdBy,
		ExecutionId:    status.ExecutionId,
		IsAdhoc:        status.IsAdhoc,
		TestsuiteId:    testsuiteId,
		TestplanId:     status.TestplanId,
		CommandRunning: false,
		TestcaseId:     testcaseId,
		MachineId:      config.MachineId,
		OS:             config.OS,
		OSVersion:      config.OSVersion,
		Browser:        config.Browser,
		BrowserVersion: config.BrowserVersion,
		ProjectName:    config.Project,
		Resolution:     config.Resolution,
		Status:         apxconstants.Initializing,
		CreatedAt:      time.Now().UTC().Format(time.RFC3339),
		StepCount:      "0",
		MachineName:    config.MachineName,
		RunName:        status.RunName,
	}
}

type NetworkLogsResponse struct {
	NetworkLogs NetworkLogs `json:"log"`
}

type NetworkLogs struct {
	Logs []LogEntry `json:"entries"`
}

type LogEntry struct {
	Request         Request   `json:"request"`
	Response        Response  `json:"response"`
	StartedDateTime time.Time `json:"startedDateTime"`
	Time            int       `json:"time"`
}

type Request struct {
	Method string `json:"method"`
	URL    string `json:"url"`
}

type Response struct {
	Status     int    `json:"status"`
	StatusText string `json:"statusText"`
}
