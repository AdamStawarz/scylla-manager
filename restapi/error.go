// Copyright (C) 2017 ScyllaDB

package restapi

import (
	"net/http"

	"github.com/go-chi/render"
	"github.com/pkg/errors"
	"github.com/scylladb/go-log"
	"github.com/scylladb/mermaid"
)

// httpError is a wrapper holding an error, HTTP status code and a user-facing
// message.
type httpError struct {
	Err        error  `json:"-"`
	StatusCode int    `json:"-"`
	Message    string `json:"message"`
	TraceID    string `json:"trace_id"`
}

func (e *httpError) Error() string {
	return e.Err.Error()
}

func respondBadRequest(w http.ResponseWriter, r *http.Request, err error) {
	render.Respond(w, r, &httpError{
		Err:        err,
		StatusCode: http.StatusBadRequest,
		Message:    errors.Wrap(err, "malformed request").Error(),
		TraceID:    log.TraceID(r.Context()),
	})
}

func respondError(w http.ResponseWriter, r *http.Request, err error, msg string) {
	cause := errors.Cause(err)

	switch {
	case cause == mermaid.ErrNotFound:
		render.Respond(w, r, &httpError{
			Err:        cause,
			StatusCode: http.StatusNotFound,
			Message:    "specified resource not found",
			TraceID:    log.TraceID(r.Context()),
		})
	case mermaid.IsErrValidate(cause):
		render.Respond(w, r, &httpError{
			Err:        cause,
			StatusCode: http.StatusBadRequest,
			Message:    errors.Wrap(cause, msg).Error(),
			TraceID:    log.TraceID(r.Context()),
		})
	default:
		render.Respond(w, r, &httpError{
			Err:        err,
			StatusCode: http.StatusInternalServerError,
			Message:    msg,
			TraceID:    log.TraceID(r.Context()),
		})
	}
}
