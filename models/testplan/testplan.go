package testplan

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/segmentio/kafka-go"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"agent/errors"
	"agent/models/edcdetails"
	"agent/models/replacements"
	"agent/models/session"
	"agent/models/testlab"
	apxconstants "agent/utils/constants"
)

type TestPlan struct {
	ID             primitive.ObjectID `json:"_id,omitempty" bson:"_id,omitempty"`
	Title          string             `json:"title,omitempty" bson:"title,omitempty"`
	Description    string             `json:"description,omitempty" bson:"description,omitempty"`
	Labels         []string           `json:"labels,omitempty" bson:"labels,omitempty"`
	CreatedBy      string             `json:"created_by,omitempty" bson:"created_by,omitempty"`
	CreatedOn      *time.Time         `json:"created_on,omitempty" bson:"created_on,omitempty"`
	UpdatedOn      *time.Time         `json:"updated_on,omitempty" bson:"updated_on,omitempty"`
	ScreenShotType string             `json:"screenshot_type,omitempty" bson:"screenshot_type,omitempty"`
	PageTimeout    int                `json:"page_timeout,omitempty" bson:"page_timeout,omitempty"`
	ElementTimeout int                `json:"element_timeout,omitempty" bson:"element_timeout,omitempty"`
	RecoverOptions *RecoverOptions    `json:"recover_options,omitempty" bson:"recover_options,omitempty"`
	TestSuites     []TestSuiteConfig  `json:"test_suites,omitempty" bson:"test_suites,omitempty"`
	Type           string             `json:"type,omitempty" bson:"type,omitempty"`
	NoOfTestsuites int                `json:"no_of_testsuites" bson:"no_of_testsuites,omitempty"`
	Status         string             `json:"status,omitempty" bson:"status,omitempty"`
	Assignees      []string           `bson:"assignees,omitempty" json:"assignees,omitempty"`
	Reviewers      []string           `bson:"reviewers,omitempty" json:"reviewers,omitempty"`
	Priority       string             `bson:"priority,omitempty" json:"priority,omitempty"`
	LastRunStatus  string             `json:"last_run_status,omitempty" bson:"last_run_status,omitempty"`
}

type TestSuiteConfig struct {
	TestSuiteId string          `json:"test_suite_id" bson:"test_suite_id"`
	TestLabs    []TestLabConfig `json:"test_labs" bson:"test_labs"`
}

type TestLabConfig struct {
	Name   string                 `json:"name" bson:"name"`
	Config map[string]interface{} `json:"config" bson:"config"`
}

func (t *TestSuiteConfig) ToKafkaMessage() (*kafka.Message, error) {
	data, err := json.Marshal(t)
	if err != nil {
		return nil, err
	}
	return &kafka.Message{
		Key:        []byte(t.TestSuiteId),
		Value:      data,
		WriterData: t.TestSuiteId,
	}, nil
}

type RecoverOptions struct {
	AbortOnTestFailure         bool `json:"abort_on_test_failure,omitempty" bson:"abort_on_test_failure,omitempty"`
	AbortOnTestSuiteFailure    bool `json:"abort_on_test_suite_failure,omitempty" bson:"abort_on_test_suite_failure,omitempty"`
	AbortOnPreRequisiteFailure bool `json:"abort_on_pre_requisite_failure,omitempty" bson:"abort_on_pre_requisite_failure,omitempty"`
}

type TestPlanScript struct {
	ID           primitive.ObjectID                  `json:"_id,omitempty" bson:"_id,omitempty"`
	TestSuiteId  string                              `json:"test_suite_id" bson:"test_suite_id"`
	TestCaseId   string                              `json:"testcase_id" bson:"testcase_id"`
	Source       string                              `json:"source" bson:"source"`
	Script       string                              `json:"script" bson:"script"`
	Replacements map[string]replacements.Replacement `json:"replacements" bson:"replacements"`
	Precondition *TestPlanScript                     `json:"precondition" bson:"precondition"`
}

type TestPlanExecutionDetails struct {
	OrgId              string                `json:"org_id"`
	ProjectId          string                `json:"project_id"`
	AppId              string                `json:"app_id"`
	TestPlanId         string                `json:"test_plan_id"`
	TestPlanName       string                `json:"test_plan_name"`
	ExecutionId        string                `json:"execution_id"`
	ExecutedBy         string                `json:"executed_by"`
	Type               string                `json:"type"`
	PageTimeout        int64                 `json:"page_timeout"`
	ElementTimeout     int64                 `json:"element_timeout"`
	TestTimeout        int64                 `json:"test_timeout"`
	TestLabs           []TestLabConfig       `json:"test_labs"`
	Configs            []TestSuiteConfig     `json:"configs"`
	Scripts            []TestPlanScript      `json:"scripts"`
	ScreenShotType     string                `json:"screenshot_type" bson:"screenshot_type"`
	RecoverOptions     RecoverOptions        `json:"recover_options" bson:"recover_options"`
	RunName            string                `json:"run_name"`
	SubjectNumber      string                `json:"subject_number" bson:"subject_number"`
	MachineId          string                `json:"machine_id" bson:"machine_id"`
	ParallelismEnabled bool                  `json:"parallelism_enabled" bson:"parallelism_enabled"`
	Workers            int                   `json:"workers" bson:"workers"`
	EDCDetails         edcdetails.EDCDetails `json:"edc_details" bson:"edc_details"`
}

func (t *TestPlanExecutionDetails) Validate() error {
	ve := errors.ValidationErrs()
	if t.OrgId == "" {
		ve.Add("OrgId", "Required")

	}
	if t.ProjectId == "" {
		ve.Add("ProjectId", "Required")
	}
	if t.AppId == "" {
		ve.Add("AppId", "Required")
	}
	if t.TestPlanId == "" {
		ve.Add("TestPlanId", "Required")
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

type TestPlanExecutionRequestBody struct {
	OrgId         string `json:"orgId"`
	ProjectId     string `json:"projectId"`
	AppId         string `json:"appId"`
	TestPlanId    string `json:"testplanId"`
	EnvironmentId string `json:"environmentId"`
	ExecutedBy    string `json:"executedBy"`
	//TODO: Get from testplan config
	PageTimeout        int64                 `json:"page_timeout"`
	ElementTimeout     int64                 `json:"element_timeout"`
	TestTimeout        int64                 `json:"test_timeout"`
	ExecutionId        string                `json:"execution_id"`
	Testlab            string                `json:"testlab"`
	SubjectNumber      string                `json:"subject_number"`
	MachineId          string                `json:"machine_id"`
	RunName            string                `json:"run_name"`
	ParallelismEnabled bool                  `json:"parallelism_enabled"`
	Workers            int                   `json:"workers" bson:"workers"`
	EDCDetails         edcdetails.EDCDetails `json:"edc_details" bson:"edc_details"`
}

func (t *TestPlanExecutionRequestBody) Validate() error {
	ve := errors.ValidationErrs()

	if t.ExecutionId == "" {
		ve.Add("ExecutionId", "Required")
	}
	if t.OrgId == "" {
		ve.Add("OrgId", "Required")
	}
	if t.ProjectId == "" {
		ve.Add("ProjectId", "Required")
	}
	if t.AppId == "" {
		ve.Add("AppId", "Required")
	}

	if t.TestPlanId == "" {
		ve.Add("TestPlanId", "Required")
	}

	if t.Testlab == "" {
		t.Testlab = apxconstants.Local
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

func NewTestLabConfigFromTestSuiteConfig(configs []TestSuiteConfig) map[string]testlab.TestLabConfig {
	testlabsConfig := make(map[string]testlab.TestLabConfig)
	for _, testSuite := range configs {
		testSuiteId := testSuite.TestSuiteId
		for _, testLab := range testSuite.TestLabs {
			testLabConfig, ok := testlabsConfig[testLab.Name]
			if !ok {
				testLabConfig = GetDefaultConfig(testLab.Name)

				testlabsConfig[testLab.Name] = testLabConfig

			}
			AppendTestSuiteConfig(testLab.Name, &testLabConfig, testLab.Config, apxconstants.CustomTestPlan, testSuiteId)
		}
	}
	return testlabsConfig
}

func NewTestLabConfigFromTestPlanConfig(testplanId, machineId string, configs []TestLabConfig) *session.Configs {
	for _, testLab := range configs {
		if testLab.Name == apxconstants.Local && testLab.Config["machineId"] == machineId {
			localConfigs := &session.Configs{}
			data, err := json.Marshal(testLab.Config)
			if err != nil {
				continue
			}
			var config *session.Config
			err = json.Unmarshal(data, &config)
			if err != nil {
				continue
			}

			config = session.SetLocalTestcaseBrowsers(config)

			project := session.Project{
				Name:      config.MachineId,
				TestMatch: fmt.Sprintf("tests/testPlan_%s/test.list.js", testplanId),
				Use:       session.Use{BrowserName: config.Browser, Viewport: session.Viewport{Width: config.GetWidth(), Height: config.GetHeight()}, Headless: true},
			}
			localConfigs.Projects = append(localConfigs.Projects, project)
			localConfigs.Configs = append(localConfigs.Configs, *config)
			localConfigs.EDCDetails = config.EDCDetails
			localConfigs.Workers = config.Workers
			return localConfigs
		}
	}
	return nil
}

func GetDefaultConfig(testlab string) testlab.TestLabConfig {
	return &session.Configs{}
}

func AppendTestSuiteConfig(testlab string, config *testlab.TestLabConfig, testsuiteConfig map[string]interface{}, planType, testSuiteId string) {
	_, err := json.Marshal(testsuiteConfig)
	if err != nil {
		return
	}
	if config == nil {
		return
	}

	switch testlab {
	// case apxconstants.BrowserStack:
	// 	browserStackConfig := (*config).(*browserstack.BrowserStackConfig)
	// 	var platformConfig browserstack.Platform
	// 	err = json.Unmarshal(bytes, &platformConfig)
	// 	if err != nil {
	// 		fmt.Println("Failed to unmarshal test suite config", err)
	// 		return
	// 	}
	// 	if planType == apxconstants.CustomTestPlan {
	// 		platformConfig.PlaywrightConfigOptions.TestMatch = fmt.Sprintf("tests/testSuite_%s/*.spec.js", testSuiteId)
	// 	}
	// 	browserStackConfig.Platforms = append(browserStackConfig.Platforms, platformConfig)
	// 	return
	// case apxconstants.LambdaTest:
	// 	lambdaTestConfig := (*config).(*lambdatest.Config)
	// 	var platformConfig browserstack.Platform
	// 	err = json.Unmarshal(bytes, &platformConfig)
	// 	if err != nil {
	// 		fmt.Println("Failed to unmarshal test suite config", err)
	// 		return
	// 	}
	// 	if planType == apxconstants.CustomTestPlan {
	// 		platformConfig.PlaywrightConfigOptions.TestMatch = fmt.Sprintf("tests/testSuite_%s/*.spec.js", testSuiteId)
	// 	}
	// 	browserStackConfig.Platforms = append(browserStackConfig.Platforms, platformConfig)
	// 	return
	}
}
