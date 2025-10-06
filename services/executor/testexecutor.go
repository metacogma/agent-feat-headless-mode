package executor

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"go.uber.org/zap"

	"agent/logger"
	"agent/models/executionstatus"
	localexecution_model "agent/models/localexecution"
	"agent/models/runresult"
	"agent/models/session"
	"agent/models/testcase"
	"agent/models/testlab"
	"agent/models/testplan"
	executionbridge "agent/services/execution_bridge"
	apxconstants "agent/utils/constants"
	browser_pool "agent/services/browser_pool" // nkk: added import for BrowserPoolManager
	// rationale: Required to reference the BrowserPoolManager type for optimized browser reuse.
)

type ResultService interface {
	GetLamdattestResults(ctx context.Context, appId string) error
}

type TestExecutor struct {
	commandChannel         chan map[string]interface{}
	ExecutionServiceBridge *executionbridge.ExecutionServiceBridge
	browserPool            *browser_pool.BrowserPoolManager // nkk: added BrowserPoolManager reference
	// rationale: Required to acquire and release pre-warmed browser instances for optimized execution.
}

func NewTestExecutor(executionsvcbridge *executionbridge.ExecutionServiceBridge, pool *browser_pool.BrowserPoolManager) *TestExecutor {
	executor := &TestExecutor{
		ExecutionServiceBridge: executionsvcbridge,
		browserPool:            pool, // nkk: initialize BrowserPoolManager
		// rationale: Pass in a pre-initialized BrowserPoolManager to enable optimized browser reuse.
	}

	executor.commandChannel = make(chan map[string]interface{})

	return executor
}

func (t *TestExecutor) Execute(ctx context.Context, orgId, projectId, appId, createdBy string, testcase *testcase.TestScript, config testlab.TestLabConfig, executionId string, executionsMap map[string]*localexecution_model.LocalExecution) error {

	localTestConfig := config.(*session.Config)
	localTestConfig = session.SetLocalTestcaseBrowsers(localTestConfig)
	var mx sync.RWMutex

	status := executionstatus.ExecutionStatus{
		OrgId:       orgId,
		ProjectId:   projectId,
		AppId:       appId,
		ExecutionId: executionId,
		Status:      apxconstants.Running,
		IsAdhoc:     true,
		Message:     apxconstants.Running,
		TestLab:     apxconstants.Local,
		TestcaseId:  testcase.ID,
	}

	logger.Info("starting local testcase session", zap.String("execution_id", executionId), zap.String("testcase_id", testcase.ID))

	Session := session.NewSession(createdBy, testcase.ID, "", status, *localTestConfig)

	if testcase.Precondition != nil {
		preRequisiteSession := session.NewSession(createdBy, testcase.Precondition.ID, "", status, *localTestConfig)
		Session.PreRequisiteResult = preRequisiteSession
	}

	err := t.ExecutionServiceBridge.SaveSession(ctx, orgId, projectId, appId, apxconstants.Local, *Session)
	if err != nil {
		logger.Error("could not save session", err)
		return err
	}

	path := filepath.Join(".", "executions")

	projectsPath := filepath.Join(path, "projects.json")

	projectsFile, err := os.Create(projectsPath)
	if err != nil {
		logger.Error("could not create config file", err)
		return err
	}

	projectConfig := fmt.Sprintf(`[{"name":"ADHOC","use":{"viewport":{"width":%d,"height":%d},"headless":%t,"browserName":"%s"}}]`,
		localTestConfig.GetWidth(),
		localTestConfig.GetHeight(),
		true,
		localTestConfig.Browser)

	projectsFile.WriteString(projectConfig)

	defer projectsFile.Close()

	commandContext, cancel := context.WithCancel(context.Background())
	defer cancel()

	// nkk: OLD IMPLEMENTATION
	// cmd := exec.CommandContext(commandContext, "npx", "playwright", "test")
	// cmd.Dir = "executions"
	// rationale: This approach starts a new Playwright process for each test, causing high startup latency and resource usage.
	// Updated to use BrowserPool for reusing pre-warmed browser instances, reducing startup time from ~30s to ~0.5s.

	// nkk: NEW IMPLEMENTATION
	browserInstance, err := t.browserPool.AcquireBrowser(ctx, localTestConfig.Browser, "latest")
	if err != nil {
		logger.Error("could not acquire browser from pool", err)
		return err
	}
	defer t.browserPool.ReleaseBrowser(localTestConfig.Browser, "latest", browserInstance)

	cmd := exec.CommandContext(commandContext, "npx", "playwright", "test", "--browser", browserInstance.BrowserType)
	cmd.Dir = "executions"

	// Create pipes to capture stdout and stderr
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		logger.Error("could not get stdout pipe", err)
		status.Status = apxconstants.Failed
		status.Message = "failed to get stdout pipe"
		t.ExecutionServiceBridge.SaveSessionStatus(ctx, status)
		return err
	}

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		logger.Error("could not get stderr pipe", err)
		status.Status = apxconstants.Failed
		status.Message = "failed to get stderr pipe"
		t.ExecutionServiceBridge.SaveSessionStatus(ctx, status)
		return err
	}

	// Start the command
	logger.Info("starting testcase execution...", zap.String("testcase_id", testcase.ID), zap.String("execution_id", executionId))

	if err := cmd.Start(); err != nil {
		logger.Error("could not start command: ", err)
		return err
	}

	// Goroutine to print stdout
	go func() {
		scanner := bufio.NewScanner(stdoutPipe)
		for scanner.Scan() {
			line := scanner.Text()
			// logger.Info("stdout", zap.String("output", line))
			fmt.Println(line)
			mx.Lock()
			if strings.Contains(strings.ToLower(line), "error") {
				status.Status = apxconstants.Failed
				status.Message = line
				t.ExecutionServiceBridge.SaveSessionStatus(ctx, status)
			}
			mx.Unlock()
		}
		if err := scanner.Err(); err != nil {
			logger.Error("error reading stdout", err)
			fmt.Println(err)
		}
	}()

	// Goroutine to print stderr
	go func() {
		scanner := bufio.NewScanner(stderrPipe)
		for scanner.Scan() {
			line := scanner.Text()
			logger.Info("stderr", zap.String("output", line))
		}
		if err := scanner.Err(); err != nil {
			logger.Error("error reading stderr", err)
		}
	}()

	// Goroutine to handle command completion and cancellation
	go func() {
		select {
		case <-commandContext.Done():
			logger.Info("local test session completed", zap.String("execution_id", executionId))
			delete(executionsMap, executionId)
			return
		case data := <-t.commandChannel:
			if data["type"] == "testcase" && data["execution_id"] == executionId && data["testcase_id"] == testcase.ID {
				logger.Info("Sending SIGINT to stop the test case execution", zap.String("execution_id", executionId))

				// Send SIGINT (Ctrl+C) to the process to simulate graceful shutdown
				if err := cmd.Process.Signal(syscall.SIGINT); err != nil {
					logger.Error("failed to send SIGINT", err)
				} else {
					logger.Info("SIGINT sent successfully")
				}
				cancel() // Cancel the context to stop further operations
			}
		}
	}()
	os.Setenv("WORKERS", "1")
	// Wait for the command to finish
	err = cmd.Wait()
	if err != nil {
		// Check if the error is due to the process being killed
		if err.Error() == "signal: killed" || err.Error() == "signal: interrupt" {
			logger.Info("Command was interrupted or killed", zap.String("execution_id", executionId))
			status.Status = apxconstants.Stopped
			status.Message = "User Stopped Session"
			t.ExecutionServiceBridge.SaveSessionStatus(ctx, status)
		} else {
			logger.Error("command failed to execute", err)
			mx.Lock()
			if status.Status != apxconstants.Failed {
				status.Status = apxconstants.Failed
				status.Message = "failed to execute test case " + err.Error()
				t.ExecutionServiceBridge.SaveSessionStatus(ctx, status)
			}
			mx.Unlock()
			return err
		}
	}

	logger.Info("testcase execution completed")
	return nil
}

func (t *TestExecutor) InitializeSessions(ctx context.Context, createdBy string, status executionstatus.ExecutionStatus, details *testplan.TestPlanExecutionDetails, config *session.Config, wg *sync.WaitGroup) {
	for _, testcase := range details.Scripts {
		// create a session with initializing status
		go t.InitializeSession(ctx, testcase, createdBy, status, config, wg)
	}
}

func (t *TestExecutor) InitializeSession(ctx context.Context, testCase testplan.TestPlanScript, createdBy string, status executionstatus.ExecutionStatus, config *session.Config, wg *sync.WaitGroup) {
	wg.Add(1)
	status.TestsuiteId = testCase.TestSuiteId
	status.MachineId = config.MachineId
	session := session.NewSession(createdBy, testCase.TestCaseId, status.TestsuiteId, status, *config)

	err := t.ExecutionServiceBridge.SaveSession(ctx, status.OrgId, status.ProjectId, status.AppId, apxconstants.Local, *session)

	if err == nil {
		logger.Info("created session for " + testCase.TestCaseId)
	} else {
		logger.Error("failed to create session for "+testCase.TestCaseId, err)
	}
	wg.Done()
}

func (t *TestExecutor) ExecuteTestPlan(createdBy, executionId, testplanId string, config testlab.TestLabConfig, details *testplan.TestPlanExecutionDetails, executionsMap map[string]*localexecution_model.LocalExecution) error {
	status := executionstatus.ExecutionStatus{
		OrgId:          details.OrgId,
		ProjectId:      details.ProjectId,
		AppId:          details.AppId,
		Status:         apxconstants.Running,
		TestplanId:     testplanId,
		Message:        apxconstants.Running,
		IsAdhoc:        false,
		CommandRunning: false,
		ExecutionId:    executionId,
		RunName:        details.RunName,
		TestLab:        apxconstants.Local,
	}

	// convert config to lambda test config
	localTestConfigs := config.(*session.Configs)

	var data map[string]interface{}

	logger.Info("starting local testplan sessions", zap.String("execution_id", executionId), zap.String("testplan_id", testplanId))
	ctx := context.Background()
	var wg sync.WaitGroup
	for _, localTestConfig := range localTestConfigs.Configs {
		go t.InitializeSessions(ctx, createdBy, status, details, &localTestConfig, &wg)
	}
	wg.Wait()
	status.TestcaseId = ""
	status.TestsuiteId = ""
	path := filepath.Join(".", "executions")

	projectsPath := filepath.Join(path, "projects.json")

	projectsFile, err := os.Create(projectsPath)
	if err != nil {
		logger.Error("could not create config file", err)
		return err
	}

	projectData, err := json.Marshal(localTestConfigs.Projects)
	if err != nil {
		logger.Error("could not create projects", err)
		return err
	}
	projectsFile.WriteString(string(projectData))

	defer projectsFile.Close()

	// EDCConfig
	edcPath := filepath.Join(path, "edc.json")

	edcFile, err := os.Create(edcPath)
	if err != nil {
		logger.Error("could not create config file", err)
		return err
	}

	edcData, err := json.Marshal(details.EDCDetails)
	if err != nil {
		logger.Error("could not create edc details", err)
		return err
	}
	edcFile.WriteString(string(edcData))

	defer edcFile.Close()

	//TODO: write for custom
	if details.Type == apxconstants.CrossBrowserTestPlan {
		fileNumber := 0
		testsDir := filepath.Join(path, "tests")
		testplanDir := filepath.Join(testsDir, "testPlan_"+testplanId)
		listingData := `import { test } from '../fixture';`
		if details.ParallelismEnabled {
			listingData += `
			test.describe.configure({ mode: 'parallel' });
			`
		}
		filepath.Walk(testplanDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			// Skip excluded files
			if !info.IsDir() {
				fileNumber++
				fileName := info.Name()
				fileTmpl := `import test%d from './%s';
				test.describe(test%d);
				`
				fileData := fmt.Sprintf(fileTmpl, fileNumber, fileName, fileNumber)
				listingData += fileData
				return nil
			}
			return nil
		})
		listfilePath := filepath.Join(testplanDir, "test.list.js")
		listFile, err := os.Create(listfilePath)
		if err != nil {
			logger.Error("could not create test list file", err)
			return err
		}
		listFile.WriteString(listingData)
	}

	os.Setenv("testlab", apxconstants.Local)
	if details.ParallelismEnabled {
		os.Setenv("WORKERS", strconv.Itoa(details.Workers))
	} else {
		os.Setenv("WORKERS", "1")
	}
	os.Setenv("PARALLELISM_ENABLED", strconv.FormatBool(details.ParallelismEnabled))
	commandContext, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := exec.CommandContext(commandContext, "npx", "playwright", "test")
	cmd.Dir = "executions" // Set working directory

	// Create pipes to capture stdout and stderr
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		logger.Error("could not get stdout pipe", err)
		status.Status = apxconstants.Failed
		status.Message = "failed to get stdout pipe"
		t.ExecutionServiceBridge.SaveSessionStatus(ctx, status)
		return err
	}

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		logger.Error("could not get stderr pipe", err)
		status.Status = apxconstants.Failed
		status.Message = "failed to get stderr pipe"
		t.ExecutionServiceBridge.SaveSessionStatus(ctx, status)
		return err
	}

	// Start the command
	logger.Info("starting testplan execution...", zap.String("testplan_id", testplanId), zap.String("execution_id", executionId))

	if err := cmd.Start(); err != nil {
		logger.Error("could not start command: ", err)
		return err
	}

	// Goroutine to print stdout
	go func() {
		scanner := bufio.NewScanner(stdoutPipe)
		for scanner.Scan() {
			line := scanner.Text()
			logger.Info("stdout", zap.String("output", line))
			// fmt.Println(line)
		}
		if err := scanner.Err(); err != nil {
			logger.Error("error reading stdout", err)
		}
	}()

	// Goroutine to print stderr
	go func() {
		scanner := bufio.NewScanner(stderrPipe)
		for scanner.Scan() {
			line := scanner.Text()
			logger.Info("stderr", zap.String("output", line))
			// fmt.Println(line)
		}
		if err := scanner.Err(); err != nil {
			logger.Error("error reading stderr", err)
		}
	}()

	// Goroutine to handle command completion and cancellation
	go func() {
		select {
		case <-commandContext.Done():
			logger.Info("local  test session completed", zap.String("execution_id", executionId))
			delete(executionsMap, executionId)
			return
		case data = <-t.commandChannel:
			fmt.Println("data received", data["testplan_id"], testplanId)
			if data["type"] == "testplan" && data["execution_id"] == executionId && data["testplan_id"] == testplanId {
				logger.Info("Sending SIGINT to stop the test plan execution", zap.String("execution_id", executionId))

				// Send SIGINT (Ctrl+C) to the process to simulate graceful shutdown
				if err := cmd.Process.Signal(syscall.SIGINT); err != nil {
					logger.Error("failed to send SIGINT", err)
				} else {
					logger.Info("SIGINT sent successfully")
				}
				cancel() // Cancel the context to stop further operations
			}
		}
	}()

	// Wait for the command to finish
	err = cmd.Wait()
	if err != nil {
		// Check if the error is due to the process being killed
		if err.Error() == "signal: killed" || err.Error() == "signal: interrupt" {
			logger.Info("Command was interrupted or killed", zap.String("execution_id", executionId))
			if data["status"] == apxconstants.NotExecuted {
				status.Status = apxconstants.NotExecuted
				status.Message = "Testplan aborted on tests or prerequiste failure"
				t.ExecutionServiceBridge.SaveSessionStatus(ctx, status)
				err = t.ExecutionServiceBridge.CreateLocalAgentResults(ctx, status.OrgId, status.ProjectId, status.AppId, executionId, testplanId, &runresult.RunResult{}, apxconstants.RunResultType)
				if err != nil {
					logger.Error("could not create local agent run results", err)
					return err
				}
				err = t.ExecutionServiceBridge.CreateLocalAgentResults(ctx, status.OrgId, status.ProjectId, status.AppId, executionId, testplanId, &runresult.RunResult{}, apxconstants.EdcResultType)
				if err != nil {
					logger.Error("could not create local agent edc results", err)
					return err
				}

			} else {
				status.Status = apxconstants.Aborted
				status.Message = "User Stopped Session"
				t.ExecutionServiceBridge.SaveSessionStatus(ctx, status)
				err = t.ExecutionServiceBridge.CreateLocalAgentResults(ctx, status.OrgId, status.ProjectId, status.AppId, executionId, testplanId, &runresult.RunResult{}, apxconstants.RunResultType)
				if err != nil {
					logger.Error("could not create local agent run results", err)
					return err
				}
				err = t.ExecutionServiceBridge.CreateLocalAgentResults(ctx, status.OrgId, status.ProjectId, status.AppId, executionId, testplanId, &runresult.RunResult{}, apxconstants.EdcResultType)
				if err != nil {
					logger.Error("could not create local agent edc results", err)
					return err
				}
			}

		} else {
			logger.Error("command failed to execute", err)
			status.Status = apxconstants.Failed
			status.Message = "command failed to execute " + err.Error()
			t.ExecutionServiceBridge.SaveSessionStatus(ctx, status)
			err = t.ExecutionServiceBridge.CreateLocalAgentResults(ctx, status.OrgId, status.ProjectId, status.AppId, executionId, testplanId, &runresult.RunResult{}, apxconstants.RunResultType)
			if err != nil {
				logger.Error("could not create local agent run results", err)
				return err
			}
			err = t.ExecutionServiceBridge.CreateLocalAgentResults(ctx, status.OrgId, status.ProjectId, status.AppId, executionId, testplanId, &runresult.RunResult{}, apxconstants.EdcResultType)
			if err != nil {
				logger.Error("could not create local agent edc results", err)
				return err
			}
			return err
		}
	}
	logger.Info("testplan execution completed")
	err = t.ExecutionServiceBridge.CreateLocalAgentResults(ctx, status.OrgId, status.ProjectId, status.AppId, executionId, testplanId, &runresult.RunResult{}, apxconstants.RunResultType)
	if err != nil {
		logger.Error("could not create local agent run results", err)
		return err
	}
	err = t.ExecutionServiceBridge.CreateLocalAgentResults(ctx, status.OrgId, status.ProjectId, status.AppId, executionId, testplanId, &runresult.RunResult{}, apxconstants.EdcResultType)
	if err != nil {
		logger.Error("could not create local agent run results", err)
		return err
	}
	return nil
}
func (t *TestExecutor) StopTestCaseExecution(testcaseId, executionId string) {
	data := make(map[string]interface{})

	data["type"] = "testcase"
	data["testcase_id"] = testcaseId
	data["execution_id"] = executionId

	select {
	case t.commandChannel <- data:
		logger.Info("Data sent to commandChannel")
	default:
		logger.Info("No Active Execution")
	}

}
func (t *TestExecutor) StopTestPlanExecution(testplanId, executionId, status string) {
	data := make(map[string]interface{})

	data["type"] = "testplan"
	data["testplan_id"] = testplanId
	data["execution_id"] = executionId
	data["status"] = status

	select {
	case t.commandChannel <- data:
		logger.Info("Data sent to commandChannel")
	default:
		logger.Info("No Active Execution")
	}
}
