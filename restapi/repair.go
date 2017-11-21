// Copyright (C) 2017 ScyllaDB

package restapi

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"

	"github.com/go-chi/chi"
	"github.com/go-chi/render"
	"github.com/pkg/errors"
	"github.com/scylladb/mermaid/repair"
	"github.com/scylladb/mermaid/uuid"
)

//go:generate mockgen -source repair.go -destination ../mermaidmock/repairservice_mock.go -package mermaidmock

// RepairService is the repair service interface required by the repair REST API handlers.
type RepairService interface {
	GetUnit(ctx context.Context, clusterID uuid.UUID, idOrName string) (*repair.Unit, error)
	PutUnit(ctx context.Context, u *repair.Unit) error
	DeleteUnit(ctx context.Context, clusterID, ID uuid.UUID) error
	ListUnits(ctx context.Context, clusterID uuid.UUID, f *repair.UnitFilter) ([]*repair.Unit, error)
	// temporary
	Repair(ctx context.Context, u *repair.Unit, taskID uuid.UUID) error
	StopRun(ctx context.Context, u *repair.Unit, taskID uuid.UUID) error

	GetConfig(ctx context.Context, src repair.ConfigSource) (*repair.Config, error)
	PutConfig(ctx context.Context, src repair.ConfigSource, c *repair.Config) error
	DeleteConfig(ctx context.Context, src repair.ConfigSource) error

	GetRun(ctx context.Context, u *repair.Unit, taskID uuid.UUID) (*repair.Run, error)
	GetLastRun(ctx context.Context, u *repair.Unit) (*repair.Run, error)
	GetProgress(ctx context.Context, u *repair.Unit, taskID uuid.UUID, hosts ...string) ([]*repair.RunProgress, error)
}

type repairHandler struct {
	chi.Router
	svc RepairService
}

func newRepairHandler(svc RepairService) http.Handler {
	h := &repairHandler{
		Router: chi.NewRouter().With(requireClusterID),
		svc:    svc,
	}

	// unit
	h.Route("/units", func(r chi.Router) {
		r.Get("/", h.listUnits)
		r.Post("/", h.createUnit)
	})
	h.Route("/unit/{unit_id}", func(r chi.Router) {
		r.Use(h.unitCtx)
		r.Get("/", h.loadUnit)
		r.Put("/", h.updateUnit)
		r.Delete("/", h.deleteUnit)
		// temporary
		r.Put("/repair", h.startRepair)
		r.Put("/stop_repair", h.stopRepair)
	})

	// config
	h.Route("/config", func(r chi.Router) {
		r.Get("/", h.getConfig)
		r.Put("/", h.updateConfig)
		r.Delete("/", h.deleteConfig)

		r.Route("/{config_type}/{external_id}", func(r chi.Router) {
			r.Get("/", h.getConfig)
			r.Put("/", h.updateConfig)
			r.Delete("/", h.deleteConfig)
		})
	})

	// task
	h.Get("/task/{task_id}", h.repairProgress)

	return h
}

//
// unit
//

func (h *repairHandler) unitCtx(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		unitID := chi.URLParam(r, "unit_id")
		if unitID == "" {
			render.Respond(w, r, httpErrBadRequest(r, errors.New("missing unit ID")))
			return
		}

		u, err := h.svc.GetUnit(r.Context(), clusterIDFromCtx(r.Context()), unitID)
		if err != nil {
			notFoundOrInternal(w, r, err, "failed to load unit")
			return
		}

		ctx := context.WithValue(r.Context(), ctxRepairUnit, u)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (h *repairHandler) mustUnitFromCtx(r *http.Request) *repair.Unit {
	u, ok := r.Context().Value(ctxRepairUnit).(*repair.Unit)
	if !ok {
		panic("missing repair unit in context")
	}
	return u
}

func (h *repairHandler) parseUnit(r *http.Request) (*repair.Unit, error) {
	var u repair.Unit
	if err := render.DecodeJSON(r.Body, &u); err != nil {
		return nil, err
	}
	u.ClusterID = clusterIDFromCtx(r.Context())
	return &u, nil
}

func (h *repairHandler) listUnits(w http.ResponseWriter, r *http.Request) {
	ids, err := h.svc.ListUnits(r.Context(), clusterIDFromCtx(r.Context()), &repair.UnitFilter{})
	if err != nil {
		render.Respond(w, r, httpErrInternal(r, err, "failed to list units"))
		return
	}

	if len(ids) == 0 {
		render.Respond(w, r, []string{})
		return
	}
	render.Respond(w, r, ids)
}

func (h *repairHandler) createUnit(w http.ResponseWriter, r *http.Request) {
	newUnit, err := h.parseUnit(r)
	if err != nil {
		render.Respond(w, r, httpErrBadRequest(r, err))
		return
	}

	if err := h.svc.PutUnit(r.Context(), newUnit); err != nil {
		render.Respond(w, r, httpErrInternal(r, err, "failed to create unit"))
		return
	}

	location := r.URL.ResolveReference(&url.URL{Path: path.Join("unit", newUnit.ID.String())})
	w.Header().Set("Location", location.String())
	w.WriteHeader(http.StatusCreated)
}

func (h *repairHandler) loadUnit(w http.ResponseWriter, r *http.Request) {
	u := h.mustUnitFromCtx(r)
	render.Respond(w, r, u)
}

func (h *repairHandler) updateUnit(w http.ResponseWriter, r *http.Request) {
	u := h.mustUnitFromCtx(r)

	newUnit, err := h.parseUnit(r)
	if err != nil {
		render.Respond(w, r, httpErrBadRequest(r, err))
		return
	}
	newUnit.ID = u.ID

	if err := h.svc.PutUnit(r.Context(), newUnit); err != nil {
		render.Respond(w, r, httpErrInternal(r, err, "failed to update unit"))
		return
	}
	render.Respond(w, r, newUnit)
}

func (h *repairHandler) deleteUnit(w http.ResponseWriter, r *http.Request) {
	u := h.mustUnitFromCtx(r)

	if err := h.svc.DeleteUnit(r.Context(), clusterIDFromCtx(r.Context()), u.ID); err != nil {
		render.Respond(w, r, httpErrInternal(r, err, "failed to delete unit"))
		return
	}
}

func (h *repairHandler) startRepair(w http.ResponseWriter, r *http.Request) {
	u := h.mustUnitFromCtx(r)

	taskID := uuid.NewTime()

	if err := h.svc.Repair(r.Context(), u, taskID); err != nil {
		render.Respond(w, r, httpErrInternal(r, err, "failed to start repair"))
		return
	}

	repairURL := r.URL.ResolveReference(&url.URL{
		Path:     path.Join("../../task", taskID.String()),
		RawQuery: fmt.Sprintf("unit_id=%s", u.ID)},
	)
	w.Header().Set("Location", repairURL.String())
	w.WriteHeader(http.StatusCreated)
}

func (h *repairHandler) stopRepair(w http.ResponseWriter, r *http.Request) {
	u := h.mustUnitFromCtx(r)

	task, err := h.svc.GetLastRun(r.Context(), u)
	if err != nil {
		notFoundOrInternal(w, r, err, "failed to load task")
	}

	if err := h.svc.StopRun(r.Context(), u, task.ID); err != nil {
		render.Respond(w, r, httpErrInternal(r, err, "failed to stop repair"))
		return
	}

	repairURL := r.URL.ResolveReference(&url.URL{
		Path:     path.Join("../../task", task.ID.String()),
		RawQuery: fmt.Sprintf("unit_id=%s", u.ID)},
	)
	w.Header().Set("Location", repairURL.String())
	w.WriteHeader(http.StatusCreated)
}

//
// config
//

type repairConfigRequest struct {
	*repair.Config
	repair.ConfigSource `json:"-"`
}

func parseConfigRequest(r *http.Request) (*repairConfigRequest, error) {
	var cr repairConfigRequest
	if err := render.DecodeJSON(r.Body, &cr.Config); err != nil {
		if err != io.EOF {
			return nil, httpErrBadRequest(r, err)
		}
	}

	routeCtx := chi.RouteContext(r.Context())
	if typ := routeCtx.URLParam("config_type"); typ != "" {
		err := cr.Type.UnmarshalText([]byte(typ))
		if err != nil {
			return nil, httpErrBadRequest(r, errors.Wrap(err, "bad config type"))
		}
	} else {
		cr.Type = repair.ClusterConfig
	}
	switch cr.Type {
	case repair.UnitConfig, repair.KeyspaceConfig, repair.ClusterConfig:
	default:
		return nil, httpErrBadRequest(r, fmt.Errorf("config type %q not allowed", cr.Type))
	}

	if id := routeCtx.URLParam("external_id"); id != "" {
		cr.ExternalID = id
	}
	cr.ClusterID = clusterIDFromCtx(r.Context())
	return &cr, nil
}

func (h *repairHandler) getConfig(w http.ResponseWriter, r *http.Request) {
	cr, err := parseConfigRequest(r)
	if err != nil {
		render.Respond(w, r, err)
		return
	}

	c, err := h.svc.GetConfig(r.Context(), cr.ConfigSource)
	if err != nil {
		notFoundOrInternal(w, r, err, "failed to load config")
		return
	}
	render.Respond(w, r, c)
}

func (h *repairHandler) updateConfig(w http.ResponseWriter, r *http.Request) {
	cr, err := parseConfigRequest(r)
	if err != nil {
		render.Respond(w, r, err)
		return
	}

	if err := h.svc.PutConfig(r.Context(), cr.ConfigSource, cr.Config); err != nil {
		render.Respond(w, r, httpErrInternal(r, err, "failed to update config"))
		return
	}
}

func (h *repairHandler) deleteConfig(w http.ResponseWriter, r *http.Request) {
	cr, err := parseConfigRequest(r)
	if err != nil {
		render.Respond(w, r, err)
		return
	}

	if err := h.svc.DeleteConfig(r.Context(), cr.ConfigSource); err != nil {
		render.Respond(w, r, httpErrInternal(r, err, "failed to delete config"))
	}
}

//
// task
//

type repairProgress struct {
	PercentComplete int `json:"percent_complete"`
	Total           int `json:"total"`
	Success         int `json:"success"`
	Error           int `json:"error"`
}

type repairHostProgress struct {
	repairProgress
	Shards map[int]*repairProgress `json:"shards"`
}

type repairProgressResponse struct {
	Keyspace string        `json:"keyspace"`
	Tables   []string      `json:"tables"`
	Status   repair.Status `json:"status"`
	repairProgress

	Hosts map[string]*repairHostProgress `json:"hosts"`
}

func (h *repairHandler) repairProgress(w http.ResponseWriter, r *http.Request) {
	unitID, err := reqUnitIDQuery(r)
	if err != nil {
		render.Respond(w, r, httpErrBadRequest(r, err))
		return
	}

	u, err := h.svc.GetUnit(r.Context(), clusterIDFromCtx(r.Context()), unitID)
	if err != nil {
		notFoundOrInternal(w, r, err, "failed to load unit")
		return
	}

	var taskID uuid.UUID
	if err := taskID.UnmarshalText([]byte(chi.URLParam(r, "task_id"))); err != nil {
		render.Respond(w, r, httpErrBadRequest(r, err))
		return
	}

	taskRun, err := h.svc.GetRun(r.Context(), u, taskID)
	if err != nil {
		notFoundOrInternal(w, r, err, "failed to load task")
		return
	}

	resp, err := h.progressResponse(r, u, taskRun)
	if err != nil {
		render.Respond(w, r, err)
		return
	}
	render.Respond(w, r, resp)
}

func (h *repairHandler) progressResponse(r *http.Request, u *repair.Unit, t *repair.Run) (*repairProgressResponse, error) {
	runs, err := h.svc.GetProgress(r.Context(), u, t.ID)
	if err != nil {
		return nil, httpErrInternal(r, err, "failed to load task progress")
	}

	if len(runs) == 0 {
		return nil, httpErrNotFound(r, err)
	}

	resp := &repairProgressResponse{
		Keyspace: t.Keyspace,
		Tables:   t.Tables,
		Status:   t.Status,
		Hosts:    make(map[string]*repairHostProgress),
	}
	for _, r := range runs {
		if _, exists := resp.Hosts[r.Host]; !exists {
			resp.Hosts[r.Host] = &repairHostProgress{
				Shards: make(map[int]*repairProgress),
			}
		}

		if r.SegmentCount == 0 {
			resp.Hosts[r.Host].Shards[r.Shard] = &repairProgress{}
			continue
		}
		resp.Hosts[r.Host].Shards[r.Shard] = &repairProgress{
			PercentComplete: int(100 * float64(r.SegmentSuccess+r.SegmentError) / float64(r.SegmentCount)),
			Total:           r.SegmentCount,
			Success:         r.SegmentSuccess,
			Error:           r.SegmentError,
		}
	}

	var totalSum float64
	for _, hostProgress := range resp.Hosts {
		var sum float64
		for _, s := range hostProgress.Shards {
			sum += float64(s.PercentComplete)
			hostProgress.Total += s.Total
			hostProgress.Success += s.Success
			hostProgress.Error += s.Error
		}
		sum /= float64(len(hostProgress.Shards))
		totalSum += sum
		hostProgress.PercentComplete = int(sum)
		resp.Total += hostProgress.Total
		resp.Success += hostProgress.Success
		resp.Error += hostProgress.Error
	}
	resp.PercentComplete = int(totalSum / float64(len(resp.Hosts)))
	return resp, nil
}
