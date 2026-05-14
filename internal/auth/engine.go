package auth

import (
	"context"
	"fmt"
)

// Engine evaluates whether a user is allowed to perform an action on a resource.
type Engine struct {
	iamStore    IAMPolicyStore
	policyStore PolicyStore
}

func NewEngine(iamStore IAMPolicyStore, policyStore PolicyStore) *Engine {
	return &Engine{
		iamStore:    iamStore,
		policyStore: policyStore,
	}
}

// IsAllowed evaluates the combined User IAM Policies and Bucket Policy.
// Returns true if allowed, false if denied (default deny or explicit deny).
func (e *Engine) IsAllowed(ctx context.Context, user *User, action, resource string, bucket string) (bool, error) {
	if user.IsRoot {
		return true, nil
	}

	allowed := false
	denied := false

	// 1. Evaluate User IAM Policies
	for _, policyName := range user.Policies {
		iamPolicy, err := e.iamStore.GetIAMPolicy(ctx, policyName)
		if err != nil {
			if err == ErrPolicyNotFound {
				continue
			}
			return false, fmt.Errorf("failed to fetch IAM policy %s: %w", policyName, err)
		}

		a, d, err := evaluatePolicyStatements(iamPolicy, user, action, resource)
		if err != nil {
			return false, err
		}
		if d {
			denied = true
		}
		if a {
			allowed = true
		}
	}

	// 2. Evaluate Bucket Policy (if applicable)
	if bucket != "" {
		bucketPolicy, err := e.policyStore.GetBucketPolicy(ctx, bucket)
		if err != nil && err != ErrPolicyNotFound {
			return false, fmt.Errorf("failed to fetch bucket policy: %w", err)
		}

		if bucketPolicy != nil {
			a, d, err := evaluatePolicyStatements(bucketPolicy, user, action, resource)
			if err != nil {
				return false, err
			}
			if d {
				denied = true
			}
			if a {
				allowed = true
			}
		}
	}

	// Explicit deny overrides any allow
	if denied {
		return false, nil
	}

	// Default is deny if not explicitly allowed
	return allowed, nil
}

// evaluatePolicyStatements checks the statements of a single policy.
// Returns (allowed, denied, error)
func evaluatePolicyStatements(policy *Policy, user *User, action, resource string) (bool, bool, error) {
	if policy == nil {
		return false, false, nil
	}

	allowed := false
	denied := false

	for _, stmt := range policy.Statement {
		// 1. Check Principal
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

		// 4. Condition (Not implemented fully yet)

		if stmt.Effect == "Deny" {
			denied = true
		} else if stmt.Effect == "Allow" {
			allowed = true
		}
	}

	return allowed, denied, nil
}
