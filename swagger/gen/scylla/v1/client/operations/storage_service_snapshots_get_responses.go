// Code generated by go-swagger; DO NOT EDIT.

package operations

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the swagger generate command

import (
	"fmt"
	"io"
	"strings"

	"github.com/go-openapi/runtime"
	"github.com/go-openapi/strfmt"

	"github.com/scylladb/scylla-manager/swagger/gen/scylla/v1/models"
)

// StorageServiceSnapshotsGetReader is a Reader for the StorageServiceSnapshotsGet structure.
type StorageServiceSnapshotsGetReader struct {
	formats strfmt.Registry
}

// ReadResponse reads a server response into the received o.
func (o *StorageServiceSnapshotsGetReader) ReadResponse(response runtime.ClientResponse, consumer runtime.Consumer) (interface{}, error) {
	switch response.Code() {
	case 200:
		result := NewStorageServiceSnapshotsGetOK()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return result, nil
	default:
		result := NewStorageServiceSnapshotsGetDefault(response.Code())
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		if response.Code()/100 == 2 {
			return result, nil
		}
		return nil, result
	}
}

// NewStorageServiceSnapshotsGetOK creates a StorageServiceSnapshotsGetOK with default headers values
func NewStorageServiceSnapshotsGetOK() *StorageServiceSnapshotsGetOK {
	return &StorageServiceSnapshotsGetOK{}
}

/*StorageServiceSnapshotsGetOK handles this case with default header values.

StorageServiceSnapshotsGetOK storage service snapshots get o k
*/
type StorageServiceSnapshotsGetOK struct {
	Payload []*models.Snapshots
}

func (o *StorageServiceSnapshotsGetOK) GetPayload() []*models.Snapshots {
	return o.Payload
}

func (o *StorageServiceSnapshotsGetOK) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	// response payload
	if err := consumer.Consume(response.Body(), &o.Payload); err != nil && err != io.EOF {
		return err
	}

	return nil
}

// NewStorageServiceSnapshotsGetDefault creates a StorageServiceSnapshotsGetDefault with default headers values
func NewStorageServiceSnapshotsGetDefault(code int) *StorageServiceSnapshotsGetDefault {
	return &StorageServiceSnapshotsGetDefault{
		_statusCode: code,
	}
}

/*StorageServiceSnapshotsGetDefault handles this case with default header values.

internal server error
*/
type StorageServiceSnapshotsGetDefault struct {
	_statusCode int

	Payload *models.ErrorModel
}

// Code gets the status code for the storage service snapshots get default response
func (o *StorageServiceSnapshotsGetDefault) Code() int {
	return o._statusCode
}

func (o *StorageServiceSnapshotsGetDefault) GetPayload() *models.ErrorModel {
	return o.Payload
}

func (o *StorageServiceSnapshotsGetDefault) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	o.Payload = new(models.ErrorModel)

	// response payload
	if err := consumer.Consume(response.Body(), o.Payload); err != nil && err != io.EOF {
		return err
	}

	return nil
}

func (o *StorageServiceSnapshotsGetDefault) Error() string {
	return fmt.Sprintf("agent [HTTP %d] %s", o._statusCode, strings.TrimRight(o.Payload.Message, "."))
}
