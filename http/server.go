package http

import (
	"context"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"go.uber.org/zap"

	"agent/config"
	"agent/errors"
	"agent/http/handlers"
	apxmiddlewares "agent/http/middleware"
	apxresp "agent/http/response"
	"agent/logger"
	"agent/utils/helpers"
)

type Server struct {
	Logger                 *zap.Logger
	Conf                   *config.ApxConfig
	AgentHandler           *handlers.AgentHandler
	ExecutionBridgeHandler *handlers.ExecutionBridgeHandler
}

func NewServer(conf *config.ApxConfig, agentHandler *handlers.AgentHandler, executionBridgeHandler *handlers.ExecutionBridgeHandler) *Server {
	return &Server{
		Conf:                   conf,
		AgentHandler:           agentHandler,
		ExecutionBridgeHandler: executionBridgeHandler,
	}
}

func (s *Server) Listen(ctx context.Context, addr string) error {
	os.Setenv("BASE_PATH", strings.Replace(s.Conf.Prefix, "/", "", -1))

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(apxmiddlewares.NewLoggerWithMetrics(s.Logger, &apxmiddlewares.Opts{
		WithReferer:   false,
		WithUserAgent: false,
	}))
	r.Use(middleware.Recoverer)
	r.Use(apxmiddlewares.EnabCors(s.Conf.Cors.AllowedOrigins))
	r.Route(s.Conf.Prefix, func(r chi.Router) {
		r.Route("/v1", func(r chi.Router) {
			r.Post("/start", s.ToHTTPHandlerFunc(s.AgentHandler.StartAgentHandler))
			r.Route("/organisations", func(r chi.Router) {
				r.Route("/{org_id}", func(r chi.Router) {
					r.Route("/projects", func(r chi.Router) {
						r.Route("/{project_id}", func(r chi.Router) {
							r.Route("/apps", func(r chi.Router) {
								r.Route("/{app_id}", func(r chi.Router) {
									r.Route("/local-agent", func(r chi.Router) {
										r.Get("/status", s.ToHTTPHandlerFunc(s.AgentHandler.GetAgentStatus))
										r.Post("/network-logs", s.ToHTTPHandlerFunc(s.ExecutionBridgeHandler.CreateLocalAgentNetworkLogs))
									})
									r.Route("/{testlab}", func(r chi.Router) {
										r.Route("/sessions", func(r chi.Router) {
											r.Post("/", s.ToHTTPHandlerFunc(s.ExecutionBridgeHandler.SaveSession))
											r.Put("/", s.ToHTTPHandlerFunc(s.ExecutionBridgeHandler.UpdateSession))
										})
										r.Route("/{execution_id}", func(r chi.Router) {
											r.Post("/update-status", s.ToHTTPHandlerFunc(s.ExecutionBridgeHandler.UpdateExecutionStatus))
											r.Put("/update-stepcount", s.ToHTTPHandlerFunc(s.ExecutionBridgeHandler.UpdateStepCount))
											r.Post("/upload-screenshots", s.ToHTTPHandlerFunc(s.ExecutionBridgeHandler.UploadScreenshots))
											r.Post("/take-screenshot", s.ToHTTPHandlerFunc(s.ExecutionBridgeHandler.TakeScreenshot))
											r.Post("/upload-video", s.ToHTTPHandlerFunc(s.ExecutionBridgeHandler.UploadVideo))
										})
									})
								})
							})
						})

					})
				})
			})
		})
	})

	errch := make(chan error)
	server := &http.Server{Addr: addr, Handler: r}
	go func() {
		logger.Info("Starting server", zap.String("addr", addr))
		errch <- server.ListenAndServe()
	}()

	select {
	case err := <-errch:
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return server.Shutdown(shutdownCtx)
	}
}

func (s *Server) ToHTTPHandlerFunc(handler func(w http.ResponseWriter, r *http.Request) (any, int, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		response, status, err := handler(w, r)
		if err != nil {
			switch err := err.(type) {
			case *errors.Error:
				helpers.PrintStruct(err)
				apxresp.RespondError(w, err)
			default:
				s.Logger.Error("internal error", zap.Error(err))
				apxresp.RespondMessage(w, http.StatusInternalServerError, "internal error")
			}
			return
		}
		if response != nil {
			apxresp.RespondJSON(w, status, response)
		}
		if status >= 100 && status < 600 {
			w.WriteHeader(status)
		}
	}
}
