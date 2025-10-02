package localexecution

import (
	"agent/errors"
	"agent/models/edcdetails"
	"agent/models/session"
	"agent/models/testcase"
	"agent/models/testplan"
)

type LocalExecution struct {
	ID                 string                `json:"_id" bson:"_id"`
	OrgId              string                `json:"org_id" bson:"org_id"`
	ProjectId          string                `json:"project_id" bson:"project_id"`
	AppId              string                `json:"app_id" bson:"app_id"`
	TestCaseScript     *testcase.TestScript  `json:"test_case_script" bson:"test_case_script"`
	ExecutionId        string                `json:"execution_id" bson:"execution_id"`
	EnvironmentId      string                `json:"environment_id" bson:"environment_id"`
	CreatedBy          string                `json:"createdBy" bson:"createdBy"`
	TestLab            string                `json:"testLab" bson:"testLab"`
	LocalSessionConfig *session.Config       `json:"localSessionConfig" bson:"localSessionConfig"`
	PageTimeout        int64                 `json:"page_timeout" bson:"page_timeout"`
	ElementTimeout     int64                 `json:"element_timeout" bson:"element_timeout"`
	TestTimeout        int64                 `json:"test_timeout" bson:"test_timeout"`
	ScreenShotType     string                `json:"screenshot_type" bson:"screenshot_type"`
	RecoverOptions     *RecoverOptions       `json:"recover_options" bson:"recover_options"`
	Type               string                `json:"type" bson:"type"`
	MachineId          string                `json:"machine_id" bson:"machine_id"`
	TestcaseId         string                `json:"testcase_id" bson:"testcase_id"`
	TestplanId         string                `json:"testplan_id" bson:"testplan_id"`
	ExecutedBy         string                `json:"executed_by" bson:"executed_by"`
	LocalDeviceId      string                `json:"local_device_id" bson:"local_device_id"`
	SubjectNumber      string                `json:"subject_number" bson:"subject_number"`
	RunName            string                `json:"run_name" bson:"run_name"`
	ParallelismEnabled bool                  `json:"parallelism_enabled" bson:"parallelism_enabled"`
	Workers            int                   `json:"workers" bson:"workers"`
	EDCDetails         edcdetails.EDCDetails `json:"edc_details" bson:"edc_details"`
}

type RecoverOptions struct {
	AbortOnTestFailure         bool `json:"abort_on_test_failure,omitempty" bson:"abort_on_test_failure,omitempty"`
	AbortOnTestSuiteFailure    bool `json:"abort_on_test_suite_failure,omitempty" bson:"abort_on_test_suite_failure,omitempty"`
	AbortOnPreRequisiteFailure bool `json:"abort_on_pre_requisite_failure,omitempty" bson:"abort_on_pre_requisite_failure,omitempty"`
}

func (l *LocalExecution) Validate() error {
	ve := errors.ValidationErrs()
	if l.MachineId == "" {
		ve.Add("machine_id", "cannot be empty")
	}
	if l.TestcaseId == "" || l.TestplanId == "" {
		ve.Add("testcase_id or testplan_id", "cannot be empty")
	}

	return ve.Err()

}

func (l *LocalExecution) ToTestcaseRequestBody() *testcase.ExecuteRequestBody {
	body := &testcase.ExecuteRequestBody{
		OrgId:              l.OrgId,
		ProjectId:          l.ProjectId,
		AppId:              l.AppId,
		TestCaseId:         l.TestcaseId,
		TestCaseScript:     l.TestCaseScript,
		ExecutionId:        l.ExecutionId,
		EnvironmentId:      l.EnvironmentId,
		CreatedBy:          l.CreatedBy,
		TestLab:            l.TestLab,
		MachineName:        "ADHOC",
		MachineId:          "ADHOC",
		BrowserName:        l.LocalSessionConfig.Browser,
		BrowserVersion:     l.LocalSessionConfig.BrowserVersion,
		PageTimeout:        l.PageTimeout,
		ElementTimeout:     l.ElementTimeout,
		TestTimeout:        l.TestTimeout,
		ScreenShotType:     l.ScreenShotType,
		RecoverOptions:     (*testplan.RecoverOptions)(l.RecoverOptions),
		LocalSessionConfig: l.LocalSessionConfig,
	}
	body.LocalSessionConfig.MachineId = "ADHOC"
	body.LocalSessionConfig.MachineName = "ADHOC"
	return body
}

func (l *LocalExecution) ToTestPlanRequestBody() *testplan.TestPlanExecutionRequestBody {
	return &testplan.TestPlanExecutionRequestBody{
		OrgId:              l.OrgId,
		ProjectId:          l.ProjectId,
		AppId:              l.AppId,
		TestPlanId:         l.TestplanId,
		EnvironmentId:      l.EnvironmentId,
		ExecutedBy:         l.ExecutedBy,
		PageTimeout:        l.PageTimeout,
		ElementTimeout:     l.ElementTimeout,
		TestTimeout:        l.TestTimeout,
		ExecutionId:        l.ExecutionId,
		SubjectNumber:      l.SubjectNumber,
		MachineId:          l.MachineId,
		RunName:            l.RunName,
		ParallelismEnabled: l.ParallelismEnabled,
		Workers:            l.Workers,
		EDCDetails:         l.EDCDetails,
	}
}
