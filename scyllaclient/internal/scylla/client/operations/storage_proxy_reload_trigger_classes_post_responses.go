// Code generated by go-swagger; DO NOT EDIT.

package operations

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the swagger generate command

import (
	"fmt"

	"github.com/go-openapi/runtime"

	strfmt "github.com/go-openapi/strfmt"
)

// StorageProxyReloadTriggerClassesPostReader is a Reader for the StorageProxyReloadTriggerClassesPost structure.
type StorageProxyReloadTriggerClassesPostReader struct {
	formats strfmt.Registry
}

// ReadResponse reads a server response into the received o.
func (o *StorageProxyReloadTriggerClassesPostReader) ReadResponse(response runtime.ClientResponse, consumer runtime.Consumer) (interface{}, error) {
	switch response.Code() {

	case 200:
		result := NewStorageProxyReloadTriggerClassesPostOK()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return result, nil

	default:
		return nil, runtime.NewAPIError("unknown error", response, response.Code())
	}
}

// NewStorageProxyReloadTriggerClassesPostOK creates a StorageProxyReloadTriggerClassesPostOK with default headers values
func NewStorageProxyReloadTriggerClassesPostOK() *StorageProxyReloadTriggerClassesPostOK {
	return &StorageProxyReloadTriggerClassesPostOK{}
}

/*StorageProxyReloadTriggerClassesPostOK handles this case with default header values.

StorageProxyReloadTriggerClassesPostOK storage proxy reload trigger classes post o k
*/
type StorageProxyReloadTriggerClassesPostOK struct {
}

func (o *StorageProxyReloadTriggerClassesPostOK) Error() string {
	return fmt.Sprintf("[POST /storage_proxy/reload_trigger_classes][%d] storageProxyReloadTriggerClassesPostOK ", 200)
}

func (o *StorageProxyReloadTriggerClassesPostOK) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	return nil
}