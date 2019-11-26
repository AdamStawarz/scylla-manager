// Code generated by go-swagger; DO NOT EDIT.

package operations

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the swagger generate command

import (
	"context"
	"net/http"
	"time"

	"github.com/go-openapi/errors"
	"github.com/go-openapi/runtime"
	cr "github.com/go-openapi/runtime/client"
	"github.com/go-openapi/swag"

	strfmt "github.com/go-openapi/strfmt"
)

// NewGetClusterClusterIDBackupsFilesParams creates a new GetClusterClusterIDBackupsFilesParams object
// with the default values initialized.
func NewGetClusterClusterIDBackupsFilesParams() *GetClusterClusterIDBackupsFilesParams {
	var ()
	return &GetClusterClusterIDBackupsFilesParams{

		timeout: cr.DefaultTimeout,
	}
}

// NewGetClusterClusterIDBackupsFilesParamsWithTimeout creates a new GetClusterClusterIDBackupsFilesParams object
// with the default values initialized, and the ability to set a timeout on a request
func NewGetClusterClusterIDBackupsFilesParamsWithTimeout(timeout time.Duration) *GetClusterClusterIDBackupsFilesParams {
	var ()
	return &GetClusterClusterIDBackupsFilesParams{

		timeout: timeout,
	}
}

// NewGetClusterClusterIDBackupsFilesParamsWithContext creates a new GetClusterClusterIDBackupsFilesParams object
// with the default values initialized, and the ability to set a context for a request
func NewGetClusterClusterIDBackupsFilesParamsWithContext(ctx context.Context) *GetClusterClusterIDBackupsFilesParams {
	var ()
	return &GetClusterClusterIDBackupsFilesParams{

		Context: ctx,
	}
}

// NewGetClusterClusterIDBackupsFilesParamsWithHTTPClient creates a new GetClusterClusterIDBackupsFilesParams object
// with the default values initialized, and the ability to set a custom HTTPClient for a request
func NewGetClusterClusterIDBackupsFilesParamsWithHTTPClient(client *http.Client) *GetClusterClusterIDBackupsFilesParams {
	var ()
	return &GetClusterClusterIDBackupsFilesParams{
		HTTPClient: client,
	}
}

/*GetClusterClusterIDBackupsFilesParams contains all the parameters to send to the API endpoint
for the get cluster cluster ID backups files operation typically these are written to a http.Request
*/
type GetClusterClusterIDBackupsFilesParams struct {

	/*ClusterID*/
	ClusterID string
	/*ClusterID*/
	QueryClusterID *string
	/*Host*/
	Host string
	/*Keyspace*/
	Keyspace []string
	/*Locations*/
	Locations []string
	/*SnapshotTag*/
	SnapshotTag string

	timeout    time.Duration
	Context    context.Context
	HTTPClient *http.Client
}

// WithTimeout adds the timeout to the get cluster cluster ID backups files params
func (o *GetClusterClusterIDBackupsFilesParams) WithTimeout(timeout time.Duration) *GetClusterClusterIDBackupsFilesParams {
	o.SetTimeout(timeout)
	return o
}

// SetTimeout adds the timeout to the get cluster cluster ID backups files params
func (o *GetClusterClusterIDBackupsFilesParams) SetTimeout(timeout time.Duration) {
	o.timeout = timeout
}

// WithContext adds the context to the get cluster cluster ID backups files params
func (o *GetClusterClusterIDBackupsFilesParams) WithContext(ctx context.Context) *GetClusterClusterIDBackupsFilesParams {
	o.SetContext(ctx)
	return o
}

// SetContext adds the context to the get cluster cluster ID backups files params
func (o *GetClusterClusterIDBackupsFilesParams) SetContext(ctx context.Context) {
	o.Context = ctx
}

// WithHTTPClient adds the HTTPClient to the get cluster cluster ID backups files params
func (o *GetClusterClusterIDBackupsFilesParams) WithHTTPClient(client *http.Client) *GetClusterClusterIDBackupsFilesParams {
	o.SetHTTPClient(client)
	return o
}

// SetHTTPClient adds the HTTPClient to the get cluster cluster ID backups files params
func (o *GetClusterClusterIDBackupsFilesParams) SetHTTPClient(client *http.Client) {
	o.HTTPClient = client
}

// WithClusterID adds the clusterID to the get cluster cluster ID backups files params
func (o *GetClusterClusterIDBackupsFilesParams) WithClusterID(clusterID string) *GetClusterClusterIDBackupsFilesParams {
	o.SetClusterID(clusterID)
	return o
}

// SetClusterID adds the clusterId to the get cluster cluster ID backups files params
func (o *GetClusterClusterIDBackupsFilesParams) SetClusterID(clusterID string) {
	o.ClusterID = clusterID
}

// WithQueryClusterID adds the clusterID to the get cluster cluster ID backups files params
func (o *GetClusterClusterIDBackupsFilesParams) WithQueryClusterID(clusterID *string) *GetClusterClusterIDBackupsFilesParams {
	o.SetQueryClusterID(clusterID)
	return o
}

// SetQueryClusterID adds the clusterId to the get cluster cluster ID backups files params
func (o *GetClusterClusterIDBackupsFilesParams) SetQueryClusterID(clusterID *string) {
	o.QueryClusterID = clusterID
}

// WithHost adds the host to the get cluster cluster ID backups files params
func (o *GetClusterClusterIDBackupsFilesParams) WithHost(host string) *GetClusterClusterIDBackupsFilesParams {
	o.SetHost(host)
	return o
}

// SetHost adds the host to the get cluster cluster ID backups files params
func (o *GetClusterClusterIDBackupsFilesParams) SetHost(host string) {
	o.Host = host
}

// WithKeyspace adds the keyspace to the get cluster cluster ID backups files params
func (o *GetClusterClusterIDBackupsFilesParams) WithKeyspace(keyspace []string) *GetClusterClusterIDBackupsFilesParams {
	o.SetKeyspace(keyspace)
	return o
}

// SetKeyspace adds the keyspace to the get cluster cluster ID backups files params
func (o *GetClusterClusterIDBackupsFilesParams) SetKeyspace(keyspace []string) {
	o.Keyspace = keyspace
}

// WithLocations adds the locations to the get cluster cluster ID backups files params
func (o *GetClusterClusterIDBackupsFilesParams) WithLocations(locations []string) *GetClusterClusterIDBackupsFilesParams {
	o.SetLocations(locations)
	return o
}

// SetLocations adds the locations to the get cluster cluster ID backups files params
func (o *GetClusterClusterIDBackupsFilesParams) SetLocations(locations []string) {
	o.Locations = locations
}

// WithSnapshotTag adds the snapshotTag to the get cluster cluster ID backups files params
func (o *GetClusterClusterIDBackupsFilesParams) WithSnapshotTag(snapshotTag string) *GetClusterClusterIDBackupsFilesParams {
	o.SetSnapshotTag(snapshotTag)
	return o
}

// SetSnapshotTag adds the snapshotTag to the get cluster cluster ID backups files params
func (o *GetClusterClusterIDBackupsFilesParams) SetSnapshotTag(snapshotTag string) {
	o.SnapshotTag = snapshotTag
}

// WriteToRequest writes these params to a swagger request
func (o *GetClusterClusterIDBackupsFilesParams) WriteToRequest(r runtime.ClientRequest, reg strfmt.Registry) error {

	if err := r.SetTimeout(o.timeout); err != nil {
		return err
	}
	var res []error

	// path param cluster_id
	if err := r.SetPathParam("cluster_id", o.ClusterID); err != nil {
		return err
	}

	if o.QueryClusterID != nil {

		// query param cluster_id
		var qrClusterID string
		if o.QueryClusterID != nil {
			qrClusterID = *o.QueryClusterID
		}
		qClusterID := qrClusterID
		if qClusterID != "" {
			if err := r.SetQueryParam("cluster_id", qClusterID); err != nil {
				return err
			}
		}

	}

	// query param host
	qrHost := o.Host
	qHost := qrHost
	if qHost != "" {
		if err := r.SetQueryParam("host", qHost); err != nil {
			return err
		}
	}

	valuesKeyspace := o.Keyspace

	joinedKeyspace := swag.JoinByFormat(valuesKeyspace, "")
	// query array param keyspace
	if err := r.SetQueryParam("keyspace", joinedKeyspace...); err != nil {
		return err
	}

	valuesLocations := o.Locations

	joinedLocations := swag.JoinByFormat(valuesLocations, "")
	// query array param locations
	if err := r.SetQueryParam("locations", joinedLocations...); err != nil {
		return err
	}

	// query param snapshot_tag
	qrSnapshotTag := o.SnapshotTag
	qSnapshotTag := qrSnapshotTag
	if qSnapshotTag != "" {
		if err := r.SetQueryParam("snapshot_tag", qSnapshotTag); err != nil {
			return err
		}
	}

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}
