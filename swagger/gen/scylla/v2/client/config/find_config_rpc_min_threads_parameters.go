// Code generated by go-swagger; DO NOT EDIT.

package config

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the swagger generate command

import (
	"context"
	"net/http"
	"time"

	"github.com/go-openapi/errors"
	"github.com/go-openapi/runtime"
	cr "github.com/go-openapi/runtime/client"
	"github.com/go-openapi/strfmt"
)

// NewFindConfigRPCMinThreadsParams creates a new FindConfigRPCMinThreadsParams object
// with the default values initialized.
func NewFindConfigRPCMinThreadsParams() *FindConfigRPCMinThreadsParams {

	return &FindConfigRPCMinThreadsParams{

		timeout: cr.DefaultTimeout,
	}
}

// NewFindConfigRPCMinThreadsParamsWithTimeout creates a new FindConfigRPCMinThreadsParams object
// with the default values initialized, and the ability to set a timeout on a request
func NewFindConfigRPCMinThreadsParamsWithTimeout(timeout time.Duration) *FindConfigRPCMinThreadsParams {

	return &FindConfigRPCMinThreadsParams{

		timeout: timeout,
	}
}

// NewFindConfigRPCMinThreadsParamsWithContext creates a new FindConfigRPCMinThreadsParams object
// with the default values initialized, and the ability to set a context for a request
func NewFindConfigRPCMinThreadsParamsWithContext(ctx context.Context) *FindConfigRPCMinThreadsParams {

	return &FindConfigRPCMinThreadsParams{

		Context: ctx,
	}
}

// NewFindConfigRPCMinThreadsParamsWithHTTPClient creates a new FindConfigRPCMinThreadsParams object
// with the default values initialized, and the ability to set a custom HTTPClient for a request
func NewFindConfigRPCMinThreadsParamsWithHTTPClient(client *http.Client) *FindConfigRPCMinThreadsParams {

	return &FindConfigRPCMinThreadsParams{
		HTTPClient: client,
	}
}

/*FindConfigRPCMinThreadsParams contains all the parameters to send to the API endpoint
for the find config rpc min threads operation typically these are written to a http.Request
*/
type FindConfigRPCMinThreadsParams struct {
	timeout    time.Duration
	Context    context.Context
	HTTPClient *http.Client
}

// WithTimeout adds the timeout to the find config rpc min threads params
func (o *FindConfigRPCMinThreadsParams) WithTimeout(timeout time.Duration) *FindConfigRPCMinThreadsParams {
	o.SetTimeout(timeout)
	return o
}

// SetTimeout adds the timeout to the find config rpc min threads params
func (o *FindConfigRPCMinThreadsParams) SetTimeout(timeout time.Duration) {
	o.timeout = timeout
}

// WithContext adds the context to the find config rpc min threads params
func (o *FindConfigRPCMinThreadsParams) WithContext(ctx context.Context) *FindConfigRPCMinThreadsParams {
	o.SetContext(ctx)
	return o
}

// SetContext adds the context to the find config rpc min threads params
func (o *FindConfigRPCMinThreadsParams) SetContext(ctx context.Context) {
	o.Context = ctx
}

// WithHTTPClient adds the HTTPClient to the find config rpc min threads params
func (o *FindConfigRPCMinThreadsParams) WithHTTPClient(client *http.Client) *FindConfigRPCMinThreadsParams {
	o.SetHTTPClient(client)
	return o
}

// SetHTTPClient adds the HTTPClient to the find config rpc min threads params
func (o *FindConfigRPCMinThreadsParams) SetHTTPClient(client *http.Client) {
	o.HTTPClient = client
}

// WriteToRequest writes these params to a swagger request
func (o *FindConfigRPCMinThreadsParams) WriteToRequest(r runtime.ClientRequest, reg strfmt.Registry) error {

	if err := r.SetTimeout(o.timeout); err != nil {
		return err
	}
	var res []error

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}