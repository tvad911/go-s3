# PLAN: IAM Policy Attachment

## Context
The current UI creates 1:1 IAM Policies named after the user/access key, which violates the standard AWS IAM pattern where policies are independent named entities (e.g., `fullaccess`) attached to multiple identities.

## Phase 1: Backend API Implementation
1. **Users Handler (`internal/handler/admin_users.go`)**:
   - Add `PutUserPolicies` to handle `PUT /_admin/users/{username}/policies`.
   - Safely update `user.Policies` array in `bboltStore` without overwriting the password hash.
2. **Service Accounts Handler (`internal/handler/admin_service_accounts.go`)**:
   - Add `PutServiceAccountPolicies` to handle `PUT /_admin/service-accounts/{id}/policies`.
   - Update `sa.Policies` array in `bboltStore`.
3. **Storage/Database**:
   - Implement `UpdateUserPolicies` and `UpdateServiceAccountPolicies` in `internal/storage/metadata/bbolt.go`.
4. **Router (`internal/server/router.go`)**:
   - Register the two new `PUT` routes.

## Phase 2: Frontend UI Implementation (`web/dist/app.js` & `index.html`)
1. **Modal Update**:
   - Replace `edit-iam-policy-modal` (JSON Editor) with `attach-iam-policy-modal` (Checkbox List).
2. **Button Update**:
   - Change "Edit Policy" buttons in User and SA tables to "Attach Policies".
3. **Logic Update**:
   - Fetch all available policies (`/_admin/policies`).
   - Fetch the selected User/SA to check which policies they currently have.
   - Send `PUT` request with array of selected policy names.

## Phase 3: Verification
- Verify User/SA can be attached to `fullaccess`.
- Verify `engine.go` correctly reads the policies array and enforces access.
