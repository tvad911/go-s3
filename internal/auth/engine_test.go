package auth_test

import (
	"context"
	"testing"

	"gos3/internal/auth"
)

// mockIAMPolicyStore implements auth.IAMPolicyStore for testing.
type mockIAMPolicyStore struct {
	policies map[string]*auth.Policy
}

func newMockIAMStore() *mockIAMPolicyStore {
	return &mockIAMPolicyStore{policies: make(map[string]*auth.Policy)}
}

func (m *mockIAMPolicyStore) GetIAMPolicy(_ context.Context, name string) (*auth.Policy, error) {
	if p, ok := m.policies[name]; ok {
		return p, nil
	}
	return nil, auth.ErrPolicyNotFound
}

func (m *mockIAMPolicyStore) PutIAMPolicy(_ context.Context, name string, policy *auth.Policy) error {
	m.policies[name] = policy
	return nil
}

func (m *mockIAMPolicyStore) DeleteIAMPolicy(_ context.Context, name string) error {
	delete(m.policies, name)
	return nil
}

func (m *mockIAMPolicyStore) ListIAMPolicies(_ context.Context) ([]string, error) {
	var names []string
	for k := range m.policies {
		names = append(names, k)
	}
	return names, nil
}

// mockPolicyStore implements auth.PolicyStore for testing.
type mockPolicyStore struct {
	policies map[string]*auth.Policy // keyed by bucket name
}

func newMockPolicyStore() *mockPolicyStore {
	return &mockPolicyStore{policies: make(map[string]*auth.Policy)}
}

func (m *mockPolicyStore) GetBucketPolicy(_ context.Context, bucket string) (*auth.Policy, error) {
	if p, ok := m.policies[bucket]; ok {
		return p, nil
	}
	return nil, auth.ErrPolicyNotFound
}

func (m *mockPolicyStore) PutBucketPolicy(_ context.Context, bucket string, policy *auth.Policy) error {
	m.policies[bucket] = policy
	return nil
}

func (m *mockPolicyStore) DeleteBucketPolicy(_ context.Context, bucket string) error {
	delete(m.policies, bucket)
	return nil
}

func TestEngine_RootAlwaysAllowed(t *testing.T) {
	engine := auth.NewEngine(newMockIAMStore(), newMockPolicyStore())
	ctx := context.Background()

	root := &auth.User{Username: "root", IsRoot: true}

	allowed, err := engine.IsAllowed(ctx, root, "s3:DeleteBucket", "arn:aws:s3:::any-bucket", "any-bucket")
	if err != nil {
		t.Fatalf("IsAllowed: %v", err)
	}
	if !allowed {
		t.Error("root user should always be allowed")
	}
}

func TestEngine_UserWithIAMAllow(t *testing.T) {
	iamStore := newMockIAMStore()
	iamStore.PutIAMPolicy(context.Background(), "read-policy", &auth.Policy{
		Version: "2012-10-17",
		Statement: []auth.Statement{
			{
				Effect:    "Allow",
				Principal: "*",
				Action:    "s3:GetObject",
				Resource:  "arn:aws:s3:::bucket/*",
			},
		},
	})

	engine := auth.NewEngine(iamStore, newMockPolicyStore())
	ctx := context.Background()

	user := &auth.User{
		Username: "reader",
		Policies: []string{"read-policy"},
	}

	allowed, err := engine.IsAllowed(ctx, user, "s3:GetObject", "arn:aws:s3:::bucket/file.txt", "bucket")
	if err != nil {
		t.Fatalf("IsAllowed: %v", err)
	}
	if !allowed {
		t.Error("user with matching IAM policy should be allowed")
	}
}

func TestEngine_UserNoPolicyDeny(t *testing.T) {
	engine := auth.NewEngine(newMockIAMStore(), newMockPolicyStore())
	ctx := context.Background()

	user := &auth.User{
		Username: "nobody",
		Policies: nil,
	}

	allowed, err := engine.IsAllowed(ctx, user, "s3:GetObject", "arn:aws:s3:::bucket/file.txt", "bucket")
	if err != nil {
		t.Fatalf("IsAllowed: %v", err)
	}
	if allowed {
		t.Error("user without any policy should be denied")
	}
}

func TestEngine_BucketPolicyAllowAnonymous(t *testing.T) {
	bucketStore := newMockPolicyStore()
	bucketStore.PutBucketPolicy(context.Background(), "public-bucket", &auth.Policy{
		Version: "2012-10-17",
		Statement: []auth.Statement{
			{
				Effect:    "Allow",
				Principal: "*",
				Action:    "s3:GetObject",
				Resource:  "arn:aws:s3:::public-bucket/*",
			},
		},
	})

	engine := auth.NewEngine(newMockIAMStore(), bucketStore)
	ctx := context.Background()

	anon := &auth.User{Username: "anonymous"}

	allowed, err := engine.IsAllowed(ctx, anon, "s3:GetObject", "arn:aws:s3:::public-bucket/readme.txt", "public-bucket")
	if err != nil {
		t.Fatalf("IsAllowed: %v", err)
	}
	if !allowed {
		t.Error("anonymous should be allowed by public bucket policy")
	}
}

func TestEngine_IAMAllowBucketDeny(t *testing.T) {
	iamStore := newMockIAMStore()
	iamStore.PutIAMPolicy(context.Background(), "allow-all", &auth.Policy{
		Version: "2012-10-17",
		Statement: []auth.Statement{
			{
				Effect:    "Allow",
				Principal: "*",
				Action:    "s3:*",
				Resource:  "arn:aws:s3:::restricted/*",
			},
		},
	})

	bucketStore := newMockPolicyStore()
	bucketStore.PutBucketPolicy(context.Background(), "restricted", &auth.Policy{
		Version: "2012-10-17",
		Statement: []auth.Statement{
			{
				Effect:    "Deny",
				Principal: "*",
				Action:    "s3:DeleteObject",
				Resource:  "arn:aws:s3:::restricted/*",
			},
		},
	})

	engine := auth.NewEngine(iamStore, bucketStore)
	ctx := context.Background()

	user := &auth.User{
		Username: "worker",
		Policies: []string{"allow-all"},
	}

	// DeleteObject → IAM Allow but Bucket Deny → Deny wins
	allowed, err := engine.IsAllowed(ctx, user, "s3:DeleteObject", "arn:aws:s3:::restricted/file.txt", "restricted")
	if err != nil {
		t.Fatalf("IsAllowed: %v", err)
	}
	if allowed {
		t.Error("explicit Deny from bucket policy should override IAM Allow")
	}

	// GetObject → IAM Allow, no Bucket Deny → Allow
	allowed, err = engine.IsAllowed(ctx, user, "s3:GetObject", "arn:aws:s3:::restricted/file.txt", "restricted")
	if err != nil {
		t.Fatalf("IsAllowed: %v", err)
	}
	if !allowed {
		t.Error("GetObject should still be allowed (no Deny for it)")
	}
}

func TestEngine_IAMDenyOverridesBucketAllow(t *testing.T) {
	iamStore := newMockIAMStore()
	iamStore.PutIAMPolicy(context.Background(), "deny-delete", &auth.Policy{
		Version: "2012-10-17",
		Statement: []auth.Statement{
			{
				Effect:    "Deny",
				Principal: "*",
				Action:    "s3:DeleteObject",
				Resource:  "arn:aws:s3:::bucket/*",
			},
		},
	})

	bucketStore := newMockPolicyStore()
	bucketStore.PutBucketPolicy(context.Background(), "bucket", &auth.Policy{
		Version: "2012-10-17",
		Statement: []auth.Statement{
			{
				Effect:    "Allow",
				Principal: "*",
				Action:    "s3:*",
				Resource:  "arn:aws:s3:::bucket/*",
			},
		},
	})

	engine := auth.NewEngine(iamStore, bucketStore)
	ctx := context.Background()

	user := &auth.User{
		Username: "restricted",
		Policies: []string{"deny-delete"},
	}

	allowed, err := engine.IsAllowed(ctx, user, "s3:DeleteObject", "arn:aws:s3:::bucket/file.txt", "bucket")
	if err != nil {
		t.Fatalf("IsAllowed: %v", err)
	}
	if allowed {
		t.Error("IAM Deny should override Bucket Allow")
	}
}

func TestEngine_MissingIAMPolicySkipped(t *testing.T) {
	engine := auth.NewEngine(newMockIAMStore(), newMockPolicyStore())
	ctx := context.Background()

	user := &auth.User{
		Username: "user",
		Policies: []string{"nonexistent-policy"},
	}

	allowed, err := engine.IsAllowed(ctx, user, "s3:GetObject", "arn:aws:s3:::bucket/file.txt", "bucket")
	if err != nil {
		t.Fatalf("IsAllowed should not error for missing policy: %v", err)
	}
	if allowed {
		t.Error("missing policy should not grant access")
	}
}
