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

// StorageServiceLoadGetReader is a Reader for the StorageServiceLoadGet structure.
type StorageServiceLoadGetReader struct {
	formats strfmt.Registry
}

// ReadResponse reads a server response into the received o.
func (o *StorageServiceLoadGetReader) ReadResponse(response runtime.ClientResponse, consumer runtime.Consumer) (interface{}, error) {
	switch response.Code() {
	case 200:
		result := NewStorageServiceLoadGetOK()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return result, nil
	default:
		result := NewStorageServiceLoadGetDefault(response.Code())
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		if response.Code()/100 == 2 {
			return result, nil
		}
		return nil, result
	}
}

// NewStorageServiceLoadGetOK creates a StorageServiceLoadGetOK with default headers values
func NewStorageServiceLoadGetOK() *StorageServiceLoadGetOK {
	return &StorageServiceLoadGetOK{}
}

/*StorageServiceLoadGetOK handles this case with default header values.

StorageServiceLoadGetOK storage service load get o k
*/
type StorageServiceLoadGetOK struct {
	Payload interface{}
}

func (o *StorageServiceLoadGetOK) GetPayload() interface{} {
	return o.Payload
}

func (o *StorageServiceLoadGetOK) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	// response payload
	if err := consumer.Consume(response.Body(), &o.Payload); err != nil && err != io.EOF {
		return err
	}

	return nil
}

// NewStorageServiceLoadGetDefault creates a StorageServiceLoadGetDefault with default headers values
func NewStorageServiceLoadGetDefault(code int) *StorageServiceLoadGetDefault {
	return &StorageServiceLoadGetDefault{
		_statusCode: code,
	}
}

/*StorageServiceLoadGetDefault handles this case with default header values.

internal server error
*/
type StorageServiceLoadGetDefault struct {
	_statusCode int

	Payload *models.ErrorModel
}

// Code gets the status code for the storage service load get default response
func (o *StorageServiceLoadGetDefault) Code() int {
	return o._statusCode
}

func (o *StorageServiceLoadGetDefault) GetPayload() *models.ErrorModel {
	return o.Payload
}

func (o *StorageServiceLoadGetDefault) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	o.Payload = new(models.ErrorModel)

	// response payload
	if err := consumer.Consume(response.Body(), o.Payload); err != nil && err != io.EOF {
		return err
	}

	return nil
}

func (o *StorageServiceLoadGetDefault) Error() string {
	return fmt.Sprintf("agent [HTTP %d] %s", o._statusCode, strings.TrimRight(o.Payload.Message, "."))
}
