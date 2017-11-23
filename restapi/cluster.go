// Copyright (C) 2017 ScyllaDB

package restapi

import (
	"context"
	"net/http"
	"net/url"
	"path"

	"github.com/go-chi/chi"
	"github.com/go-chi/render"
	"github.com/pkg/errors"
	"github.com/scylladb/mermaid/cluster"
	"github.com/scylladb/mermaid/uuid"
)

//go:generate mockgen -source cluster.go -destination ../mermaidmock/clusterservice_mock.go -package mermaidmock

// ClusterService is the cluster service interface required by the repair REST
// API handlers.
type ClusterService interface {
	ListClusters(ctx context.Context, f *cluster.Filter) ([]*cluster.Cluster, error)
	GetCluster(ctx context.Context, idOrName string) (*cluster.Cluster, error)
	PutCluster(ctx context.Context, c *cluster.Cluster) error
	DeleteCluster(ctx context.Context, id uuid.UUID) error
}

type clusterHandler struct {
	chi.Router
	svc ClusterService
}

func newClusterHandler(svc ClusterService) http.Handler {
	h := &clusterHandler{
		Router: chi.NewRouter(),
		svc:    svc,
	}

	h.Route("/clusters", func(r chi.Router) {
		r.Get("/", h.listClusters)
		r.Post("/", h.createCluster)
	})
	h.Route("/cluster/{cluster_id}", func(r chi.Router) {
		r.Use(h.clusterCtx)
		r.Get("/", h.loadCluster)
		r.Put("/", h.updateCluster)
		r.Delete("/", h.deleteCluster)
	})
	return h
}

func (h *clusterHandler) listClusters(w http.ResponseWriter, r *http.Request) {
	ids, err := h.svc.ListClusters(r.Context(), &cluster.Filter{})
	if err != nil {
		render.Respond(w, r, httpErrInternal(r, err, "failed to list clusters"))
		return
	}

	if len(ids) == 0 {
		render.Respond(w, r, []struct{}{})
		return
	}
	render.Respond(w, r, ids)
}

func (h *clusterHandler) parseCluster(r *http.Request) (*cluster.Cluster, error) {
	var c cluster.Cluster
	if err := render.DecodeJSON(r.Body, &c); err != nil {
		return nil, err
	}
	return &c, nil
}

func (h *clusterHandler) createCluster(w http.ResponseWriter, r *http.Request) {
	newCluster, err := h.parseCluster(r)
	if err != nil {
		render.Respond(w, r, httpErrBadRequest(r, err))
		return
	}
	if newCluster.ID != uuid.Nil {
		render.Respond(w, r, httpErrBadRequest(r, errors.Errorf("unexpected ID %q", newCluster.ID)))
		return
	}

	if err := h.svc.PutCluster(r.Context(), newCluster); err != nil {
		render.Respond(w, r, httpErrInternal(r, err, "failed to create unit"))
		return
	}

	location := r.URL.ResolveReference(&url.URL{
		Path: path.Join("cluster", newCluster.ID.String()),
	})
	w.Header().Set("Location", location.String())
	w.WriteHeader(http.StatusCreated)
}

func (h *clusterHandler) clusterCtx(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clusterID := chi.URLParam(r, "cluster_id")
		if clusterID == "" {
			render.Respond(w, r, httpErrBadRequest(r, errors.New("missing cluster ID")))
			return
		}

		c, err := h.svc.GetCluster(r.Context(), clusterID)
		if err != nil {
			notFoundOrInternal(w, r, err, "failed to load cluster")
			return
		}

		ctx := context.WithValue(r.Context(), ctxCluster, c)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (h *clusterHandler) mustClusterFromCtx(r *http.Request) *cluster.Cluster {
	c, ok := r.Context().Value(ctxCluster).(*cluster.Cluster)
	if !ok {
		panic("missing repair unit in context")
	}
	return c
}

func (h *clusterHandler) loadCluster(w http.ResponseWriter, r *http.Request) {
	c := h.mustClusterFromCtx(r)
	render.Respond(w, r, c)
}

func (h *clusterHandler) updateCluster(w http.ResponseWriter, r *http.Request) {
	c := h.mustClusterFromCtx(r)

	newCluster, err := h.parseCluster(r)
	if err != nil {
		render.Respond(w, r, httpErrBadRequest(r, err))
		return
	}
	newCluster.ID = c.ID

	if err := h.svc.PutCluster(r.Context(), newCluster); err != nil {
		render.Respond(w, r, httpErrInternal(r, err, "failed to update unit"))
		return
	}
	render.Respond(w, r, newCluster)
}

func (h *clusterHandler) deleteCluster(w http.ResponseWriter, r *http.Request) {
	c := h.mustClusterFromCtx(r)

	if err := h.svc.DeleteCluster(r.Context(), c.ID); err != nil {
		render.Respond(w, r, httpErrInternal(r, err, "failed to delete unit"))
		return
	}
}
