package apxconstants

const (

	// Test plan types
	CustomTestPlan       = "custom"
	CrossBrowserTestPlan = "cross_browser"

	// Execution status
	InQueue             = "in_queue"
	Initializing        = "initializing"
	Running             = "running"
	Stopped             = "stopped"
	Failed              = "failed"
	Passed              = "passed"
	Timeout             = "timeout"
	NotExecuted         = "Not Executed"
	Aborted             = "Aborted"
	TestScriptVariables = `
		testcase_id: "%s",
		execution_id: "%s",
		testlab: process.env.TESTLAB,
		created_by: "%s",
		testsuite_id: "%s",
		testplan_id: "%s",
		is_adhoc: %t,
		is_prerequisite: %t,
		parent_testcase_id: "%s",
		screenshot_type: "%s",
		abort_on_test_failure: %t,
		abort_On_pre_requisite_failure: %t,
		step_count: 0,
		status: "running",
		org_id: "%s",
		project_id: "%s",
		app_id: "%s",
		testcase_name: "%s",
		source: "%s",
	`

	EdcVariables = `
	VAULT_DNS: "%s",
	VAULT_VERSION: "%s",
	VAULT_STUDY_NAME: "%s",
	VAULT_STUDY_COUNTRY: "%s",
	VAULT_SITE_NAME: "%s",
	VAULT_SUBJECT_NAME: "%s",
	VAULT_USER_NAME: "%s",
	VAULT_PASSWORD: "%s",
	EDC_VEEVA_LOGIN_URL: "%s",
	`

	// Testlab
	Local = "local"
)
