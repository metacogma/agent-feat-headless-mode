package handlers

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi"

	"agent/errors"
	"agent/models/executionstatus"
	"agent/models/executionstep"
	"agent/models/runresult"
	"agent/models/screenshot"
	"agent/models/session"
	"agent/models/uploadvideo"
)

type ExecutionBridge interface {
	SaveSession(ctx context.Context, orgId string, projectId string, appId string, testlab string, session session.Session) error
	SaveSessionStatus(ctx context.Context, status executionstatus.ExecutionStatus) error
	CreateLocalAgentResults(ctx context.Context, orgId string, projectId string, appId string, executionId string, testPlanId string, runResult *runresult.RunResult, resultType string) error
	UpdateSession(ctx context.Context, orgId string, projectId string, appId string, testlab string, session session.Session) error
	CreateLocalAgentNetworkLogs(ctx context.Context, session session.Session) error
	UpdateStepCount(ctx context.Context, orgId string, projectId string, appId string, testlab string, executionId string, body executionstep.ExecutionStep) error
	UploadScreenshots(ctx context.Context, orgId string, projectId string, appId string, testlab string, executionId string, screenshot screenshot.UploadScreenshotRequest) error
	TakeScreenshot(ctx context.Context, orgId string, projectId string, appId string, testlab string, executionId string, screenshot screenshot.TakeScreenshot) error
	UploadVideo(ctx context.Context, data uploadvideo.UploadVideo) error
}

type ExecutionBridgeHandler struct {
	ExecutionBridge ExecutionBridge
}

func NewExecutionBridgeHandler(executionBridge ExecutionBridge) *ExecutionBridgeHandler {
	return &ExecutionBridgeHandler{
		ExecutionBridge: executionBridge,
	}
}

func (h *ExecutionBridgeHandler) TakeScreenshot(w http.ResponseWriter, r *http.Request) (response any, status int, err error) {
	orgId := chi.URLParam(r, "org_id")
	if orgId == "" {
		return nil, http.StatusBadRequest, errors.EmptyParamErr("org_id")
	}
	projectId := chi.URLParam(r, "project_id")
	if projectId == "" {
		return nil, http.StatusBadRequest, errors.EmptyParamErr("project_id")
	}
	appId := chi.URLParam(r, "app_id")
	if appId == "" {
		return nil, http.StatusBadRequest, errors.EmptyParamErr("app_id")
	}
	testLab := chi.URLParam(r, "testlab")
	if testLab == "" {
		return nil, http.StatusBadRequest, errors.EmptyParamErr("testlab")
	}
	executionId := chi.URLParam(r, "execution_id")
	if executionId == "" {
		return nil, http.StatusBadRequest, errors.EmptyParamErr("execution_id")
	}

	var screenshot screenshot.TakeScreenshot
	err = json.NewDecoder(r.Body).Decode(&screenshot)
	if err != nil {
		return nil, http.StatusBadRequest, errors.InvalidBodyErr(err)
	}
	go h.ExecutionBridge.TakeScreenshot(r.Context(), orgId, projectId, appId, testLab, executionId, screenshot)
	if err == nil {
		return nil, http.StatusOK, nil

	}
	return
}

func (h *ExecutionBridgeHandler) UploadScreenshots(w http.ResponseWriter, r *http.Request) (response any, status int, err error) {
	var body screenshot.UploadScreenshotRequest
	err = json.NewDecoder(r.Body).Decode(&body)
	if err != nil {
		return nil, http.StatusBadRequest, errors.InvalidBodyErr(err)
	}
	if err := body.Validate(); err != nil {
		return nil, http.StatusBadRequest, errors.ValidationFailedErr(err)
	}
	testlab := chi.URLParam(r, "testlab")
	if testlab == "" {
		return nil, http.StatusBadRequest, errors.EmptyParamErr("testlab")
	}
	orgId := chi.URLParam(r, "org_id")
	if orgId == "" {
		return nil, http.StatusBadRequest, errors.EmptyParamErr("org_id")
	}
	projectId := chi.URLParam(r, "project_id")
	if projectId == "" {
		return nil, http.StatusBadRequest, errors.EmptyParamErr("project_id")
	}
	appId := chi.URLParam(r, "app_id")
	if appId == "" {
		return nil, http.StatusBadRequest, errors.EmptyParamErr("app_id")
	}
	executionId := chi.URLParam(r, "execution_id")
	if executionId == "" {
		return nil, http.StatusBadRequest, errors.EmptyParamErr("execution_id")
	}

	go h.ExecutionBridge.UploadScreenshots(r.Context(), orgId, projectId, appId, testlab, executionId, body)

	if err == nil {
		return nil, http.StatusOK, nil
	}
	return
}

func (h *ExecutionBridgeHandler) UpdateExecutionStatus(w http.ResponseWriter, r *http.Request) (response any, status int, err error) {
	var body executionstatus.ExecutionStatus
	err = json.NewDecoder(r.Body).Decode(&body)
	if err != nil {
		return nil, http.StatusBadRequest, errors.InvalidBodyErr(err)
	}
	if err := body.Validate(); err != nil {
		return nil, http.StatusBadRequest, errors.ValidationFailedErr(err)
	}

	go h.ExecutionBridge.SaveSessionStatus(r.Context(), body)
	return nil, http.StatusOK, nil
}

func (h *ExecutionBridgeHandler) SaveSession(w http.ResponseWriter, r *http.Request) (response any, status int, err error) {
	testlab := chi.URLParam(r, "testlab")
	if testlab == "" {
		return nil, http.StatusBadRequest, errors.EmptyParamErr("testlab")
	}
	orgId := chi.URLParam(r, "org_id")
	if orgId == "" {
		return nil, http.StatusBadRequest, errors.EmptyParamErr("org_id")
	}
	projectId := chi.URLParam(r, "project_id")
	if projectId == "" {
		return nil, http.StatusBadRequest, errors.EmptyParamErr("project_id")
	}
	appId := chi.URLParam(r, "app_id")
	if appId == "" {
		return nil, http.StatusBadRequest, errors.EmptyParamErr("app_id")
	}

	var session session.Session
	err = json.NewDecoder(r.Body).Decode(&session)
	if err != nil {
		return nil, http.StatusBadRequest, errors.InvalidBodyErr(err)
	}

	go h.ExecutionBridge.SaveSession(r.Context(), orgId, projectId, appId, testlab, session)

	return nil, http.StatusOK, nil
}
func (h *ExecutionBridgeHandler) UpdateSession(w http.ResponseWriter, r *http.Request) (response any, status int, err error) {
	testlab := chi.URLParam(r, "testlab")
	if testlab == "" {
		return nil, http.StatusBadRequest, errors.EmptyParamErr("testlab")
	}
	orgId := chi.URLParam(r, "org_id")
	if orgId == "" {
		return nil, http.StatusBadRequest, errors.EmptyParamErr("org_id")
	}
	projectId := chi.URLParam(r, "project_id")
	if projectId == "" {
		return nil, http.StatusBadRequest, errors.EmptyParamErr("project_id")
	}
	appId := chi.URLParam(r, "app_id")
	if appId == "" {
		return nil, http.StatusBadRequest, errors.EmptyParamErr("app_id")
	}

	var session session.Session
	err = json.NewDecoder(r.Body).Decode(&session)
	if err != nil {
		return nil, http.StatusBadRequest, errors.InvalidBodyErr(err)
	}
	go h.ExecutionBridge.UpdateSession(r.Context(), orgId, projectId, appId, testlab, session)
	return nil, http.StatusOK, nil
}

func (h *ExecutionBridgeHandler) UpdateStepCount(w http.ResponseWriter, r *http.Request) (response any, status int, err error) {
	testlab := chi.URLParam(r, "testlab")
	if testlab == "" {
		return nil, http.StatusBadRequest, errors.EmptyParamErr("testlab")
	}
	orgId := chi.URLParam(r, "org_id")
	if orgId == "" {
		return nil, http.StatusBadRequest, errors.EmptyParamErr("org_id")
	}
	projectId := chi.URLParam(r, "project_id")
	if projectId == "" {
		return nil, http.StatusBadRequest, errors.EmptyParamErr("project_id")
	}
	appId := chi.URLParam(r, "app_id")
	if appId == "" {
		return nil, http.StatusBadRequest, errors.EmptyParamErr("app_id")
	}
	executionId := chi.URLParam(r, "execution_id")
	if executionId == "" {
		return nil, http.StatusBadRequest, errors.EmptyParamErr("execution_id")
	}

	var body executionstep.ExecutionStep
	err = json.NewDecoder(r.Body).Decode(&body)
	if err != nil {
		return nil, http.StatusBadRequest, errors.InvalidBodyErr(err)
	}

	err = body.Validate()
	if err != nil {
		return nil, http.StatusBadRequest, errors.ValidationFailedErr(err)
	}

	go h.ExecutionBridge.UpdateStepCount(r.Context(), orgId, projectId, appId, testlab, executionId, body)

	return nil, http.StatusOK, nil

}

func (h *ExecutionBridgeHandler) CreateLocalAgentNetworkLogs(w http.ResponseWriter, r *http.Request) (response any, status int, err error) {
	var session session.Session
	err = json.NewDecoder(r.Body).Decode(&session)
	if err != nil {
		return nil, http.StatusBadRequest, errors.InvalidBodyErr(err)
	}

	go h.ExecutionBridge.CreateLocalAgentNetworkLogs(r.Context(), session)
	return nil, http.StatusOK, nil
}

func (h *ExecutionBridgeHandler) UploadVideo(w http.ResponseWriter, r *http.Request) (response any, status int, err error) {
	var body uploadvideo.UploadVideo

	err = json.NewDecoder(r.Body).Decode(&body)
	if err != nil {
		return nil, http.StatusBadRequest, errors.InvalidBodyErr(err)
	}
	go h.ExecutionBridge.UploadVideo(r.Context(), body)
	return nil, http.StatusOK, nil
}
