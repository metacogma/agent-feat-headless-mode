package testcase

import (
	"go.mongodb.org/mongo-driver/bson/primitive"

	"agent/errors"
	"agent/models/replacements"
	"agent/models/session"
	"agent/models/testlab"
	"agent/models/testplan"
	apxconstants "agent/utils/constants"
)

type TestCase struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"_id,omitempty"`
	Title       string             `bson:"title,omitempty" json:"title,omitempty"`
	Description string             `bson:"description,omitempty" json:"description,omitempty"`
	Priority    string             `bson:"priority,omitempty" json:"priority,omitempty"`
	CreatedBy   string             `bson:"created_by,omitempty" json:"created_by,omitempty"`
}

type TestScript struct {
	ID           string                              `bson:"_id,omitempty" json:"_id"`
	Title        string                              `bson:"title" json:"title"`
	Script       string                              `bson:"script" json:"script"`
	Source       string                              `bson:"source" json:"source"`
	Replacements map[string]replacements.Replacement `bson:"replacements" json:"replacements"`
	Precondition *TestScript                         `bson:"precondition" json:"precondition"`
}

type ExecuteRequestBody struct {
	OrgId              string                   `json:"orgId"`
	ProjectId          string                   `json:"projectId"`
	AppId              string                   `json:"appId"`
	TestCaseId         string                   `json:"testCaseId"`
	TestCaseScript     *TestScript              `json:"test_case_script"`
	ExecutionId        string                   `json:"execution_id"`
	EnvironmentId      string                   `json:"environment_id"`
	CreatedBy          string                   `json:"createdBy"`
	TestLab            string                   `json:"testLab"`
	MachineName        string                   `json:"machineName" bson:"machine_name"`
	MachineId          string                   `json:"machineId" bson:"machine_id"`
	BrowserName        string                   `json:"browserName" bson:"browser_name"`
	BrowserVersion     string                   `json:"browserVersion" bson:"browser_version"`
	PageTimeout        int64                    `json:"page_timeout"`
	ElementTimeout     int64                    `json:"element_timeout"`
	TestTimeout        int64                    `json:"test_timeout"`
	ScreenShotType     string                   `json:"screenshot_type" bson:"screenshot_type"`
	RecoverOptions     *testplan.RecoverOptions `json:"recover_options" bson:"recover_options"`
	LocalSessionConfig *session.Config          `json:"localSessionConfig" bson:"localSessionConfig"`
}

func (e *ExecuteRequestBody) ToTestLabConfig() testlab.TestLabConfig {
	config := e.LocalSessionConfig

	return config
}

func (t *ExecuteRequestBody) Validate() error {
	ve := errors.ValidationErrs()

	if t.OrgId == "" {
		ve.Add("orgId", "cannot be empty")
	}

	if t.ProjectId == "" {
		ve.Add("projectId", "cannot be empty")
	}

	if t.AppId == "" {
		ve.Add("appId", "cannot be empty")
	}

	if t.TestCaseId == "" {
		ve.Add("testCaseId", "cannot be empty")
	}

	if t.RecoverOptions == nil {
		t.RecoverOptions = &testplan.RecoverOptions{
			AbortOnTestFailure:         false,
			AbortOnTestSuiteFailure:    false,
			AbortOnPreRequisiteFailure: false,
		}
	}

	if t.TestLab == "" {
		t.TestLab = apxconstants.Local
	}
	if t.TestTimeout == 0 {
		t.TestTimeout = 240 * 1000
	} else {
		t.TestTimeout = t.TestTimeout * 1000
	}
	if t.PageTimeout == 0 {
		t.PageTimeout = 30 * 1000
	} else {
		t.PageTimeout = t.PageTimeout * 1000
	}
	if t.ElementTimeout == 0 {
		t.ElementTimeout = 30 * 1000
	} else {
		t.ElementTimeout = t.ElementTimeout * 1000
	}

	return ve.Err()
}
