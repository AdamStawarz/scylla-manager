// Code generated by go-swagger; DO NOT EDIT.

package operations

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the swagger generate command

import (
	"net/http"
	"time"

	"golang.org/x/net/context"

	"github.com/go-openapi/errors"
	"github.com/go-openapi/runtime"
	cr "github.com/go-openapi/runtime/client"

	strfmt "github.com/go-openapi/strfmt"
)

// NewCompactionManagerCompactionsGetParams creates a new CompactionManagerCompactionsGetParams object
// with the default values initialized.
func NewCompactionManagerCompactionsGetParams() *CompactionManagerCompactionsGetParams {

	return &CompactionManagerCompactionsGetParams{

		timeout: cr.DefaultTimeout,
	}
}

// NewCompactionManagerCompactionsGetParamsWithTimeout creates a new CompactionManagerCompactionsGetParams object
// with the default values initialized, and the ability to set a timeout on a request
func NewCompactionManagerCompactionsGetParamsWithTimeout(timeout time.Duration) *CompactionManagerCompactionsGetParams {

	return &CompactionManagerCompactionsGetParams{

		timeout: timeout,
	}
}

// NewCompactionManagerCompactionsGetParamsWithContext creates a new CompactionManagerCompactionsGetParams object
// with the default values initialized, and the ability to set a context for a request
func NewCompactionManagerCompactionsGetParamsWithContext(ctx context.Context) *CompactionManagerCompactionsGetParams {

	return &CompactionManagerCompactionsGetParams{

		Context: ctx,
	}
}

// NewCompactionManagerCompactionsGetParamsWithHTTPClient creates a new CompactionManagerCompactionsGetParams object
// with the default values initialized, and the ability to set a custom HTTPClient for a request
func NewCompactionManagerCompactionsGetParamsWithHTTPClient(client *http.Client) *CompactionManagerCompactionsGetParams {

	return &CompactionManagerCompactionsGetParams{
		HTTPClient: client,
	}
}

/*CompactionManagerCompactionsGetParams contains all the parameters to send to the API endpoint
for the compaction manager compactions get operation typically these are written to a http.Request
*/
type CompactionManagerCompactionsGetParams struct {
	timeout    time.Duration
	Context    context.Context
	HTTPClient *http.Client
}

// WithTimeout adds the timeout to the compaction manager compactions get params
func (o *CompactionManagerCompactionsGetParams) WithTimeout(timeout time.Duration) *CompactionManagerCompactionsGetParams {
	o.SetTimeout(timeout)
	return o
}

// SetTimeout adds the timeout to the compaction manager compactions get params
func (o *CompactionManagerCompactionsGetParams) SetTimeout(timeout time.Duration) {
	o.timeout = timeout
}

// WithContext adds the context to the compaction manager compactions get params
func (o *CompactionManagerCompactionsGetParams) WithContext(ctx context.Context) *CompactionManagerCompactionsGetParams {
	o.SetContext(ctx)
	return o
}

// SetContext adds the context to the compaction manager compactions get params
func (o *CompactionManagerCompactionsGetParams) SetContext(ctx context.Context) {
	o.Context = ctx
}

// WithHTTPClient adds the HTTPClient to the compaction manager compactions get params
func (o *CompactionManagerCompactionsGetParams) WithHTTPClient(client *http.Client) *CompactionManagerCompactionsGetParams {
	o.SetHTTPClient(client)
	return o
}

// SetHTTPClient adds the HTTPClient to the compaction manager compactions get params
func (o *CompactionManagerCompactionsGetParams) SetHTTPClient(client *http.Client) {
	o.HTTPClient = client
}

// WriteToRequest writes these params to a swagger request
func (o *CompactionManagerCompactionsGetParams) WriteToRequest(r runtime.ClientRequest, reg strfmt.Registry) error {

	if err := r.SetTimeout(o.timeout); err != nil {
		return err
	}
	var res []error

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}