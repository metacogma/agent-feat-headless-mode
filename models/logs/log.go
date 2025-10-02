package logs

type Log struct {
	ExecutionId string     `json:"execution_id"`
	TestPlanId  string     `json:"testPlan_id"`
	TestCaseId  string     `json:"testcase_id"`
	MachineId   string     `json:"machine_id"`
	Logs        []Snapshot `json:"snapshot"`
}

type ExecutionLog struct {
	Type     string   `json:"type"`
	Snapshot Snapshot `json:"snapshot"`
}

type Snapshot struct {
	StartedDateTime string   `json:"startedDateTime"`
	Time            float64  `json:"time"`
	Request         Request  `json:"request"`
	Response        Response `json:"response"`
}

type Request struct {
	Method string `json:"method"`
	URL    string `json:"url"`
}

type Response struct {
	Status     int    `json:"status"`
	StatusText string `json:"statusText"`
}
