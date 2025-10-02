package screenshot

import "agent/errors"

type UploadScreenshotRequest struct {
	OrgId            string `json:"org_id"`
	ProjectId        string `json:"project_id"`
	AppId            string `json:"app_id"`
	ExecutionId      string `json:"execution_id"`
	Testlab          string `json:"testlab"`
	TestcaseId       string `json:"testcase_id,omitempty"`
	TestsuiteId      string `json:"testsuite_id,omitempty"`
	TestplanId       string `json:"testplan_id,omitempty"`
	MachineId        string `json:"machine_id,omitempty"`
	IsAdhoc          bool   `json:"is_adhoc,omitempty"`
	IsPreRequisite   bool   `json:"is_prerequisite,omitempty"`
	ParentTestCaseId string `json:"parent_testcase_id,omitempty"`
}

func (r *UploadScreenshotRequest) Validate() error {
	ve := errors.ValidationErrs()
	if r.ExecutionId == "" {
		ve.Add("execution_id", "cannot be empty")
	}

	if !r.IsAdhoc {
		if r.TestcaseId == "" {
			ve.Add("testcase_id", "cannot be empty")
		}
		if r.TestsuiteId == "" {
			ve.Add("testsuite_id", "cannot be empty")
		}
		if r.TestplanId == "" {
			ve.Add("testplan_id", "cannot be empty")
		}
	}

	return ve.Err()
}

type TakeScreenshot struct {
	Screenshot     string `json:"screenshot"`
	ScreenshotPath string `json:"screenshotPath"`
	ExecutionId    string `json:"execution_id"`
}

func (t *TakeScreenshot) Validate() error {
	ve := errors.ValidationErrs()
	if t.ExecutionId == "" {
		ve.Add("execution_id", "cannot be empty")
	}
	if t.Screenshot == "" {
		ve.Add("screenshot", "cannot be empty")
	}
	if t.ScreenshotPath == "" {
		ve.Add("screenshotPath", "cannot be empty")
	}
	return ve.Err()
}
