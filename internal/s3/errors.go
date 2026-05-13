package s3

import (
	"encoding/xml"
	"net/http"
)

// Error represents an S3 XML Error response.
type Error struct {
	XMLName    xml.Name `xml:"Error"`
	Code       string   `xml:"Code"`
	Message    string   `xml:"Message"`
	Resource   string   `xml:"Resource"`
	RequestId  string   `xml:"RequestId"`
	HostId     string   `xml:"HostId"`
	HTTPStatus int      `xml:"-"`
}

func (e Error) Error() string {
	return e.Message
}

// Common S3 errors
var (
	ErrAccessDenied = Error{
		Code:       "AccessDenied",
		Message:    "Access Denied",
		HTTPStatus: http.StatusForbidden,
	}
	ErrBucketAlreadyExists = Error{
		Code:       "BucketAlreadyExists",
		Message:    "The requested bucket name is not available. The bucket namespace is shared by all users of the system. Please select a different name and try again.",
		HTTPStatus: http.StatusConflict,
	}
	ErrBucketAlreadyOwnedByYou = Error{
		Code:       "BucketAlreadyOwnedByYou",
		Message:    "Your previous request to create the named bucket succeeded and you already own it.",
		HTTPStatus: http.StatusConflict,
	}
	ErrBucketNotEmpty = Error{
		Code:       "BucketNotEmpty",
		Message:    "The bucket you tried to delete is not empty",
		HTTPStatus: http.StatusConflict,
	}
	ErrEntityTooLarge = Error{
		Code:       "EntityTooLarge",
		Message:    "Your proposed upload exceeds the maximum allowed object size.",
		HTTPStatus: http.StatusBadRequest,
	}
	ErrInsufficientStorage = Error{
		Code:       "InsufficientStorage",
		Message:    "There is not enough space on the disk.",
		HTTPStatus: http.StatusInsufficientStorage,
	}
	ErrInvalidAccessKeyId = Error{
		Code:       "InvalidAccessKeyId",
		Message:    "The AWS Access Key Id you provided does not exist in our records.",
		HTTPStatus: http.StatusForbidden,
	}
	ErrInvalidBucketName = Error{
		Code:       "InvalidBucketName",
		Message:    "The specified bucket is not valid.",
		HTTPStatus: http.StatusBadRequest,
	}
	ErrInvalidPart = Error{
		Code:       "InvalidPart",
		Message:    "One or more of the specified parts could not be found. The part may not have been uploaded, or the specified entity tag may not match the part's entity tag.",
		HTTPStatus: http.StatusBadRequest,
	}
	ErrInvalidPartOrder = Error{
		Code:       "InvalidPartOrder",
		Message:    "The list of parts was not in ascending order. The parts list must be specified in order by part number.",
		HTTPStatus: http.StatusBadRequest,
	}
	ErrInvalidRange = Error{
		Code:       "InvalidRange",
		Message:    "The requested range is not satisfiable",
		HTTPStatus: http.StatusRequestedRangeNotSatisfiable,
	}
	ErrInvalidCopySource = Error{
		Code:       "InvalidArgument",
		Message:    "Copy Source must mention the source bucket and key: sourcebucket/sourcekey",
		HTTPStatus: http.StatusBadRequest,
	}
	ErrKeyTooLongError = Error{
		Code:       "KeyTooLongError",
		Message:    "Your key is too long",
		HTTPStatus: http.StatusBadRequest,
	}
	ErrMalformedXML = Error{
		Code:       "MalformedXML",
		Message:    "The XML you provided was not well-formed or did not validate against our published schema",
		HTTPStatus: http.StatusBadRequest,
	}
	ErrMethodNotAllowed = Error{
		Code:       "MethodNotAllowed",
		Message:    "The specified method is not allowed against this resource.",
		HTTPStatus: http.StatusMethodNotAllowed,
	}
	ErrMissingContentLength = Error{
		Code:       "MissingContentLength",
		Message:    "You must provide the Content-Length HTTP header.",
		HTTPStatus: http.StatusLengthRequired,
	}
	ErrNoSuchBucket = Error{
		Code:       "NoSuchBucket",
		Message:    "The specified bucket does not exist",
		HTTPStatus: http.StatusNotFound,
	}
	ErrNoSuchKey = Error{
		Code:       "NoSuchKey",
		Message:    "The specified key does not exist.",
		HTTPStatus: http.StatusNotFound,
	}
	ErrNoSuchUpload = Error{
		Code:       "NoSuchUpload",
		Message:    "The specified multipart upload does not exist. The upload ID may be invalid, or the upload may have been aborted or completed.",
		HTTPStatus: http.StatusNotFound,
	}
	ErrNoSuchLifecycleConfiguration = Error{
		Code:       "NoSuchLifecycleConfiguration",
		Message:    "The lifecycle configuration does not exist",
		HTTPStatus: http.StatusNotFound,
	}
	ErrNotImplemented = Error{
		Code:       "NotImplemented",
		Message:    "A header you provided implies functionality that is not implemented",
		HTTPStatus: http.StatusNotImplemented,
	}
	ErrSignatureDoesNotMatch = Error{
		Code:       "SignatureDoesNotMatch",
		Message:    "The request signature we calculated does not match the signature you provided. Check your key and signing method.",
		HTTPStatus: http.StatusForbidden,
	}
	ErrSlowDown = Error{
		Code:       "SlowDown",
		Message:    "Please reduce your request rate.",
		HTTPStatus: http.StatusTooManyRequests,
	}
	ErrInternalError = Error{
		Code:       "InternalError",
		Message:    "We encountered an internal error. Please try again.",
		HTTPStatus: http.StatusInternalServerError,
	}
)

// WithResource returns a new Error with the specified resource.
func (e Error) WithResource(resource string) Error {
	e.Resource = resource
	return e
}
