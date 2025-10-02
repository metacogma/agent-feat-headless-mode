package runresult

import (
	"agent/errors"
)

type RunResult struct {
	Id                     string          `json:"_id" bson:"_id,omitempty"`
	ExecutionId            string          `json:"execution_id" bson:"execution_id"`
	TestPlanId             string          `json:"testplan_id" bson:"testplan_id"`
	ExecutedBy             string          `json:"created_by" bson:"created_by"`
	CreatedAt              string          `json:"created_at" bson:"created_at"`
	EndedAt                string          `json:"ended_at" bson:"ended_at"`
	Duration               *int64          `json:"duration" bson:"duration"`
	Status                 string          `json:"status" bson:"status"`
	AssignedTo             []string        `json:"assigned_to" bson:"assigned_to"`
	TestPlanName           string          `json:"title" bson:"title"`
	TestLab                []TestLabConfig `json:"test_lab" bson:"test_lab"`
	RunName                string          `json:"run_name" bson:"run_name"`
	ExecutedTestCasesCount int             `json:"executed_test_cases_count" bson:"executed_test_cases_count"`
}

func (r *RunResult) Validate() error {
	ve := errors.ValidationErrs()
	if r.ExecutionId == "" {
		ve.Add("execution_id", "cannot be empty")
	}

	if r.TestPlanId == "" {
		ve.Add("testplan_id", "cannot be empty")
	}

	if r.ExecutedBy == "" {
		ve.Add("created_by", "cannot be empty")
	}

	if r.CreatedAt == "" {
		ve.Add("created_at", "cannot be empty")
	}

	if r.Status == "" {
		ve.Add("status", "cannot be empty")
	}

	if r.TestPlanName == "" {
		ve.Add("title", "cannot be empty")
	}

	if r.RunName == "" {
		ve.Add("run_name", "cannot be empty")
	}

	if r.Duration == nil {
		ve.Add("duration", "cannot be empty")
	}

	return ve.Err()
}

type TestLabConfig struct {
	Name   string                 `json:"name" bson:"name"`
	Config map[string]interface{} `json:"config" bson:"config"`
}
