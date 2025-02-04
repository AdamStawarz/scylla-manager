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

// StorageProxyReadRPCTimeoutPostReader is a Reader for the StorageProxyReadRPCTimeoutPost structure.
type StorageProxyReadRPCTimeoutPostReader struct {
	formats strfmt.Registry
}

// ReadResponse reads a server response into the received o.
func (o *StorageProxyReadRPCTimeoutPostReader) ReadResponse(response runtime.ClientResponse, consumer runtime.Consumer) (interface{}, error) {
	switch response.Code() {
	case 200:
		result := NewStorageProxyReadRPCTimeoutPostOK()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return result, nil
	default:
		result := NewStorageProxyReadRPCTimeoutPostDefault(response.Code())
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		if response.Code()/100 == 2 {
			return result, nil
		}
		return nil, result
	}
}

// NewStorageProxyReadRPCTimeoutPostOK creates a StorageProxyReadRPCTimeoutPostOK with default headers values
func NewStorageProxyReadRPCTimeoutPostOK() *StorageProxyReadRPCTimeoutPostOK {
	return &StorageProxyReadRPCTimeoutPostOK{}
}

/*StorageProxyReadRPCTimeoutPostOK handles this case with default header values.

StorageProxyReadRPCTimeoutPostOK storage proxy read Rpc timeout post o k
*/
type StorageProxyReadRPCTimeoutPostOK struct {
}

func (o *StorageProxyReadRPCTimeoutPostOK) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	return nil
}

// NewStorageProxyReadRPCTimeoutPostDefault creates a StorageProxyReadRPCTimeoutPostDefault with default headers values
func NewStorageProxyReadRPCTimeoutPostDefault(code int) *StorageProxyReadRPCTimeoutPostDefault {
	return &StorageProxyReadRPCTimeoutPostDefault{
		_statusCode: code,
	}
}

/*StorageProxyReadRPCTimeoutPostDefault handles this case with default header values.

internal server error
*/
type StorageProxyReadRPCTimeoutPostDefault struct {
	_statusCode int

	Payload *models.ErrorModel
}

// Code gets the status code for the storage proxy read Rpc timeout post default response
func (o *StorageProxyReadRPCTimeoutPostDefault) Code() int {
	return o._statusCode
}

func (o *StorageProxyReadRPCTimeoutPostDefault) GetPayload() *models.ErrorModel {
	return o.Payload
}

func (o *StorageProxyReadRPCTimeoutPostDefault) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	o.Payload = new(models.ErrorModel)

	// response payload
	if err := consumer.Consume(response.Body(), o.Payload); err != nil && err != io.EOF {
		return err
	}

	return nil
}

func (o *StorageProxyReadRPCTimeoutPostDefault) Error() string {
	return fmt.Sprintf("agent [HTTP %d] %s", o._statusCode, strings.TrimRight(o.Payload.Message, "."))
}
