package executionstatus

import "agent/errors"

type ExecutionStatus struct {
	OrgId                      string `json:"org_id" bson:"org_id"`
	ProjectId                  string `json:"project_id" bson:"project_id"`
	AppId                      string `json:"app_id" bson:"app_id"`
	Status                     string `json:"status" bson:"status"`
	Message                    string `json:"message" bson:"message"`
	ExecutionId                string `json:"execution_id" bson:"execution_id"`
	TestplanId                 string `json:"testplan_id,omitempty" bson:"testplan_id,omitempty"`
	RunName                    string `json:"run_name,omitempty" bson:"run_name,omitempty"`
	TestLab                    string `json:"testlab,omitempty" bson:"testlab,omitempty"`
	TestsuiteId                string `json:"testsuite_id,omitempty" bson:"testsuite_id,omitempty"`
	TestcaseId                 string `json:"testcase_id,omitempty" bson:"testcase_id,omitempty"`
	MachineId                  string `json:"machine_id,omitempty" bson:"machine_id,omitempty"`
	IsAdhoc                    bool   `json:"is_adhoc" bson:"is_adhoc"`
	CommandRunning             bool   `json:"command_running" bson:"command_running"`
	IsPreRequisite             bool   `json:"is_prerequisite" bson:"is_prerequisite"`
	ParentTestCaseId           string `json:"parent_testcase_id,omitempty" bson:"parent_testcase_id,omitempty"`
	StepCount                  int    `json:"step_count,omitempty" bson:"step_count,omitempty"`
	AbortOnTestFailure         bool   `json:"abort_on_test_failure,omitempty" bson:"abort_on_test_failure,omitempty"`
	AbortOnPreRequisiteFailure bool   `json:"abort_on_pre_requisite_failure,omitempty" bson:"abort_on_pre_requisite_failure,omitempty"`
}

func (e *ExecutionStatus) Validate() error {
	ve := errors.ValidationErrs()

	if e.Status == "" {
		ve.Add("status", "cannot be empty")
	}
	if e.ExecutionId == "" {
		ve.Add("execution_id", "cannot be empty")
	}
	if !e.IsAdhoc {
		if e.TestcaseId == "" {
			ve.Add("testcase_id", "cannot be empty")
		}
		if e.TestsuiteId == "" {
			ve.Add("testsuite_id", "cannot be empty")
		}
		if e.TestplanId == "" {
			ve.Add("testplan_id", "cannot be empty")
		}
	}
	return ve.Err()
}
