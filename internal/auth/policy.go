package auth

import (
	"context"
	"errors"
	"strings"
)

// Policy definitions based on AWS IAM JSON
type Policy struct {
	Version   string      `json:"Version"`
	Statement []Statement `json:"Statement"`
}

type Statement struct {
	Sid       string                       `json:"Sid,omitempty"`
	Effect    string                       `json:"Effect"`              // "Allow" or "Deny"
	Principal interface{}                  `json:"Principal,omitempty"` // Can be string or map
	Action    interface{}                  `json:"Action"`              // Can be string or []string
	Resource  interface{}                  `json:"Resource"`            // Can be string or []string
	Condition map[string]map[string]string `json:"Condition,omitempty"`
}

var ErrPolicyNotFound = errors.New("policy not found")

type PolicyStore interface {
	GetBucketPolicy(ctx context.Context, bucket string) (*Policy, error)
	PutBucketPolicy(ctx context.Context, bucket string, policy *Policy) error
	DeleteBucketPolicy(ctx context.Context, bucket string) error
}

// Evaluate evaluates if an action is allowed for a given user, resource, and policy
func EvaluatePolicy(policy *Policy, user *User, action, resource string) (bool, error) {
	if policy == nil {
		return false, nil
	}

	allowed := false
	denied := false

	for _, stmt := range policy.Statement {
		// 1. Check Principal (Simplified: AWS ARN matching is complex, we'll support "*" or "arn:aws:iam:::user/username")
		if !matchPrincipal(stmt.Principal, user) {
			continue
		}

		// 2. Check Action
		if !matchListOrString(stmt.Action, action) {
			continue
		}

		// 3. Check Resource
		if !matchListOrString(stmt.Resource, resource) {
			continue
		}

		// 4. Condition (Not implemented fully yet, simple pass-through)

		if stmt.Effect == "Deny" {
			denied = true
		} else if stmt.Effect == "Allow" {
			allowed = true
		}
	}

	if denied {
		return false, nil
	}
	return allowed, nil
}

func matchPrincipal(principal interface{}, user *User) bool {
	if principal == nil {
		return false
	}
	
	// Helper to check if a pattern matches the user
	matchesUser := func(p string) bool {
		if p == "*" {
			return true
		}
		// Match user ARN
		if strings.HasSuffix(p, "/"+user.Username) {
			return true
		}
		// Match Service Account ARN (we define it as arn:aws:iam:::serviceaccount/ACCESS_KEY)
		if user.AccessKeyID != "" && strings.HasSuffix(p, "/"+user.AccessKeyID) {
			return true
		}
		return false
	}

	switch p := principal.(type) {
	case string:
		return matchesUser(p)
	case map[string]interface{}:
		// e.g. {"AWS": ["*"]} or {"AWS": "arn..."}
		if aws, ok := p["AWS"]; ok {
			return matchListOrString(aws, "arn:aws:iam:::user/"+user.Username) || 
				(user.AccessKeyID != "" && matchListOrString(aws, "arn:aws:iam:::serviceaccount/"+user.AccessKeyID)) ||
				matchListOrString(aws, "*")
		}
	}
	return false
}

func matchListOrString(field interface{}, value string) bool {
	if field == nil {
		return false
	}
	switch v := field.(type) {
	case string:
		return matchWildcard(v, value)
	case []interface{}:
		for _, item := range v {
			if s, ok := item.(string); ok {
				if matchWildcard(s, value) {
					return true
				}
			}
		}
	}
	return false
}

func matchWildcard(pattern, value string) bool {
	if pattern == "*" {
		return true
	}
	if strings.HasSuffix(pattern, "*") {
		return strings.HasPrefix(value, strings.TrimSuffix(pattern, "*"))
	}
	return pattern == value
}
