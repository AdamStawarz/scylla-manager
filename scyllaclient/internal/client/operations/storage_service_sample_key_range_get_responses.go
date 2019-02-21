// Code generated by go-swagger; DO NOT EDIT.

package operations

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the swagger generate command

import (
	"fmt"
	"io"

	"github.com/go-openapi/runtime"

	strfmt "github.com/go-openapi/strfmt"
)

// StorageServiceSampleKeyRangeGetReader is a Reader for the StorageServiceSampleKeyRangeGet structure.
type StorageServiceSampleKeyRangeGetReader struct {
	formats strfmt.Registry
}

// ReadResponse reads a server response into the received o.
func (o *StorageServiceSampleKeyRangeGetReader) ReadResponse(response runtime.ClientResponse, consumer runtime.Consumer) (interface{}, error) {
	switch response.Code() {

	case 200:
		result := NewStorageServiceSampleKeyRangeGetOK()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return result, nil

	default:
		return nil, runtime.NewAPIError("unknown error", response, response.Code())
	}
}

// NewStorageServiceSampleKeyRangeGetOK creates a StorageServiceSampleKeyRangeGetOK with default headers values
func NewStorageServiceSampleKeyRangeGetOK() *StorageServiceSampleKeyRangeGetOK {
	return &StorageServiceSampleKeyRangeGetOK{}
}

/*StorageServiceSampleKeyRangeGetOK handles this case with default header values.

StorageServiceSampleKeyRangeGetOK storage service sample key range get o k
*/
type StorageServiceSampleKeyRangeGetOK struct {
	Payload []string
}

func (o *StorageServiceSampleKeyRangeGetOK) Error() string {
	return fmt.Sprintf("[GET /storage_service/sample_key_range][%d] storageServiceSampleKeyRangeGetOK  %+v", 200, o.Payload)
}

func (o *StorageServiceSampleKeyRangeGetOK) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	// response payload
	if err := consumer.Consume(response.Body(), &o.Payload); err != nil && err != io.EOF {
		return err
	}

	return nil
}