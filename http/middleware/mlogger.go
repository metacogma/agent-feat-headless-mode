package apxmiddlewares

import (
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/middleware"
	"github.com/samber/lo"
	"go.uber.org/zap"
)

// mlogger is custom middleware stack that contains logger middleware and metrics middleware

// Opts contains the logger middleware configuration.
type Opts struct {
	// WithReferer enables logging the "Referer" HTTP header value.
	WithReferer bool

	// WithUserAgent enables logging the "User-Agent" HTTP header value.
	WithUserAgent bool
}

type path string

const (
	health  path = "/health"
	metrics path = "/metrics"
)

var healthAndMonitorPaths = []path{health, metrics}

// getAPIType returns the API request service type.
func getAPIType(r *http.Request) path {
	switch {
	case strings.Contains(r.URL.Path, "/metrics"):
		return metrics
	case strings.Contains(r.URL.Path, "/health"):
		return health
	default:
		return ""
	}
}

// NewLoggerWithMetrics returns a logger middleware for chi, that implements the http.Handler interface.
// It also logs the application metrics for the requests.
func NewLoggerWithMetrics(logger *zap.Logger, opts *Opts) func(next http.Handler) http.Handler {
	if logger == nil {
		return func(next http.Handler) http.Handler { return next }
	}
	if opts == nil {
		opts = &Opts{}
	}

	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			t1 := time.Now()
			defer func() {
				requestDuration := time.Since(t1).Milliseconds()
				reqLogger := logger.With(
					zap.String("proto", r.Proto),
					zap.String("path", r.URL.Path),
					zap.String("reqId", middleware.GetReqID(r.Context())),
					zap.Int64("latency", requestDuration),
					zap.Int("status", ww.Status()),
					zap.Int("size", ww.BytesWritten()),
					zap.String("method", r.Method),
				)

				if opts.WithReferer {
					ref := ww.Header().Get("Referer")
					if ref == "" {
						ref = r.Header.Get("Referer")
					}
					if ref != "" {
						reqLogger = reqLogger.With(zap.String("ref", ref))
					}
				}
				if opts.WithUserAgent {
					ua := ww.Header().Get("User-Agent")
					if ua == "" {
						ua = r.Header.Get("User-Agent")
					}
					if ua != "" {
						reqLogger = reqLogger.With(zap.String("ua", ua))
					}
				}

				t := getAPIType(r)
				if !lo.Contains(healthAndMonitorPaths, t) {
					reqLogger.Info("Served")
				} else {
					reqLogger.Debug("Served")
				}
			}()
			next.ServeHTTP(ww, r)
		}
		return http.HandlerFunc(fn)
	}
}
