package s3

import "time"

// ListAllMyBucketsResult is the response for ListBuckets.
type ListAllMyBucketsResult struct {
	XMLName string `xml:"http://s3.amazonaws.com/doc/2006-03-01/ ListAllMyBucketsResult"`
	Owner   Owner  `xml:"Owner"`
	Buckets struct {
		Bucket []Bucket `xml:"Bucket"`
	} `xml:"Buckets"`
}

type Owner struct {
	ID          string `xml:"ID"`
	DisplayName string `xml:"DisplayName"`
}

type Bucket struct {
	Name         string    `xml:"Name"`
	CreationDate time.Time `xml:"CreationDate"`
}

// CreateBucketConfiguration is the request body for CreateBucket.
type CreateBucketConfiguration struct {
	XMLName            string `xml:"http://s3.amazonaws.com/doc/2006-03-01/ CreateBucketConfiguration"`
	LocationConstraint string `xml:"LocationConstraint"`
}

// ListBucketResult is the response for ListObjects V1.
type ListBucketResult struct {
	XMLName        string         `xml:"http://s3.amazonaws.com/doc/2006-03-01/ ListBucketResult"`
	Name           string         `xml:"Name"`
	Prefix         string         `xml:"Prefix"`
	Marker         string         `xml:"Marker"`
	MaxKeys        int            `xml:"MaxKeys"`
	Delimiter      string         `xml:"Delimiter,omitempty"`
	IsTruncated    bool           `xml:"IsTruncated"`
	NextMarker     string         `xml:"NextMarker,omitempty"`
	Contents       []Object       `xml:"Contents"`
	CommonPrefixes []CommonPrefix `xml:"CommonPrefixes,omitempty"`
}

// ListBucketResultV2 is the response for ListObjects V2.
type ListBucketResultV2 struct {
	XMLName               string         `xml:"http://s3.amazonaws.com/doc/2006-03-01/ ListBucketResult"`
	Name                  string         `xml:"Name"`
	Prefix                string         `xml:"Prefix"`
	ContinuationToken     string         `xml:"ContinuationToken,omitempty"`
	NextContinuationToken string         `xml:"NextContinuationToken,omitempty"`
	KeyCount              int            `xml:"KeyCount"`
	MaxKeys               int            `xml:"MaxKeys"`
	Delimiter             string         `xml:"Delimiter,omitempty"`
	IsTruncated           bool           `xml:"IsTruncated"`
	Contents              []Object       `xml:"Contents"`
	CommonPrefixes        []CommonPrefix `xml:"CommonPrefixes,omitempty"`
}

type Object struct {
	Key          string    `xml:"Key"`
	LastModified time.Time `xml:"LastModified"`
	ETag         string    `xml:"ETag"`
	Size         int64     `xml:"Size"`
	StorageClass string    `xml:"StorageClass"`
	Owner        *Owner    `xml:"Owner,omitempty"`
}

type CommonPrefix struct {
	Prefix string `xml:"Prefix"`
}

// Delete is the request body for DeleteObjects.
type Delete struct {
	XMLName string             `xml:"Delete"`
	Quiet   bool               `xml:"Quiet"`
	Objects []ObjectIdentifier `xml:"Object"`
}

type ObjectIdentifier struct {
	Key       string `xml:"Key"`
	VersionId string `xml:"VersionId,omitempty"`
}

// DeleteResult is the response for DeleteObjects.
type DeleteResult struct {
	XMLName string        `xml:"http://s3.amazonaws.com/doc/2006-03-01/ DeleteResult"`
	Deleted []DeletedItem `xml:"Deleted"`
	Error   []DeleteError `xml:"Error"`
}

type DeletedItem struct {
	Key string `xml:"Key"`
}

type DeleteError struct {
	Key     string `xml:"Key"`
	Code    string `xml:"Code"`
	Message string `xml:"Message"`
}

// CopyObjectResult is the response for CopyObject.
type CopyObjectResult struct {
	XMLName      string    `xml:"CopyObjectResult"`
	LastModified time.Time `xml:"LastModified"`
	ETag         string    `xml:"ETag"`
}

// InitiateMultipartUploadResult is the response for CreateMultipartUpload.
type InitiateMultipartUploadResult struct {
	XMLName  string `xml:"http://s3.amazonaws.com/doc/2006-03-01/ InitiateMultipartUploadResult"`
	Bucket   string `xml:"Bucket"`
	Key      string `xml:"Key"`
	UploadId string `xml:"UploadId"`
}

// CompleteMultipartUpload is the request body for CompleteMultipartUpload.
type CompleteMultipartUpload struct {
	XMLName string `xml:"CompleteMultipartUpload"`
	Parts   []Part `xml:"Part"`
}

type Part struct {
	PartNumber int    `xml:"PartNumber"`
	ETag       string `xml:"ETag"`
}

// CompleteMultipartUploadResult is the response for CompleteMultipartUpload.
type CompleteMultipartUploadResult struct {
	XMLName  string `xml:"http://s3.amazonaws.com/doc/2006-03-01/ CompleteMultipartUploadResult"`
	Location string `xml:"Location"`
	Bucket   string `xml:"Bucket"`
	Key      string `xml:"Key"`
	ETag     string `xml:"ETag"`
}

// ListPartsResult is the response for ListParts.
type ListPartsResult struct {
	XMLName              string     `xml:"http://s3.amazonaws.com/doc/2006-03-01/ ListPartsResult"`
	Bucket               string     `xml:"Bucket"`
	Key                  string     `xml:"Key"`
	UploadId             string     `xml:"UploadId"`
	PartNumberMarker     int        `xml:"PartNumberMarker"`
	NextPartNumberMarker int        `xml:"NextPartNumberMarker"`
	MaxParts             int        `xml:"MaxParts"`
	IsTruncated          bool       `xml:"IsTruncated"`
	Initiator            Owner      `xml:"Initiator"`
	Owner                Owner      `xml:"Owner"`
	Parts                []PartInfo `xml:"Part"`
}

type PartInfo struct {
	PartNumber   int       `xml:"PartNumber"`
	LastModified time.Time `xml:"LastModified"`
	ETag         string    `xml:"ETag"`
	Size         int64     `xml:"Size"`
}

// ListMultipartUploadsResult is the response for ListMultipartUploads.
type ListMultipartUploadsResult struct {
	XMLName            string         `xml:"http://s3.amazonaws.com/doc/2006-03-01/ ListMultipartUploadsResult"`
	Bucket             string         `xml:"Bucket"`
	KeyMarker          string         `xml:"KeyMarker"`
	UploadIdMarker     string         `xml:"UploadIdMarker"`
	NextKeyMarker      string         `xml:"NextKeyMarker"`
	NextUploadIdMarker string         `xml:"NextUploadIdMarker"`
	MaxUploads         int            `xml:"MaxUploads"`
	IsTruncated        bool           `xml:"IsTruncated"`
	Uploads            []Upload       `xml:"Upload"`
	CommonPrefixes     []CommonPrefix `xml:"CommonPrefixes,omitempty"`
}

type Upload struct {
	Key       string    `xml:"Key"`
	UploadId  string    `xml:"UploadId"`
	Initiator Owner     `xml:"Initiator"`
	Owner     Owner     `xml:"Owner"`
	Initiated time.Time `xml:"Initiated"`
}
