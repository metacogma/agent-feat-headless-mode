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

	}

	var device map[string]string
	err = json.Unmarshal(deviceConfiguration, &device)
	if err != nil {
		logger.Error("error unmarshalling device config file", err)

	}

	if device["device_id"] == machineId {
		return map[string]string{"status": "online"}, http.StatusOK, nil
	}

	return
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
