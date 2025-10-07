package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"os"

	"agent/config"
	"agent/errors"
	"agent/logger"
	localexecution_model "agent/models/localexecution"
	"agent/services/executor"
)

type AgentHandler struct {
	ExecutionService *executor.TestCaseExecutorService
	Config           *config.ApxConfig
}

func NewAgentHandler(executionsvc *executor.TestCaseExecutorService, config *config.ApxConfig) *AgentHandler {
	return &AgentHandler{
		ExecutionService: executionsvc,
		Config:           config,
	}
}

func (a *AgentHandler) GetAgentStatus(w http.ResponseWriter, r *http.Request) (response any, status int, err error) {
	machineId := r.URL.Query().Get("machine_id")
	if machineId == "" {
		return nil, http.StatusBadRequest, errors.EmptyParamErr("machine_id")
	}

	deviceConfiguration, err := os.ReadFile("./configuration/machine_config.json")
	if err != nil {
		logger.Error("error reading device config file", err)
		return map[string]string{
			"status": "error",
			"error":  "failed to read configuration file",
		}, http.StatusInternalServerError, err
	}

	var device map[string]interface{}
	err = json.Unmarshal(deviceConfiguration, &device)
	if err != nil {
		logger.Error("error unmarshalling device config file", err)
		return map[string]string{
			"status": "error",
			"error":  "failed to parse configuration file",
		}, http.StatusInternalServerError, err
	}

	// Check both device_id and machine_id for backward compatibility
	configMachineId, ok := device["machine_id"].(string)
	if !ok {
		// Try device_id as fallback
		configMachineId, ok = device["device_id"].(string)
	}

	if ok && configMachineId == machineId {
		return map[string]interface{}{
			"status":     "online",
			"machine_id": configMachineId,
			"hostname":   device["hostname"],
		}, http.StatusOK, nil
	}

	return map[string]string{
		"status": "not_found",
		"error":  "machine_id does not match registered device",
	}, http.StatusNotFound, nil
}

func (a *AgentHandler) StartAgentHandler(w http.ResponseWriter, r *http.Request) (response any, status int, err error) {
	a.StartAgent(context.Background())
	return map[string]interface{}{"message": "agent started"}, http.StatusOK, nil
}

func (a *AgentHandler) StartAgent(ctx context.Context) {
	localexecutions := make(chan *localexecution_model.LocalExecution, 50)
	ctx = context.WithValue(ctx, "device_id", a.Config.MachineId)
	go a.ExecutionService.PollForLocalexecutions(ctx, localexecutions)
	go a.ExecutionService.StartLocalExecution(ctx, localexecutions)
}
