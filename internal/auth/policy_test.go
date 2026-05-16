package auth_test

import (
	"testing"

	"gos3/internal/auth"
)

func TestEvaluatePolicy_Allow(t *testing.T) {
	policy := &auth.Policy{
		Version: "2012-10-17",
		Statement: []auth.Statement{
			{
				Effect:    "Allow",
				Principal: "*",
				Action:    "s3:GetObject",
				Resource:  "arn:aws:s3:::my-bucket/*",
			},
		},
	}
	user := &auth.User{Username: "testuser"}

	allowed, err := auth.EvaluatePolicy(policy, user, "s3:GetObject", "arn:aws:s3:::my-bucket/file.txt")
	if err != nil {
		t.Fatalf("EvaluatePolicy: %v", err)
	}
	if !allowed {
		t.Error("expected allowed=true")
	}
}

func TestEvaluatePolicy_DenyOverridesAllow(t *testing.T) {
	policy := &auth.Policy{
		Version: "2012-10-17",
		Statement: []auth.Statement{
			{
				Effect:    "Allow",
				Principal: "*",
				Action:    "s3:GetObject",
				Resource:  "arn:aws:s3:::bucket/*",
			},
			{
				Effect:    "Deny",
				Principal: "*",
				Action:    "s3:GetObject",
				Resource:  "arn:aws:s3:::bucket/secret/*",
			},
		},
	}
	user := &auth.User{Username: "testuser"}

	allowed, _ := auth.EvaluatePolicy(policy, user, "s3:GetObject", "arn:aws:s3:::bucket/secret/file.txt")
	if allowed {
		t.Error("expected denied — Deny should override Allow")
	}
}

func TestEvaluatePolicy_DefaultDeny(t *testing.T) {
	policy := &auth.Policy{
		Version: "2012-10-17",
		Statement: []auth.Statement{
			{
				Effect:    "Allow",
				Principal: "*",
				Action:    "s3:PutObject",
				Resource:  "arn:aws:s3:::bucket/*",
			},
		},
	}
	user := &auth.User{Username: "testuser"}

	// GetObject not in policy → default deny
	allowed, _ := auth.EvaluatePolicy(policy, user, "s3:GetObject", "arn:aws:s3:::bucket/file.txt")
	if allowed {
		t.Error("expected default deny for non-matching action")
	}
}

func TestEvaluatePolicy_WildcardAction(t *testing.T) {
	policy := &auth.Policy{
		Version: "2012-10-17",
		Statement: []auth.Statement{
			{
				Effect:    "Allow",
				Principal: "*",
				Action:    "s3:*",
				Resource:  "arn:aws:s3:::bucket/*",
			},
		},
	}
	user := &auth.User{Username: "testuser"}

	actions := []string{"s3:GetObject", "s3:PutObject", "s3:DeleteObject", "s3:ListBucket"}
	for _, action := range actions {
		allowed, _ := auth.EvaluatePolicy(policy, user, action, "arn:aws:s3:::bucket/file.txt")
		if !allowed {
			t.Errorf("s3:* should allow %s", action)
		}
	}
}

func TestEvaluatePolicy_WildcardPrincipal(t *testing.T) {
	policy := &auth.Policy{
		Version: "2012-10-17",
		Statement: []auth.Statement{
			{
				Effect:    "Allow",
				Principal: "*",
				Action:    "s3:GetObject",
				Resource:  "arn:aws:s3:::public-bucket/*",
			},
		},
	}

	users := []string{"alice", "bob", "anonymous"}
	for _, username := range users {
		user := &auth.User{Username: username}
		allowed, _ := auth.EvaluatePolicy(policy, user, "s3:GetObject", "arn:aws:s3:::public-bucket/data.txt")
		if !allowed {
			t.Errorf("wildcard principal should allow user %q", username)
		}
	}
}

func TestEvaluatePolicy_PrincipalMatchUsername(t *testing.T) {
	policy := &auth.Policy{
		Version: "2012-10-17",
		Statement: []auth.Statement{
			{
				Effect:    "Allow",
				Principal: "arn:aws:iam:::user/alice",
				Action:    "s3:GetObject",
				Resource:  "arn:aws:s3:::private-bucket/*",
			},
		},
	}

	alice := &auth.User{Username: "alice"}
	allowed, _ := auth.EvaluatePolicy(policy, alice, "s3:GetObject", "arn:aws:s3:::private-bucket/doc.txt")
	if !allowed {
		t.Error("alice should be allowed")
	}

	bob := &auth.User{Username: "bob"}
	allowed, _ = auth.EvaluatePolicy(policy, bob, "s3:GetObject", "arn:aws:s3:::private-bucket/doc.txt")
	if allowed {
		t.Error("bob should be denied")
	}
}

func TestEvaluatePolicy_PrincipalMatchAccessKeyID(t *testing.T) {
	policy := &auth.Policy{
		Version: "2012-10-17",
		Statement: []auth.Statement{
			{
				Effect:    "Allow",
				Principal: "arn:aws:iam:::serviceaccount/AKID123",
				Action:    "s3:GetObject",
				Resource:  "arn:aws:s3:::data-bucket/*",
			},
		},
	}

	sa := &auth.User{Username: "svc", AccessKeyID: "AKID123"}
	allowed, _ := auth.EvaluatePolicy(policy, sa, "s3:GetObject", "arn:aws:s3:::data-bucket/report.csv")
	if !allowed {
		t.Error("service account with matching AccessKeyID should be allowed")
	}

	other := &auth.User{Username: "svc2", AccessKeyID: "AKID999"}
	allowed, _ = auth.EvaluatePolicy(policy, other, "s3:GetObject", "arn:aws:s3:::data-bucket/report.csv")
	if allowed {
		t.Error("different AccessKeyID should be denied")
	}
}

func TestEvaluatePolicy_ResourcePrefixWildcard(t *testing.T) {
	policy := &auth.Policy{
		Version: "2012-10-17",
		Statement: []auth.Statement{
			{
				Effect:    "Allow",
				Principal: "*",
				Action:    "s3:GetObject",
				Resource:  "arn:aws:s3:::bucket/docs/*",
			},
		},
	}
	user := &auth.User{Username: "testuser"}

	tests := []struct {
		resource string
		want     bool
	}{
		{"arn:aws:s3:::bucket/docs/readme.md", true},
		{"arn:aws:s3:::bucket/docs/sub/file.txt", true},
		{"arn:aws:s3:::bucket/images/photo.jpg", false},
		{"arn:aws:s3:::bucket/docs", false}, // no trailing slash → no match
	}

	for _, tt := range tests {
		allowed, _ := auth.EvaluatePolicy(policy, user, "s3:GetObject", tt.resource)
		if allowed != tt.want {
			t.Errorf("EvaluatePolicy(%q) = %v, want %v", tt.resource, allowed, tt.want)
		}
	}
}

func TestEvaluatePolicy_NilPolicy(t *testing.T) {
	user := &auth.User{Username: "testuser"}
	allowed, err := auth.EvaluatePolicy(nil, user, "s3:GetObject", "arn:aws:s3:::bucket/file.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if allowed {
		t.Error("nil policy should deny")
	}
}

func TestEvaluatePolicy_MultipleActions(t *testing.T) {
	policy := &auth.Policy{
		Version: "2012-10-17",
		Statement: []auth.Statement{
			{
				Effect:    "Allow",
				Principal: "*",
				Action:    []interface{}{"s3:GetObject", "s3:PutObject"},
				Resource:  "arn:aws:s3:::bucket/*",
			},
		},
	}
	user := &auth.User{Username: "testuser"}

	allowed, _ := auth.EvaluatePolicy(policy, user, "s3:GetObject", "arn:aws:s3:::bucket/file.txt")
	if !allowed {
		t.Error("GetObject should be allowed")
	}

	allowed, _ = auth.EvaluatePolicy(policy, user, "s3:PutObject", "arn:aws:s3:::bucket/file.txt")
	if !allowed {
		t.Error("PutObject should be allowed")
	}

	allowed, _ = auth.EvaluatePolicy(policy, user, "s3:DeleteObject", "arn:aws:s3:::bucket/file.txt")
	if allowed {
		t.Error("DeleteObject should be denied")
	}
}

func TestEvaluatePolicy_MapPrincipal(t *testing.T) {
	policy := &auth.Policy{
		Version: "2012-10-17",
		Statement: []auth.Statement{
			{
				Effect: "Allow",
				Principal: map[string]interface{}{
					"AWS": "*",
				},
				Action:   "s3:GetObject",
				Resource: "arn:aws:s3:::public-bucket/*",
			},
		},
	}
	user := &auth.User{Username: "anyone"}
	allowed, _ := auth.EvaluatePolicy(policy, user, "s3:GetObject", "arn:aws:s3:::public-bucket/file.txt")
	if !allowed {
		t.Error("map principal with wildcard should allow")
	}
}
