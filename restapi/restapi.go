// Copyright (C) 2017 ScyllaDB

package restapi

import (
	"net/http"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/go-chi/render"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/scylladb/mermaid/log"
)

func init() {
	render.Respond = httpErrorRender
}

// Services contains REST API services.
type Services struct {
	Cluster   ClusterService
	Repair    RepairService
	Scheduler SchedService
}

// New returns an http.Handler implementing mermaid v1 REST API.
func New(svc *Services, logger log.Logger) http.Handler {
	r := chi.NewRouter()
	r.Use(traceIDMiddleware)
	r.Use(middleware.RequestLogger(httpLogger{logger}))
	r.Use(recoverPanics)
	r.Use(render.SetContentType(render.ContentTypeJSON))

	if svc.Cluster != nil {
		r.Mount("/api/v1/", newClusterHandler(svc.Cluster))
	}

	if svc.Repair != nil {
		r.With(clusterFilter{svc: svc.Cluster}.clusterCtx).
			Mount("/api/v1/cluster/{cluster_id}/repair/", newRepairHandler(svc.Repair, svc.Scheduler))
	}

	if svc.Scheduler != nil {
		r.With(clusterFilter{svc: svc.Cluster}.clusterCtx).
			Mount("/api/v1/cluster/{cluster_id}/", newSchedHandler(svc.Scheduler))
	}

	r.Get("/api/v1/version", newVersionHandler())

	r.Mount("/metrics", promhttp.Handler())

	return r
}

func httpErrorRender(w http.ResponseWriter, r *http.Request, v interface{}) {
	if err, ok := v.(error); ok {
		httpErr, _ := v.(*httpError)
		if httpErr == nil {
			httpErr = &httpError{
				Err:        err,
				StatusCode: http.StatusInternalServerError,
				Message:    "unexpected error, consult logs",
				TraceID:    log.TraceID(r.Context()),
			}
		}

		if le, _ := middleware.GetLogEntry(r).(*httpLogEntry); le != nil {
			le.AddFields("Error", httpErr.Error())
		}

		render.Status(r, httpErr.StatusCode)
		render.DefaultResponder(w, r, httpErr)
		return
	}

	render.DefaultResponder(w, r, v)
}
