package executionstep

import "agent/errors"

type ExecutionStep struct {
	OrgId            string `json:"org_id"`
	ProjectId        string `json:"project_id"`
	AppId            string `json:"app_id"`
	ExecutionId      string `json:"execution_id"`
	TestcaseId       string `json:"testcase_id,omitempty"`
	MachineId        string `json:"machine_id,omitempty"`
	TestsuiteId      string `json:"testsuite_id,omitempty"`
	TestplanId       string `json:"testplan_id,omitempty"`
	IsAdhoc          bool   `json:"is_adhoc"`
	StepCount        string `json:"step_count"`
	IsPreRequisite   bool   `json:"is_prerequisite"`
	ParentTestCaseId string `json:"parent_testcase_id,omitempty"`
}

func (t *ExecutionStep) Validate() error {
	ve := errors.ValidationErrs()
	if t.ExecutionId == "" {
		ve.Add("execution_id", "cannot be empty")
	}
	if !t.IsAdhoc {
		if t.TestcaseId == "" {
			ve.Add("testcase_id", "cannot be empty")
		}
		if t.TestsuiteId == "" {
			ve.Add("testsuite_id", "cannot be empty")
		}
		if t.TestplanId == "" {
			ve.Add("testplan_id", "cannot be empty")
		}
	}
	return ve.Err()
}
