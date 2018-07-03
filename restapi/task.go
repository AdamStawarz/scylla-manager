// Copyright (C) 2017 ScyllaDB

package restapi

import (
	"context"
	"math"
	"net/http"
	"net/url"
	"path"
	"sort"
	"strconv"
	"time"

	"github.com/go-chi/chi"
	"github.com/go-chi/render"
	"github.com/pkg/errors"
	"github.com/scylladb/mermaid/internal/timeutc"
	"github.com/scylladb/mermaid/repair"
	"github.com/scylladb/mermaid/sched"
	"github.com/scylladb/mermaid/sched/runner"
	"github.com/scylladb/mermaid/uuid"
)

//go:generate mockgen -destination sched_mock.go -mock_names SchedService=mockSchedService -package restapi github.com/scylladb/mermaid/restapi SchedService

// SchedService is the scheduler service interface required by the scheduler REST API handlers.
type SchedService interface {
	GetTask(ctx context.Context, clusterID uuid.UUID, tp sched.TaskType, idOrName string) (*sched.Task, error)
	PutTask(ctx context.Context, t *sched.Task) error
	DeleteTask(ctx context.Context, t *sched.Task) error
	ListTasks(ctx context.Context, clusterID uuid.UUID, tp sched.TaskType) ([]*sched.Task, error)
	StartTask(ctx context.Context, t *sched.Task, opts runner.Opts) error
	StopTask(ctx context.Context, t *sched.Task) error
	GetLastRun(ctx context.Context, t *sched.Task, n int) ([]*sched.Run, error)
}

// RepairService is the repair service interface required by the repair REST API handlers.
type RepairService interface {
	GetRun(ctx context.Context, clusterID, taskID, runID uuid.UUID) (*repair.Run, error)
	GetProgress(ctx context.Context, clusterID, taskID, runID uuid.UUID) (repair.Progress, error)
	GetTarget(ctx context.Context, clusterID uuid.UUID, p runner.Properties) (repair.Target, error)
}

type taskHandler struct {
	schedSvc  SchedService
	repairSvc RepairService
}

func newTaskHandler(schedSvc SchedService, repairSvc RepairService) *chi.Mux {
	m := chi.NewMux()
	h := &taskHandler{
		schedSvc:  schedSvc,
		repairSvc: repairSvc,
	}

	m.Route("/tasks", func(r chi.Router) {
		r.Get("/", h.listTasks)
		r.Post("/", h.createTask)
	})

	m.Route("/task/{task_type}/{task_id}", func(r chi.Router) {
		r.Use(h.taskCtx)
		r.Get("/", h.loadTask)
		r.Put("/", h.updateTask)
		r.Delete("/", h.deleteTask)
		r.Put("/start", h.startTask)
		r.Put("/stop", h.stopTask)
		r.Get("/history", h.taskHistory)
		r.Get("/{run_id}/progress", h.taskProgress)
	})

	return m
}

func (h *taskHandler) taskCtx(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rctx := chi.RouteContext(r.Context())
		var taskType sched.TaskType
		if t := rctx.URLParam("task_type"); t == "" {
			respondBadRequest(w, r, errors.New("missing task type"))
			return
		} else if err := taskType.UnmarshalText([]byte(t)); err != nil {
			respondBadRequest(w, r, err)
			return
		}
		taskID := rctx.URLParam("task_id")
		if taskID == "" {
			respondBadRequest(w, r, errors.New("missing task ID"))
			return
		}

		t, err := h.schedSvc.GetTask(r.Context(), mustClusterIDFromCtx(r), taskType, taskID)
		if err != nil {
			respondError(w, r, err, "failed to load task")
			return
		}

		ctx := context.WithValue(r.Context(), ctxTask, t)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

type extendedTask struct {
	*sched.Task
	Status         runner.Status `json:"status,omitempty"`
	Cause          string        `json:"cause,omitempty"`
	StartTime      *time.Time    `json:"start_time,omitempty"`
	EndTime        *time.Time    `json:"end_time,omitempty"`
	NextActivation *time.Time    `json:"next_activation,omitempty"`
}

func (h *taskHandler) listTasks(w http.ResponseWriter, r *http.Request) {
	all := false
	if a := r.FormValue("all"); a != "" {
		var err error
		all, err = strconv.ParseBool(a)
		if err != nil {
			respondBadRequest(w, r, err)
			return
		}
	}
	var taskType sched.TaskType
	if t := r.FormValue("type"); t != "" {
		if err := taskType.UnmarshalText([]byte(t)); err != nil {
			respondBadRequest(w, r, err)
			return
		}
	}

	var status runner.Status
	if s := r.FormValue("status"); s != "" {
		if err := status.UnmarshalText([]byte(s)); err != nil {
			respondBadRequest(w, r, err)
			return
		}
	}

	tasks, err := h.schedSvc.ListTasks(r.Context(), mustClusterIDFromCtx(r), taskType)
	if err != nil {
		respondError(w, r, err, "failed to list tasks")
		return
	}

	hist := make([]extendedTask, 0, len(tasks))
	for _, t := range tasks {
		if !all && !t.Enabled {
			continue
		}

		e := extendedTask{Task: t}

		runs, err := h.schedSvc.GetLastRun(r.Context(), t, t.Sched.NumRetries)
		if err != nil {
			respondError(w, r, err, "failed to load task run")
			return
		}
		if len(runs) > 0 {
			e.Status = runs[0].Status
			e.Cause = runs[0].Cause
			if tm := runs[0].StartTime; !tm.IsZero() {
				e.StartTime = &tm
			}
			e.EndTime = runs[0].EndTime
		}
		if a := t.Sched.NextActivation(timeutc.Now(), runs); a.After(timeutc.Now()) {
			e.NextActivation = &a
		}

		hist = append(hist, e)
	}

	sort.Slice(hist, func(i, j int) bool {
		l := int64(math.MaxInt64)
		if hist[i].NextActivation != nil {
			l = hist[i].NextActivation.Unix()
		}
		r := int64(math.MaxInt64)
		if hist[j].NextActivation != nil {
			r = hist[j].NextActivation.Unix()
		}
		return l < r
	})

	render.Respond(w, r, hist)
}

func (h *taskHandler) parseTask(r *http.Request) (*sched.Task, error) {
	var t sched.Task
	if err := render.DecodeJSON(r.Body, &t); err != nil {
		return nil, err
	}
	t.ClusterID = mustClusterIDFromCtx(r)
	return &t, nil
}

func (h *taskHandler) createTask(w http.ResponseWriter, r *http.Request) {
	newTask, err := h.parseTask(r)
	if err != nil {
		respondBadRequest(w, r, err)
		return
	}
	if newTask.ID != uuid.Nil {
		respondBadRequest(w, r, errors.Errorf("unexpected ID %q", newTask.ID))
		return
	}

	if _, err := h.repairSvc.GetTarget(r.Context(), newTask.ClusterID, newTask.Properties); err != nil {
		respondError(w, r, err, "failed to create repair target")
		return
	}

	if err := h.schedSvc.PutTask(r.Context(), newTask); err != nil {
		respondError(w, r, err, "failed to create task")
		return
	}

	taskURL := r.URL.ResolveReference(&url.URL{Path: path.Join("task", newTask.Type.String(), newTask.ID.String())})
	w.Header().Set("Location", taskURL.String())
	w.WriteHeader(http.StatusCreated)
}

func (h *taskHandler) loadTask(w http.ResponseWriter, r *http.Request) {
	t := mustTaskFromCtx(r)
	render.Respond(w, r, t)
}

func (h *taskHandler) updateTask(w http.ResponseWriter, r *http.Request) {
	t := mustTaskFromCtx(r)
	newTask, err := h.parseTask(r)
	if err != nil {
		respondBadRequest(w, r, err)
		return
	}
	newTask.ID = t.ID
	newTask.Type = t.Type

	if err := h.schedSvc.PutTask(r.Context(), newTask); err != nil {
		respondError(w, r, err, "failed to update task")
		return
	}
	render.Respond(w, r, newTask)
}

func (h *taskHandler) deleteTask(w http.ResponseWriter, r *http.Request) {
	t := mustTaskFromCtx(r)
	if err := h.schedSvc.DeleteTask(r.Context(), t); err != nil {
		respondError(w, r, err, "failed to delete task")
		return
	}
}

func (h *taskHandler) startTask(w http.ResponseWriter, r *http.Request) {
	t := mustTaskFromCtx(r)

	opts, err := h.optsFromRequest(r)
	if err != nil {
		respondBadRequest(w, r, err)
	}

	if err := h.schedSvc.StartTask(r.Context(), t, opts); err != nil {
		respondError(w, r, err, "failed to start task")
		return
	}
}

func (h *taskHandler) optsFromRequest(r *http.Request) (opts runner.Opts, err error) {
	opts = runner.DefaultOpts
	if cont := r.FormValue("continue"); cont != "" {
		if opts.Continue, err = strconv.ParseBool(cont); err != nil {
			return
		}
	}
	return
}

func (h *taskHandler) stopTask(w http.ResponseWriter, r *http.Request) {
	t := mustTaskFromCtx(r)
	if err := h.schedSvc.StopTask(r.Context(), t); err != nil {
		respondError(w, r, err, "failed to stop task")
		return
	}
}

func (h *taskHandler) taskHistory(w http.ResponseWriter, r *http.Request) {
	t := mustTaskFromCtx(r)

	limit := 10
	if l := r.FormValue("limit"); l != "" {
		var err error
		limit, err = strconv.Atoi(l)
		if err != nil {
			respondBadRequest(w, r, err)
			return
		}
	}

	runs, err := h.schedSvc.GetLastRun(r.Context(), t, limit)
	if err != nil {
		respondError(w, r, err, "failed to load task history")
		return
	}
	if len(runs) == 0 {
		render.Respond(w, r, []string{})
		return
	}
	render.Respond(w, r, runs)
}

func (h *taskHandler) taskProgress(w http.ResponseWriter, r *http.Request) {
	t := mustTaskFromCtx(r)

	runID, err := uuid.Parse(chi.URLParam(r, "run_id"))
	if err != nil {
		respondBadRequest(w, r, err)
		return
	}

	prog, err := h.repairSvc.GetProgress(r.Context(), t.ClusterID, t.ID, runID)
	if err != nil {
		respondError(w, r, err, "failed to load repair run progress")
		return
	}

	render.Respond(w, r, prog)
}
