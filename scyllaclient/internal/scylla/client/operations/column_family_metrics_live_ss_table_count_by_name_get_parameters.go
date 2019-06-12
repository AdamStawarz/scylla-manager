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

// NewColumnFamilyMetricsLiveSsTableCountByNameGetParams creates a new ColumnFamilyMetricsLiveSsTableCountByNameGetParams object
// with the default values initialized.
func NewColumnFamilyMetricsLiveSsTableCountByNameGetParams() *ColumnFamilyMetricsLiveSsTableCountByNameGetParams {
	var ()
	return &ColumnFamilyMetricsLiveSsTableCountByNameGetParams{

		timeout: cr.DefaultTimeout,
	}
}

// NewColumnFamilyMetricsLiveSsTableCountByNameGetParamsWithTimeout creates a new ColumnFamilyMetricsLiveSsTableCountByNameGetParams object
// with the default values initialized, and the ability to set a timeout on a request
func NewColumnFamilyMetricsLiveSsTableCountByNameGetParamsWithTimeout(timeout time.Duration) *ColumnFamilyMetricsLiveSsTableCountByNameGetParams {
	var ()
	return &ColumnFamilyMetricsLiveSsTableCountByNameGetParams{

		timeout: timeout,
	}
}

// NewColumnFamilyMetricsLiveSsTableCountByNameGetParamsWithContext creates a new ColumnFamilyMetricsLiveSsTableCountByNameGetParams object
// with the default values initialized, and the ability to set a context for a request
func NewColumnFamilyMetricsLiveSsTableCountByNameGetParamsWithContext(ctx context.Context) *ColumnFamilyMetricsLiveSsTableCountByNameGetParams {
	var ()
	return &ColumnFamilyMetricsLiveSsTableCountByNameGetParams{

		Context: ctx,
	}
}

// NewColumnFamilyMetricsLiveSsTableCountByNameGetParamsWithHTTPClient creates a new ColumnFamilyMetricsLiveSsTableCountByNameGetParams object
// with the default values initialized, and the ability to set a custom HTTPClient for a request
func NewColumnFamilyMetricsLiveSsTableCountByNameGetParamsWithHTTPClient(client *http.Client) *ColumnFamilyMetricsLiveSsTableCountByNameGetParams {
	var ()
	return &ColumnFamilyMetricsLiveSsTableCountByNameGetParams{
		HTTPClient: client,
	}
}

/*ColumnFamilyMetricsLiveSsTableCountByNameGetParams contains all the parameters to send to the API endpoint
for the column family metrics live ss table count by name get operation typically these are written to a http.Request
*/
type ColumnFamilyMetricsLiveSsTableCountByNameGetParams struct {

	/*Name
	  The column family name in keyspace:name format

	*/
	Name string

	timeout    time.Duration
	Context    context.Context
	HTTPClient *http.Client
}

// WithTimeout adds the timeout to the column family metrics live ss table count by name get params
func (o *ColumnFamilyMetricsLiveSsTableCountByNameGetParams) WithTimeout(timeout time.Duration) *ColumnFamilyMetricsLiveSsTableCountByNameGetParams {
	o.SetTimeout(timeout)
	return o
}

// SetTimeout adds the timeout to the column family metrics live ss table count by name get params
func (o *ColumnFamilyMetricsLiveSsTableCountByNameGetParams) SetTimeout(timeout time.Duration) {
	o.timeout = timeout
}

// WithContext adds the context to the column family metrics live ss table count by name get params
func (o *ColumnFamilyMetricsLiveSsTableCountByNameGetParams) WithContext(ctx context.Context) *ColumnFamilyMetricsLiveSsTableCountByNameGetParams {
	o.SetContext(ctx)
	return o
}

// SetContext adds the context to the column family metrics live ss table count by name get params
func (o *ColumnFamilyMetricsLiveSsTableCountByNameGetParams) SetContext(ctx context.Context) {
	o.Context = ctx
}

// WithHTTPClient adds the HTTPClient to the column family metrics live ss table count by name get params
func (o *ColumnFamilyMetricsLiveSsTableCountByNameGetParams) WithHTTPClient(client *http.Client) *ColumnFamilyMetricsLiveSsTableCountByNameGetParams {
	o.SetHTTPClient(client)
	return o
}

// SetHTTPClient adds the HTTPClient to the column family metrics live ss table count by name get params
func (o *ColumnFamilyMetricsLiveSsTableCountByNameGetParams) SetHTTPClient(client *http.Client) {
	o.HTTPClient = client
}

// WithName adds the name to the column family metrics live ss table count by name get params
func (o *ColumnFamilyMetricsLiveSsTableCountByNameGetParams) WithName(name string) *ColumnFamilyMetricsLiveSsTableCountByNameGetParams {
	o.SetName(name)
	return o
}

// SetName adds the name to the column family metrics live ss table count by name get params
func (o *ColumnFamilyMetricsLiveSsTableCountByNameGetParams) SetName(name string) {
	o.Name = name
}

// WriteToRequest writes these params to a swagger request
func (o *ColumnFamilyMetricsLiveSsTableCountByNameGetParams) WriteToRequest(r runtime.ClientRequest, reg strfmt.Registry) error {

	if err := r.SetTimeout(o.timeout); err != nil {
		return err
	}
	var res []error

	// path param name
	if err := r.SetPathParam("name", o.Name); err != nil {
		return err
	}

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}