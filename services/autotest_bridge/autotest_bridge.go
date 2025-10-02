package autotestbridge

import (
	bytes1 "bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"agent/logger"
	"agent/models/environments"
	"agent/models/integrations"
	"agent/models/localdevice"
	localexecution_model "agent/models/localexecution"
	"agent/models/testcase"
	"agent/models/testplan"
)

type AutotestBridgeService struct {
	testCaseServiceEndPoint string
}

func NewAutoTestBridgeService(testCaseServiceEndPoint string) *AutotestBridgeService {
	return &AutotestBridgeService{
		testCaseServiceEndPoint: testCaseServiceEndPoint,
	}
}

func (s *AutotestBridgeService) GetTestCase(orgId, projectId string, appId string, testCaseId string, environmentId string) (*testcase.TestScript, error) {
	requestUrl := s.testCaseServiceEndPoint + fmt.Sprintf("/local-agent/organisations/%s/projects/%s/apps/%s/testcases/%s/testscript?environmentId=%s", orgId, projectId, appId, testCaseId, environmentId)
	var testcase testcase.TestScript
	res, err := http.Get(requestUrl)
	if err != nil {
		logger.Error("error getting testcase", err)
		return nil, err
	}
	err = json.NewDecoder(res.Body).Decode(&testcase)
	if err != nil {
		logger.Error("error decoding testcase", err)
		return nil, err
	}
	return &testcase, nil
}

func (s *AutotestBridgeService) GetTestPlanExecutionDetails(orgId, projectId, appId, testplanId, environmentId string) (*testplan.TestPlanExecutionDetails, error) {
	requestUrl := s.testCaseServiceEndPoint + fmt.Sprintf("/local-agent/organisations/%s/projects/%s/apps/%s/testplans/%s/execution-details?environmentId=%s", orgId, projectId, appId, testplanId, environmentId)
	var details testplan.TestPlanExecutionDetails
	res, err := http.Get(requestUrl)
	if err != nil {
		logger.Error("error getting testplan", err)
		return nil, err
	}
	err = json.NewDecoder(res.Body).Decode(&details)
	if err != nil {
		logger.Error("error decoding testplan", err)
		return nil, err
	}
	return &details, nil
}

func (s *AutotestBridgeService) GetEnvironmentDetails(orgId, projectId, appId, environmentId string) (*environments.Environment, error) {
	requestUrl := s.testCaseServiceEndPoint + fmt.Sprintf("/local-agent/organisations/%s/projects/%s/apps/%s/environments/%s", orgId, projectId, appId, environmentId)
	var environment environments.Environment
	res, err := http.Get(requestUrl)
	if err != nil {
		logger.Error("error getting environment details", err)
		return nil, err
	}
	err = json.NewDecoder(res.Body).Decode(&environment)
	if err != nil {
		logger.Error("error decoding environment details", err)
		return nil, err
	}
	return &environment, nil
}

func (s *AutotestBridgeService) GetLocalexecution(deviceId string) (*localexecution_model.LocalExecution, error) {
	baseUrl := s.testCaseServiceEndPoint + "/local-agent/local-executions"
	parsedUrl, err := url.Parse(baseUrl)
	if err != nil {
		logger.Error("error parsing url", err)
		return nil, err
	}
	query := parsedUrl.Query()
	query.Add("deviceId", deviceId)
	parsedUrl.RawQuery = query.Encode()
	requestUrl := parsedUrl.String()

	var localexecutions *localexecution_model.LocalExecution

	request, err := http.NewRequest(http.MethodGet, requestUrl, nil)
	if err != nil {
		logger.Error("error creating request", err)
		return nil, err
	}

	res, err := http.DefaultClient.Do(request)
	if err != nil {
		logger.Error("error getting localexecutions", err)
		return nil, err
	}

	respbytes, err := io.ReadAll(res.Body)
	if err != nil {
		logger.Error("error getting localexecutions", err)
		return nil, err
	}
	err = json.Unmarshal(respbytes, &localexecutions)
	if err != nil {
		logger.Error("error decoding localexecutions", err)
		return nil, err
	}
	return localexecutions, nil
}

func (s *AutotestBridgeService) DeleteLocalexecution(deviceId string, localexecutionId string) error {
	requestUrl := s.testCaseServiceEndPoint + fmt.Sprintf("/local-agent/local-executions?machineId=%s&executionId=%s", deviceId, localexecutionId)
	request, err := http.NewRequest(http.MethodDelete, requestUrl, nil)
	if err != nil {
		logger.Error("error creating request", err)
		return err
	}

	_, err = http.DefaultClient.Do(request)
	if err != nil {
		logger.Error("error deleting localexecution", err)
		return err
	}
	return nil
}

func (s *AutotestBridgeService) FindTestplanById(orgId, projectId string, appId string, testplanId string) (*testplan.TestPlan, error) {
	requestUrl := s.testCaseServiceEndPoint + fmt.Sprintf("/local-agent/organisations/%s/projects/%s/apps/%s/testplans/%s", orgId, projectId, appId, testplanId)
	var testplan testplan.TestPlan
	res, err := http.Get(requestUrl)
	if err != nil {
		logger.Error("error getting testplan", err)
		return nil, err
	}
	err = json.NewDecoder(res.Body).Decode(&testplan)
	if err != nil {
		logger.Error("error decoding testplan", err)
		return nil, err
	}
	return &testplan, nil
}

func (s *AutotestBridgeService) InsertLocalDevice(localDevice *localdevice.Config, machineId string) (*localdevice.Config, error) {

	baseUrl := s.testCaseServiceEndPoint + "/local-devices"
	parsedUrl, err := url.Parse(baseUrl)
	if err != nil {
		logger.Error("error parsing url", err)
		return nil, err
	}
	query := parsedUrl.Query()
	query.Add("machineId", machineId)
	parsedUrl.RawQuery = query.Encode()
	requestUrl := parsedUrl.String()

	bytes, err := json.Marshal(localDevice)
	if err != nil {
		logger.Error("error marshalling local device", err)
		return nil, err
	}

	request, err := http.NewRequest(http.MethodPost, requestUrl, bytes1.NewReader(bytes))
	if err != nil {
		logger.Error("error creating request", err)
		return nil, err
	}

	_, err = http.DefaultClient.Do(request)
	if err != nil {
		logger.Error("error creating local device", err)
		return nil, err
	}

	return localDevice, nil
}

func (s *AutotestBridgeService) GetEdcIntegrationDetails(orgId, projectId, appId string) (*integrations.VeevaVault, error) {
	integrationType := "edc"
	integrationName := "veeva_vault"
	requestUrl := s.testCaseServiceEndPoint + fmt.Sprintf("/local-agent/organisations/%s/projects/%s/apps/%s/integrations/%s/%s", orgId, projectId, appId, integrationType, integrationName)
	fmt.Println("requestUrl", requestUrl)
	var response integrations.IntegrationModel
	res, err := http.Get(requestUrl)
	if err != nil {
		logger.Error("error getting edc integration details", err)
		return nil, err
	}
	err = json.NewDecoder(res.Body).Decode(&response)
	if err != nil {
		logger.Error("error decoding edc integration details", err)
		return nil, err
	}
	fmt.Println("response", response)
	return &response.Integrations.Edc.VeevaVault, nil
}
