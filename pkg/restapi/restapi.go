// Copyright (C) 2017 ScyllaDB

package restapi

import (
	"fmt"
	"net/http"
	"sync"

	"github.com/go-chi/chi"
	"github.com/go-chi/render"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/scylladb/go-log"
	"github.com/scylladb/mermaid/pkg/scyllaclient"
	"github.com/scylladb/mermaid/pkg/util/httphandler"
	"github.com/scylladb/mermaid/pkg/util/httplog"
)

func init() {
	render.Respond = responder
}

// New returns an http.Handler implementing mermaid v1 REST API.
func New(services Services, logger log.Logger) http.Handler {
	r := chi.NewRouter()

	r.Use(
		interactive,
		httplog.TraceID,
		httplog.RequestLogger(logger),
		render.SetContentType(render.ContentTypeJSON),
	)

	r.Get("/ping", httphandler.Heartbeat())
	r.Get("/version", httphandler.Version())
	r.Get("/api/v1/version", httphandler.Version()) // For backwards compatibility

	r.Mount("/api/v1/", newClusterHandler(services.Cluster))
	f := clusterFilter{svc: services.Cluster}.clusterCtx
	r.With(f).Mount("/api/v1/cluster/{cluster_id}/status", newStatusHandler(services.Cluster, services.HealthCheck))
	r.With(f).Mount("/api/v1/cluster/{cluster_id}/tasks", newTasksHandler(services))
	r.With(f).Mount("/api/v1/cluster/{cluster_id}/task", newTaskHandler(services))
	r.With(f).Mount("/api/v1/cluster/{cluster_id}/backups", newBackupHandler(services))
	r.With(f).Mount("/api/v1/cluster/{cluster_id}/repairs", newRepairHandler(services))

	// NotFound registered last due to https://github.com/go-chi/chi/issues/297
	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		logger.Info(r.Context(), "Request path not found", "path", r.URL.Path)
		render.Respond(w, r, &httpError{
			StatusCode: http.StatusNotFound,
			Message:    fmt.Sprintf("find endpoint for path %s - make sure api-url is correct", r.URL.Path),
			TraceID:    log.TraceID(r.Context()),
		})
	})

	return r
}

func interactive(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r = r.WithContext(scyllaclient.Interactive(r.Context()))
		next.ServeHTTP(w, r)
	})
}

// NewPrometheus returns an http.Handler exposing Prometheus metrics on
// '/metrics'.
func NewPrometheus(svc ClusterService, mw *MetricsWatcher) http.Handler {
	r := chi.NewRouter()

	r.Get("/metrics", mw.requestHandler)

	// Exposing Consul API to Prometheus for discovering nodes.
	// The idea is to use already working discovering mechanism to avoid
	// extending Prometheus it self.
	r.Mount("/v1", newConsulHandler(svc))

	return r
}

// MetricsWatcher keeps track of registered callbacks for metrics requests.
type MetricsWatcher struct {
	mu        sync.Mutex
	callbacks []func() bool
}

func (mw *MetricsWatcher) requestHandler(w http.ResponseWriter, r *http.Request) {
	var unregister []int
	mw.mu.Lock()
	for i, callback := range mw.callbacks {
		if listening := callback(); !listening {
			unregister = append(unregister, i)
		}
	}
	for _, i := range unregister {
		mw.callbacks = append(mw.callbacks[:i], mw.callbacks[i+1:]...)
	}
	mw.mu.Unlock()
	promhttp.Handler().ServeHTTP(w, r)
}

// OnRequest registers callback to be executed when metrics are requested.
// If callback returns false upon execution, callback will be unregistered.
func (mw *MetricsWatcher) OnRequest(callback func() bool) {
	mw.mu.Lock()
	defer mw.mu.Unlock()
	if mw.callbacks == nil {
		mw.callbacks = make([]func() bool, 0)
	}
	mw.callbacks = append(mw.callbacks, callback)
}
