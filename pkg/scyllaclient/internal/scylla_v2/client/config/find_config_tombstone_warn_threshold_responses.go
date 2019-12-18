// Code generated by go-swagger; DO NOT EDIT.

package config

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the swagger generate command

import (
	"fmt"
	"io"
	"strings"

	"github.com/go-openapi/runtime"

	strfmt "github.com/go-openapi/strfmt"

	models "github.com/scylladb/mermaid/pkg/scyllaclient/internal/scylla_v2/models"
)

// FindConfigTombstoneWarnThresholdReader is a Reader for the FindConfigTombstoneWarnThreshold structure.
type FindConfigTombstoneWarnThresholdReader struct {
	formats strfmt.Registry
}

// ReadResponse reads a server response into the received o.
func (o *FindConfigTombstoneWarnThresholdReader) ReadResponse(response runtime.ClientResponse, consumer runtime.Consumer) (interface{}, error) {
	switch response.Code() {
	case 200:
		result := NewFindConfigTombstoneWarnThresholdOK()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return result, nil
	default:
		result := NewFindConfigTombstoneWarnThresholdDefault(response.Code())
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		if response.Code()/100 == 2 {
			return result, nil
		}
		return nil, result
	}
}

// NewFindConfigTombstoneWarnThresholdOK creates a FindConfigTombstoneWarnThresholdOK with default headers values
func NewFindConfigTombstoneWarnThresholdOK() *FindConfigTombstoneWarnThresholdOK {
	return &FindConfigTombstoneWarnThresholdOK{}
}

/*FindConfigTombstoneWarnThresholdOK handles this case with default header values.

Config value
*/
type FindConfigTombstoneWarnThresholdOK struct {
	Payload int64
}

func (o *FindConfigTombstoneWarnThresholdOK) GetPayload() int64 {
	return o.Payload
}

func (o *FindConfigTombstoneWarnThresholdOK) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	// response payload
	if err := consumer.Consume(response.Body(), &o.Payload); err != nil && err != io.EOF {
		return err
	}

	return nil
}

// NewFindConfigTombstoneWarnThresholdDefault creates a FindConfigTombstoneWarnThresholdDefault with default headers values
func NewFindConfigTombstoneWarnThresholdDefault(code int) *FindConfigTombstoneWarnThresholdDefault {
	return &FindConfigTombstoneWarnThresholdDefault{
		_statusCode: code,
	}
}

/*FindConfigTombstoneWarnThresholdDefault handles this case with default header values.

unexpected error
*/
type FindConfigTombstoneWarnThresholdDefault struct {
	_statusCode int

	Payload *models.ErrorModel
}

// Code gets the status code for the find config tombstone warn threshold default response
func (o *FindConfigTombstoneWarnThresholdDefault) Code() int {
	return o._statusCode
}

func (o *FindConfigTombstoneWarnThresholdDefault) GetPayload() *models.ErrorModel {
	return o.Payload
}

func (o *FindConfigTombstoneWarnThresholdDefault) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	o.Payload = new(models.ErrorModel)

	// response payload
	if err := consumer.Consume(response.Body(), o.Payload); err != nil && err != io.EOF {
		return err
	}

	return nil
}

func (o *FindConfigTombstoneWarnThresholdDefault) Error() string {
	return fmt.Sprintf("agent [HTTP %d] %s", o._statusCode, strings.TrimRight(o.Payload.Message, "."))
}