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

// NewStorageServiceGossipingGetParams creates a new StorageServiceGossipingGetParams object
// with the default values initialized.
func NewStorageServiceGossipingGetParams() *StorageServiceGossipingGetParams {

	return &StorageServiceGossipingGetParams{

		timeout: cr.DefaultTimeout,
	}
}

// NewStorageServiceGossipingGetParamsWithTimeout creates a new StorageServiceGossipingGetParams object
// with the default values initialized, and the ability to set a timeout on a request
func NewStorageServiceGossipingGetParamsWithTimeout(timeout time.Duration) *StorageServiceGossipingGetParams {

	return &StorageServiceGossipingGetParams{

		timeout: timeout,
	}
}

// NewStorageServiceGossipingGetParamsWithContext creates a new StorageServiceGossipingGetParams object
// with the default values initialized, and the ability to set a context for a request
func NewStorageServiceGossipingGetParamsWithContext(ctx context.Context) *StorageServiceGossipingGetParams {

	return &StorageServiceGossipingGetParams{

		Context: ctx,
	}
}

// NewStorageServiceGossipingGetParamsWithHTTPClient creates a new StorageServiceGossipingGetParams object
// with the default values initialized, and the ability to set a custom HTTPClient for a request
func NewStorageServiceGossipingGetParamsWithHTTPClient(client *http.Client) *StorageServiceGossipingGetParams {

	return &StorageServiceGossipingGetParams{
		HTTPClient: client,
	}
}

/*StorageServiceGossipingGetParams contains all the parameters to send to the API endpoint
for the storage service gossiping get operation typically these are written to a http.Request
*/
type StorageServiceGossipingGetParams struct {
	timeout    time.Duration
	Context    context.Context
	HTTPClient *http.Client
}

// WithTimeout adds the timeout to the storage service gossiping get params
func (o *StorageServiceGossipingGetParams) WithTimeout(timeout time.Duration) *StorageServiceGossipingGetParams {
	o.SetTimeout(timeout)
	return o
}

// SetTimeout adds the timeout to the storage service gossiping get params
func (o *StorageServiceGossipingGetParams) SetTimeout(timeout time.Duration) {
	o.timeout = timeout
}

// WithContext adds the context to the storage service gossiping get params
func (o *StorageServiceGossipingGetParams) WithContext(ctx context.Context) *StorageServiceGossipingGetParams {
	o.SetContext(ctx)
	return o
}

// SetContext adds the context to the storage service gossiping get params
func (o *StorageServiceGossipingGetParams) SetContext(ctx context.Context) {
	o.Context = ctx
}

// WithHTTPClient adds the HTTPClient to the storage service gossiping get params
func (o *StorageServiceGossipingGetParams) WithHTTPClient(client *http.Client) *StorageServiceGossipingGetParams {
	o.SetHTTPClient(client)
	return o
}

// SetHTTPClient adds the HTTPClient to the storage service gossiping get params
func (o *StorageServiceGossipingGetParams) SetHTTPClient(client *http.Client) {
	o.HTTPClient = client
}

// WriteToRequest writes these params to a swagger request
func (o *StorageServiceGossipingGetParams) WriteToRequest(r runtime.ClientRequest, reg strfmt.Registry) error {

	if err := r.SetTimeout(o.timeout); err != nil {
		return err
	}
	var res []error

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}