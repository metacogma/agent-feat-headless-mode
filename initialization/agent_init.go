package init

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/google/uuid"
	"github.com/pkg/browser"

	"agent/config"
	"agent/logger"
	autotestbridge "agent/services/autotest_bridge"
	"agent/utils/helpers"
)

func EnsureConfigFile() (*os.File, error) {
	configurationFolderPath := filepath.Join(".", "configuration")
	err := helpers.CreateFolder(configurationFolderPath)
	if err != nil {
		fmt.Println("error creating configuration folder", err)
		return nil, err
	}

	machineConfigFilePath := filepath.Join(configurationFolderPath, "machine_config.json")
	machineConfigFile, err := helpers.CreateFile(machineConfigFilePath)
	if err != nil {
		fmt.Println("error creating machine config file", err)
		return nil, err
	}

	return machineConfigFile, nil
}

func EnsureRegistration(apxconfig *config.ApxConfig, autotestBridgeSvc *autotestbridge.AutotestBridgeService, isBackground bool) (string, error) {
	machineConfigFile, err := EnsureConfigFile()
	if err != nil {
		logger.Error("error creating machine config file", err)
		return "", err
	}
	defer machineConfigFile.Close()

	byteData, err := io.ReadAll(machineConfigFile)
	if err != nil {
		logger.Error("error reading machine config file", err)
		return "", err
	}

	if len(byteData) != 0 {
		err = json.Unmarshal(byteData, &apxconfig)
		if err != nil {
			logger.Error("error unmarshalling machine config file", err)
			return "", err
		}
	}
	var machineId string

	if apxconfig.MachineId == "" {
		uuid := uuid.New()
		apxconfig.MachineId = uuid.String()
		machineId = apxconfig.MachineId
		bytes, err := json.MarshalIndent(apxconfig, "", "  ")
		if err != nil {
			logger.Error("error marshalling machine config file", err)
			return "", err
		}
		err = machineConfigFile.Truncate(0)
		if err != nil {
			logger.Error("error truncating file", err)
			return "", err
		}
		_, err = machineConfigFile.Seek(0, 0)
		if err != nil {
			logger.Error("error seeking to beginning of file", err)
			return "", err
		}
		_, err = machineConfigFile.Write(bytes)
		if err != nil {
			logger.Error("error writing to machine config file", err)
			return "", err
		}

		_, err = autotestBridgeSvc.InsertLocalDevice(apxconfig.ToLocalDevice(), machineId)
		if err != nil {
			logger.Error("error inserting local device", err)
			return "", err
		}

	} else {
		machineId = apxconfig.MachineId
		parsedUrl, err := url.Parse(apxconfig.ServerDomain + "/checkDeviceRegistration")
		if err != nil {
			logger.Error("error parsing url", err)
			return "", err
		}
		queryParams := url.Values{}
		queryParams.Add("machineId", machineId)
		parsedUrl.RawQuery = queryParams.Encode()
		res, err := http.Get(parsedUrl.String())
		if err != nil {
			logger.Error("error getting registration status", err)
			//FIX ME : Add retry logic
			// handle based on error scenarios : 1) service not available , 2) document not present in mongo.
			return "", err
		}
		defer res.Body.Close()
	}

	if isBackground {
		return machineId, nil
	}

	browserUrl, err := url.Parse(apxconfig.DashboardDomain + "/agent")
	if err != nil {
		logger.Error("error parsing url", err)
		return "", err
	}
	queryParams := url.Values{}
	url := fmt.Sprintf("http://localhost%s%s/v1/start", apxconfig.Listen, apxconfig.Prefix)
	logger.Info("endPoint", url)
	queryParams.Add("endPoint", url)
	queryParams.Add("machineId", machineId)
	queryParams.Add("orgId", apxconfig.OrgId)
	queryParams.Add("projectId", apxconfig.ProjectId)
	queryParams.Add("nCores", fmt.Sprint(runtime.NumCPU()))
	browserUrl.RawQuery = queryParams.Encode()

	err = browser.OpenURL(browserUrl.String())
	if err != nil {
		logger.Error("error opening browser", err)
		return "", err
	}
	return machineId, nil

}

func InstallDependencies(folderPath string) error {
	cmd := exec.Command("npm", "i")
	cmd.Dir = folderPath
	stdoutPipe, _ := cmd.StdoutPipe()
	stderrPipe, _ := cmd.StderrPipe()

	defer stdoutPipe.Close()
	defer stderrPipe.Close()

	logger.Info("Running 'npm i' inside folder", folderPath)

	err := cmd.Start()
	if err != nil {
		logger.Error("Error running 'npm i' inside folder", folderPath, err)
	}
	go helpers.StdOutput(stdoutPipe)
	go helpers.StdError(stderrPipe)
	cmd.Wait()

	playwrightCmd := exec.Command("npm", "install", "playwright@1.47.2")
	playwrightCmd.Dir = folderPath
	playwrightCmdStdoutPipe, _ := playwrightCmd.StdoutPipe()
	playwrightCmdStderrPipe, _ := playwrightCmd.StderrPipe()

	logger.Info("Running 'npm install playwright@1.47.2' inside folder", folderPath)

	err = playwrightCmd.Start()

	if err != nil {
		logger.Error("Error running 'npm install playwright@1.47.2' inside folder", folderPath, err)
	}

	go helpers.StdOutput(playwrightCmdStdoutPipe)
	go helpers.StdError(playwrightCmdStderrPipe)

	playwrightCmd.Wait()

	logger.Info("Running 'npx playwright install' inside folder", folderPath)

	depCmd := exec.Command("npx", "playwright", "install")
	depCmd.Dir = folderPath
	depCmdStdoutPipe, _ := depCmd.StdoutPipe()
	depCmdStderrPipe, _ := depCmd.StderrPipe()

	logger.Info("Running 'npx  playwright install' inside folder", folderPath)

	err = depCmd.Start()
	if err != nil {
		logger.Error("Error running 'npx  playwright install' inside folder", folderPath, err)
	}
	go helpers.StdOutput(depCmdStdoutPipe)
	go helpers.StdError(depCmdStderrPipe)

	depCmd.Wait()
	return err
}
