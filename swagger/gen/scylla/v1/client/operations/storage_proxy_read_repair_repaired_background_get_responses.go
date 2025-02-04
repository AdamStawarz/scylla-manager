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

// StorageProxyReadRepairRepairedBackgroundGetReader is a Reader for the StorageProxyReadRepairRepairedBackgroundGet structure.
type StorageProxyReadRepairRepairedBackgroundGetReader struct {
	formats strfmt.Registry
}

// ReadResponse reads a server response into the received o.
func (o *StorageProxyReadRepairRepairedBackgroundGetReader) ReadResponse(response runtime.ClientResponse, consumer runtime.Consumer) (interface{}, error) {
	switch response.Code() {
	case 200:
		result := NewStorageProxyReadRepairRepairedBackgroundGetOK()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return result, nil
	default:
		result := NewStorageProxyReadRepairRepairedBackgroundGetDefault(response.Code())
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		if response.Code()/100 == 2 {
			return result, nil
		}
		return nil, result
	}
}

// NewStorageProxyReadRepairRepairedBackgroundGetOK creates a StorageProxyReadRepairRepairedBackgroundGetOK with default headers values
func NewStorageProxyReadRepairRepairedBackgroundGetOK() *StorageProxyReadRepairRepairedBackgroundGetOK {
	return &StorageProxyReadRepairRepairedBackgroundGetOK{}
}

/*StorageProxyReadRepairRepairedBackgroundGetOK handles this case with default header values.

StorageProxyReadRepairRepairedBackgroundGetOK storage proxy read repair repaired background get o k
*/
type StorageProxyReadRepairRepairedBackgroundGetOK struct {
	Payload interface{}
}

func (o *StorageProxyReadRepairRepairedBackgroundGetOK) GetPayload() interface{} {
	return o.Payload
}

func (o *StorageProxyReadRepairRepairedBackgroundGetOK) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	// response payload
	if err := consumer.Consume(response.Body(), &o.Payload); err != nil && err != io.EOF {
		return err
	}

	return nil
}

// NewStorageProxyReadRepairRepairedBackgroundGetDefault creates a StorageProxyReadRepairRepairedBackgroundGetDefault with default headers values
func NewStorageProxyReadRepairRepairedBackgroundGetDefault(code int) *StorageProxyReadRepairRepairedBackgroundGetDefault {
	return &StorageProxyReadRepairRepairedBackgroundGetDefault{
		_statusCode: code,
	}
}

/*StorageProxyReadRepairRepairedBackgroundGetDefault handles this case with default header values.

internal server error
*/
type StorageProxyReadRepairRepairedBackgroundGetDefault struct {
	_statusCode int

	Payload *models.ErrorModel
}

// Code gets the status code for the storage proxy read repair repaired background get default response
func (o *StorageProxyReadRepairRepairedBackgroundGetDefault) Code() int {
	return o._statusCode
}

func (o *StorageProxyReadRepairRepairedBackgroundGetDefault) GetPayload() *models.ErrorModel {
	return o.Payload
}

func (o *StorageProxyReadRepairRepairedBackgroundGetDefault) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	o.Payload = new(models.ErrorModel)

	// response payload
	if err := consumer.Consume(response.Body(), o.Payload); err != nil && err != io.EOF {
		return err
	}

	return nil
}

func (o *StorageProxyReadRepairRepairedBackgroundGetDefault) Error() string {
	return fmt.Sprintf("agent [HTTP %d] %s", o._statusCode, strings.TrimRight(o.Payload.Message, "."))
}
