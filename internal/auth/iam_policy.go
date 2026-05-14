package auth

import "context"

type IAMPolicyStore interface {
	GetIAMPolicy(ctx context.Context, name string) (*Policy, error)
	PutIAMPolicy(ctx context.Context, name string, policy *Policy) error
	DeleteIAMPolicy(ctx context.Context, name string) error
	ListIAMPolicies(ctx context.Context) ([]string, error)
}
