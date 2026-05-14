package handler

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"

	"gos3/internal/auth"
	"gos3/internal/s3"
)

// ListIAMPolicies handles GET /_admin/policies
func (h *AdminHandler) ListIAMPolicies(w http.ResponseWriter, r *http.Request) {
	if err := h.CheckAdminPolicy(r, "admin:ListIAMPolicies"); err != nil {
		WriteError(w, r, err)
		return
	}

	names, err := h.MetaStore.ListIAMPolicies(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(names)
}

// GetIAMPolicy handles GET /_admin/policies/{name}
func (h *AdminHandler) GetIAMPolicy(w http.ResponseWriter, r *http.Request) {
	if err := h.CheckAdminPolicy(r, "admin:GetIAMPolicy"); err != nil {
		WriteError(w, r, err)
		return
	}

	name := chi.URLParam(r, "name")
	policy, err := h.MetaStore.GetIAMPolicy(r.Context(), name)
	if err != nil {
		if err == auth.ErrPolicyNotFound {
			http.Error(w, "Policy not found", http.StatusNotFound)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(policy)
}

// PutIAMPolicy handles PUT /_admin/policies/{name}
func (h *AdminHandler) PutIAMPolicy(w http.ResponseWriter, r *http.Request) {
	if err := h.CheckAdminPolicy(r, "admin:PutIAMPolicy"); err != nil {
		WriteError(w, r, err)
		return
	}

	name := chi.URLParam(r, "name")
	
	var policy auth.Policy
	if err := json.NewDecoder(io.LimitReader(r.Body, 2<<20)).Decode(&policy); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := h.MetaStore.PutIAMPolicy(r.Context(), name, &policy); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.recordAudit(r, "PutIAMPolicy", name, "")
	w.WriteHeader(http.StatusNoContent)
}

// DeleteIAMPolicy handles DELETE /_admin/policies/{name}
func (h *AdminHandler) DeleteIAMPolicy(w http.ResponseWriter, r *http.Request) {
	if err := h.CheckAdminPolicy(r, "admin:DeleteIAMPolicy"); err != nil {
		WriteError(w, r, err)
		return
	}

	name := chi.URLParam(r, "name")
	if err := h.MetaStore.DeleteIAMPolicy(r.Context(), name); err != nil {
		if err == auth.ErrPolicyNotFound {
			http.Error(w, "Policy not found", http.StatusNotFound)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	h.recordAudit(r, "DeleteIAMPolicy", name, "")
	w.WriteHeader(http.StatusNoContent)
}

// CheckAdminPolicy evaluates whether the user can perform an admin action.
func (h *AdminHandler) CheckAdminPolicy(r *http.Request, action string) error {
	user := auth.GetUser(r.Context())
	if user.IsRoot {
		return nil
	}

	// For admin actions, the resource is generally the system itself
	allowed, err := h.PolicyEngine.IsAllowed(r.Context(), user, action, "arn:aws:s3:::*", "")
	if err != nil {
		return s3.ErrAccessDenied
	}
	if !allowed {
		return s3.ErrAccessDenied
	}
	return nil
}
