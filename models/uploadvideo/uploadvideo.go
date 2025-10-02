package uploadvideo

import (
	"mime/multipart"

	"agent/errors"
)

type UploadVideo struct {
	Video            multipart.File `json:"video" validate:"required"`
	OrgId            string         `json:"org_id" validate:"required"`
	ProjectId        string         `json:"project_id" validate:"required"`
	AppId            string         `json:"app_id" validate:"required"`
	ExecutionId      string         `json:"execution_id" validate:"required"`
	Testlab          string         `json:"testlab" validate:"required"`
	TestcaseId       string         `json:"testcase_id,omitempty"`
	TestsuiteId      string         `json:"testsuite_id,omitempty"`
	TestplanId       string         `json:"testplan_id,omitempty"`
	MachineId        string         `json:"machine_id,omitempty"`
	IsAdhoc          bool           `json:"is_adhoc,omitempty"`
	IsPreRequisite   bool           `json:"is_prerequisite,omitempty"`
	ParentTestCaseId string         `json:"parent_testcase_id,omitempty"`
	OutputDir        string         `json:"output_dir,omitempty"`
}

func (r *UploadVideo) Validate() error {
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
