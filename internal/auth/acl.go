package auth

import "net/http"

const (
	ACLPrivate           = "private"
	ACLPublicRead        = "public-read"
	ACLPublicReadWrite   = "public-read-write"
	ACLAuthenticatedRead = "authenticated-read"
)

// ParseACL extracts the canned ACL from the request headers.
// If no ACL is provided, it returns "private" by default.
func ParseACL(r *http.Request) string {
	acl := r.Header.Get("x-amz-acl")
	switch acl {
	case ACLPublicRead, ACLPublicReadWrite, ACLAuthenticatedRead:
		return acl
	default:
		return ACLPrivate
	}
}

// IsPublicRead checks if the resource is publicly readable.
func IsPublicRead(acl string) bool {
	return acl == ACLPublicRead || acl == ACLPublicReadWrite
}
