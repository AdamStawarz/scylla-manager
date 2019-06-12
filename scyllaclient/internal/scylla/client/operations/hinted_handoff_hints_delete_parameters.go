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

// NewHintedHandoffHintsDeleteParams creates a new HintedHandoffHintsDeleteParams object
// with the default values initialized.
func NewHintedHandoffHintsDeleteParams() *HintedHandoffHintsDeleteParams {
	var ()
	return &HintedHandoffHintsDeleteParams{

		timeout: cr.DefaultTimeout,
	}
}

// NewHintedHandoffHintsDeleteParamsWithTimeout creates a new HintedHandoffHintsDeleteParams object
// with the default values initialized, and the ability to set a timeout on a request
func NewHintedHandoffHintsDeleteParamsWithTimeout(timeout time.Duration) *HintedHandoffHintsDeleteParams {
	var ()
	return &HintedHandoffHintsDeleteParams{

		timeout: timeout,
	}
}

// NewHintedHandoffHintsDeleteParamsWithContext creates a new HintedHandoffHintsDeleteParams object
// with the default values initialized, and the ability to set a context for a request
func NewHintedHandoffHintsDeleteParamsWithContext(ctx context.Context) *HintedHandoffHintsDeleteParams {
	var ()
	return &HintedHandoffHintsDeleteParams{

		Context: ctx,
	}
}

// NewHintedHandoffHintsDeleteParamsWithHTTPClient creates a new HintedHandoffHintsDeleteParams object
// with the default values initialized, and the ability to set a custom HTTPClient for a request
func NewHintedHandoffHintsDeleteParamsWithHTTPClient(client *http.Client) *HintedHandoffHintsDeleteParams {
	var ()
	return &HintedHandoffHintsDeleteParams{
		HTTPClient: client,
	}
}

/*HintedHandoffHintsDeleteParams contains all the parameters to send to the API endpoint
for the hinted handoff hints delete operation typically these are written to a http.Request
*/
type HintedHandoffHintsDeleteParams struct {

	/*Host
	  Optional String rep. of endpoint address to delete hints for

	*/
	Host *string

	timeout    time.Duration
	Context    context.Context
	HTTPClient *http.Client
}

// WithTimeout adds the timeout to the hinted handoff hints delete params
func (o *HintedHandoffHintsDeleteParams) WithTimeout(timeout time.Duration) *HintedHandoffHintsDeleteParams {
	o.SetTimeout(timeout)
	return o
}

// SetTimeout adds the timeout to the hinted handoff hints delete params
func (o *HintedHandoffHintsDeleteParams) SetTimeout(timeout time.Duration) {
	o.timeout = timeout
}

// WithContext adds the context to the hinted handoff hints delete params
func (o *HintedHandoffHintsDeleteParams) WithContext(ctx context.Context) *HintedHandoffHintsDeleteParams {
	o.SetContext(ctx)
	return o
}

// SetContext adds the context to the hinted handoff hints delete params
func (o *HintedHandoffHintsDeleteParams) SetContext(ctx context.Context) {
	o.Context = ctx
}

// WithHTTPClient adds the HTTPClient to the hinted handoff hints delete params
func (o *HintedHandoffHintsDeleteParams) WithHTTPClient(client *http.Client) *HintedHandoffHintsDeleteParams {
	o.SetHTTPClient(client)
	return o
}

// SetHTTPClient adds the HTTPClient to the hinted handoff hints delete params
func (o *HintedHandoffHintsDeleteParams) SetHTTPClient(client *http.Client) {
	o.HTTPClient = client
}

// WithHost adds the host to the hinted handoff hints delete params
func (o *HintedHandoffHintsDeleteParams) WithHost(host *string) *HintedHandoffHintsDeleteParams {
	o.SetHost(host)
	return o
}

// SetHost adds the host to the hinted handoff hints delete params
func (o *HintedHandoffHintsDeleteParams) SetHost(host *string) {
	o.Host = host
}

// WriteToRequest writes these params to a swagger request
func (o *HintedHandoffHintsDeleteParams) WriteToRequest(r runtime.ClientRequest, reg strfmt.Registry) error {

	if err := r.SetTimeout(o.timeout); err != nil {
		return err
	}
	var res []error

	if o.Host != nil {

		// query param host
		var qrHost string
		if o.Host != nil {
			qrHost = *o.Host
		}
		qHost := qrHost
		if qHost != "" {
			if err := r.SetQueryParam("host", qHost); err != nil {
				return err
			}
		}

	}

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}