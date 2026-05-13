package client

import (
	"encoding/xml"
	"time"
)

// Bucket represents an S3 bucket.
type Bucket struct {
	Name         string
	CreationDate time.Time
}

// ListAllMyBucketsResult is the S3 XML response for list buckets.
type ListAllMyBucketsResult struct {
	XMLName xml.Name `xml:"ListAllMyBucketsResult"`
	Owner   Owner    `xml:"Owner"`
	Buckets struct {
		Buckets []struct {
			Name         string    `xml:"Name"`
			CreationDate time.Time `xml:"CreationDate"`
		} `xml:"Bucket"`
	} `xml:"Buckets"`
}

// Object represents an object in a bucket list.
type Object struct {
	Key          string    `xml:"Key"`
	LastModified time.Time `xml:"LastModified"`
	ETag         string    `xml:"ETag"`
	Size         int64     `xml:"Size"`
	StorageClass string    `xml:"StorageClass"`
	Owner        *Owner    `xml:"Owner"`
}

// CommonPrefix represents a common prefix in a list result.
type CommonPrefix struct {
	Prefix string `xml:"Prefix"`
}

// ListBucketResult represents an S3 XML response for list objects.
type ListBucketResult struct {
	XMLName        xml.Name       `xml:"ListBucketResult"`
	Name           string         `xml:"Name"`
	Prefix         string         `xml:"Prefix"`
	Marker         string         `xml:"Marker"`
	NextMarker     string         `xml:"NextMarker"`
	MaxKeys        int            `xml:"MaxKeys"`
	Delimiter      string         `xml:"Delimiter"`
	IsTruncated    bool           `xml:"IsTruncated"`
	Contents       []Object       `xml:"Contents"`
	CommonPrefixes []CommonPrefix `xml:"CommonPrefixes"`
}

// ListBucketResultV2 represents an S3 XML response for list objects V2.
type ListBucketResultV2 struct {
	XMLName               xml.Name       `xml:"ListBucketResult"`
	Name                  string         `xml:"Name"`
	Prefix                string         `xml:"Prefix"`
	ContinuationToken     string         `xml:"ContinuationToken"`
	NextContinuationToken string         `xml:"NextContinuationToken"`
	MaxKeys               int            `xml:"MaxKeys"`
	Delimiter             string         `xml:"Delimiter"`
	IsTruncated           bool           `xml:"IsTruncated"`
	KeyCount              int            `xml:"KeyCount"`
	Contents              []Object       `xml:"Contents"`
	CommonPrefixes        []CommonPrefix `xml:"CommonPrefixes"`
}

// Owner represents the owner of a bucket or object.
type Owner struct {
	ID          string `xml:"ID"`
	DisplayName string `xml:"DisplayName"`
}

// ErrorResponse represents an S3 XML Error response.
type ErrorResponse struct {
	XMLName    xml.Name `xml:"Error"`
	Code       string   `xml:"Code"`
	Message    string   `xml:"Message"`
	Resource   string   `xml:"Resource"`
	RequestId  string   `xml:"RequestId"`
	HTTPStatus int      `xml:"-"`
}

func (e ErrorResponse) Error() string {
	return e.Message
}
