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

// NewFindConfigInterDcStreamThroughputOutboundMegabitsPerSecParams creates a new FindConfigInterDcStreamThroughputOutboundMegabitsPerSecParams object
// with the default values initialized.
func NewFindConfigInterDcStreamThroughputOutboundMegabitsPerSecParams() *FindConfigInterDcStreamThroughputOutboundMegabitsPerSecParams {

	return &FindConfigInterDcStreamThroughputOutboundMegabitsPerSecParams{

		timeout: cr.DefaultTimeout,
	}
}

// NewFindConfigInterDcStreamThroughputOutboundMegabitsPerSecParamsWithTimeout creates a new FindConfigInterDcStreamThroughputOutboundMegabitsPerSecParams object
// with the default values initialized, and the ability to set a timeout on a request
func NewFindConfigInterDcStreamThroughputOutboundMegabitsPerSecParamsWithTimeout(timeout time.Duration) *FindConfigInterDcStreamThroughputOutboundMegabitsPerSecParams {

	return &FindConfigInterDcStreamThroughputOutboundMegabitsPerSecParams{

		timeout: timeout,
	}
}

// NewFindConfigInterDcStreamThroughputOutboundMegabitsPerSecParamsWithContext creates a new FindConfigInterDcStreamThroughputOutboundMegabitsPerSecParams object
// with the default values initialized, and the ability to set a context for a request
func NewFindConfigInterDcStreamThroughputOutboundMegabitsPerSecParamsWithContext(ctx context.Context) *FindConfigInterDcStreamThroughputOutboundMegabitsPerSecParams {

	return &FindConfigInterDcStreamThroughputOutboundMegabitsPerSecParams{

		Context: ctx,
	}
}

// NewFindConfigInterDcStreamThroughputOutboundMegabitsPerSecParamsWithHTTPClient creates a new FindConfigInterDcStreamThroughputOutboundMegabitsPerSecParams object
// with the default values initialized, and the ability to set a custom HTTPClient for a request
func NewFindConfigInterDcStreamThroughputOutboundMegabitsPerSecParamsWithHTTPClient(client *http.Client) *FindConfigInterDcStreamThroughputOutboundMegabitsPerSecParams {

	return &FindConfigInterDcStreamThroughputOutboundMegabitsPerSecParams{
		HTTPClient: client,
	}
}

/*FindConfigInterDcStreamThroughputOutboundMegabitsPerSecParams contains all the parameters to send to the API endpoint
for the find config inter dc stream throughput outbound megabits per sec operation typically these are written to a http.Request
*/
type FindConfigInterDcStreamThroughputOutboundMegabitsPerSecParams struct {
	timeout    time.Duration
	Context    context.Context
	HTTPClient *http.Client
}

// WithTimeout adds the timeout to the find config inter dc stream throughput outbound megabits per sec params
func (o *FindConfigInterDcStreamThroughputOutboundMegabitsPerSecParams) WithTimeout(timeout time.Duration) *FindConfigInterDcStreamThroughputOutboundMegabitsPerSecParams {
	o.SetTimeout(timeout)
	return o
}

// SetTimeout adds the timeout to the find config inter dc stream throughput outbound megabits per sec params
func (o *FindConfigInterDcStreamThroughputOutboundMegabitsPerSecParams) SetTimeout(timeout time.Duration) {
	o.timeout = timeout
}

// WithContext adds the context to the find config inter dc stream throughput outbound megabits per sec params
func (o *FindConfigInterDcStreamThroughputOutboundMegabitsPerSecParams) WithContext(ctx context.Context) *FindConfigInterDcStreamThroughputOutboundMegabitsPerSecParams {
	o.SetContext(ctx)
	return o
}

// SetContext adds the context to the find config inter dc stream throughput outbound megabits per sec params
func (o *FindConfigInterDcStreamThroughputOutboundMegabitsPerSecParams) SetContext(ctx context.Context) {
	o.Context = ctx
}

// WithHTTPClient adds the HTTPClient to the find config inter dc stream throughput outbound megabits per sec params
func (o *FindConfigInterDcStreamThroughputOutboundMegabitsPerSecParams) WithHTTPClient(client *http.Client) *FindConfigInterDcStreamThroughputOutboundMegabitsPerSecParams {
	o.SetHTTPClient(client)
	return o
}

// SetHTTPClient adds the HTTPClient to the find config inter dc stream throughput outbound megabits per sec params
func (o *FindConfigInterDcStreamThroughputOutboundMegabitsPerSecParams) SetHTTPClient(client *http.Client) {
	o.HTTPClient = client
}

// WriteToRequest writes these params to a swagger request
func (o *FindConfigInterDcStreamThroughputOutboundMegabitsPerSecParams) WriteToRequest(r runtime.ClientRequest, reg strfmt.Registry) error {

	if err := r.SetTimeout(o.timeout); err != nil {
		return err
	}
	var res []error

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}