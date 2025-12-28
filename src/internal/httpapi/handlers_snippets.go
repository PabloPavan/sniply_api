package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/PabloPavan/Sniply/internal"
	"github.com/PabloPavan/Sniply/internal/identity"
	"github.com/PabloPavan/Sniply/internal/snippets"
	"github.com/PabloPavan/Sniply/internal/telemetry"
	"github.com/PabloPavan/Sniply/internal/users"
	"go.opentelemetry.io/otel/attribute"
)

type SnippetsRepo interface {
	Create(ctx context.Context, s *snippets.Snippet) error
	GetByIDPublicOnly(ctx context.Context, id string) (*snippets.Snippet, error)
	List(ctx context.Context, f snippets.SnippetFilter) ([]*snippets.Snippet, error)
	Update(ctx context.Context, s *snippets.Snippet) error
	Delete(ctx context.Context, id string, creatorID string) error
}

type SnippetsHandler struct {
	Repo     SnippetsRepo
	RepoUser UsersRepo
}

// Create Snippet
// @Summary Create snippet
// @Tags snippets
// @Accept json
// @Produce json
// @Security SessionAuth
// @Param body body snippets.CreateSnippetRequest true "snippet"
// @Success 201 {object} snippets.Snippet
// @Failure 400 {string} string
// @Failure 401 {string} string
// @Failure 409 {string} string
// @Failure 500 {string} string
// @Router /snippets [post]
func (h *SnippetsHandler) Create(w http.ResponseWriter, r *http.Request) {
	creatorID, ok := identity.UserID(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req snippets.CreateSnippetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	req.Content = strings.TrimSpace(req.Content)
	req.Language = strings.TrimSpace(req.Language)

	if req.Name == "" || req.Content == "" {
		http.Error(w, "name and content are required", http.StatusBadRequest)
		return
	}
	if req.Language == "" {
		req.Language = "txt"
	}
	if req.Visibility == "" {
		req.Visibility = snippets.VisibilityPrivate
	}

	ctx := r.Context()

	s := &snippets.Snippet{
		ID:         "snp_" + internal.RandomHex(12),
		Name:       req.Name,
		Content:    req.Content,
		Language:   req.Language,
		Tags:       req.Tags,
		Visibility: req.Visibility,
		CreatorID:  creatorID,
	}

	createCtx, span := telemetry.StartSpan(ctx, "snippets.create",
		attribute.String("snippet.id", s.ID),
		attribute.String("snippet.language", s.Language),
		attribute.Int("snippet.size_bytes", len(s.Content)),
		attribute.String("user.id", creatorID),
	)
	err := h.Repo.Create(createCtx, s)
	span.End()
	if err != nil {
		if snippets.IsUniqueViolationID(err) {
			http.Error(w, "snippet already exists", http.StatusConflict)
			return
		}

		http.Error(w, "failed to create snippet", http.StatusInternalServerError)
		return
	}

	telemetry.LogInfo(r.Context(), "snippet created",
		telemetry.LogString("event", "snippet.created"),
		telemetry.LogString("snippet.id", s.ID),
		telemetry.LogString("snippet.language", s.Language),
		telemetry.LogInt("snippet.size_bytes", len(s.Content)),
		telemetry.LogString("user.id", creatorID),
	)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(s)
}

// GetByID Snippet
// @Summary Get snippet by id
// @Tags snippets
// @Produce json
// @Security SessionAuth
// @Param id path string true "snippet id"
// @Success 200 {object} snippets.Snippet
// @Failure 400 {string} string
// @Failure 404 {string} string
// @Failure 500 {string} string
// @Router /snippets/{id} [get]
func (h *SnippetsHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(chi.URLParam(r, "id"))
	if id == "" {
		http.Error(w, "id is required", http.StatusBadRequest)
		return
	}

	s, err := h.Repo.GetByIDPublicOnly(r.Context(), id)
	if err != nil {
		if snippets.IsNotFound(err) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to load snippet", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(s)
}

// List Snippets
// @Summary List snippets
// @Tags snippets
// @Produce json
// @Security SessionAuth
// @Param q query string false "search"
// @Param creator query string false "creator id"
// @Param language query string false "language"
// @Param tag query string false "tag"
// @Param visibility query string false "visibility"
// @Param limit query int false "limit"
// @Param offset query int false "offset"
// @Success 200 {array} snippets.Snippet
// @Failure 400 {string} string
// @Failure 401 {string} string
// @Failure 403 {string} string
// @Failure 500 {string} string
// @Router /snippets [get]
func (h *SnippetsHandler) List(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	creator := strings.TrimSpace(r.URL.Query().Get("creator"))
	language := strings.TrimSpace(r.URL.Query().Get("language"))
	tag := strings.TrimSpace(r.URL.Query().Get("tag"))
	var tags []string
	if tag != "" {
		tags = []string{tag}
	}

	if creator != "" {
		_, err := h.RepoUser.GetByID(r.Context(), creator)
		if err != nil {
			if users.IsNotFound(err) {
				http.Error(w, "creator not found", http.StatusBadRequest)
				return
			}
			http.Error(w, "failed to load creator", http.StatusInternalServerError)
			return
		}
	}

	visibilityStr := strings.TrimSpace(r.URL.Query().Get("visibility"))
	visibility := snippets.VisibilityPublic
	if visibilityStr == string(snippets.VisibilityPrivate) {
		visibility = snippets.VisibilityPrivate
		if creator == "" {
			http.Error(w, "creator is required", http.StatusBadRequest)
			return
		}

		requesterID, ok := identity.UserID(r.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		isAdmin := identity.IsAdmin(r.Context())
		if !isAdmin && requesterID != creator {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
	}

	limit := 100
	offset := 0
	if l := strings.TrimSpace(r.URL.Query().Get("limit")); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 {
			limit = v
		}
	}
	if o := strings.TrimSpace(r.URL.Query().Get("offset")); o != "" {
		if v, err := strconv.Atoi(o); err == nil && v >= 0 {
			offset = v
		}
	}

	f := snippets.SnippetFilter{
		Query:      q,
		Creator:    creator,
		Language:   language,
		Tags:       tags,
		Visibility: visibility,
		Limit:      limit,
		Offset:     offset,
	}

	s, err := h.Repo.List(r.Context(), f)
	if err != nil {
		http.Error(w, "failed to list snippets", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(s)
}

// Update Snippet
// @Summary Update snippet
// @Tags snippets
// @Accept json
// @Security SessionAuth
// @Param id path string true "snippet id"
// @Param body body snippets.CreateSnippetRequest true "snippet"
// @Success 204
// @Failure 400 {string} string
// @Failure 401 {string} string
// @Failure 404 {string} string
// @Failure 500 {string} string
// @Router /snippets/{id} [put]
func (h *SnippetsHandler) Update(w http.ResponseWriter, r *http.Request) {
	creatorID, ok := identity.UserID(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	id := strings.TrimSpace(chi.URLParam(r, "id"))
	if id == "" {
		http.Error(w, "id is required", http.StatusBadRequest)
		return
	}

	var req snippets.CreateSnippetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	req.Content = strings.TrimSpace(req.Content)
	req.Language = strings.TrimSpace(req.Language)

	if req.Name == "" || req.Content == "" {
		http.Error(w, "name and content are required", http.StatusBadRequest)
		return
	}
	if req.Language == "" {
		req.Language = "txt"
	}
	if req.Visibility == "" {
		req.Visibility = snippets.VisibilityPrivate
	}

	s := &snippets.Snippet{
		ID:         id,
		Name:       req.Name,
		Content:    req.Content,
		Language:   req.Language,
		Tags:       req.Tags,
		Visibility: req.Visibility,
		CreatorID:  creatorID,
	}

	if err := h.Repo.Update(r.Context(), s); err != nil {
		if snippets.IsNotFound(err) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to update snippet", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(s)
}

// Delete Snippet
// @Summary Delete snippet
// @Tags snippets
// @Security SessionAuth
// @Param id path string true "snippet id"
// @Success 204
// @Failure 400 {string} string
// @Failure 401 {string} string
// @Failure 404 {string} string
// @Failure 500 {string} string
// @Router /snippets/{id} [delete]
func (h *SnippetsHandler) Delete(w http.ResponseWriter, r *http.Request) {
	creatorID, ok := identity.UserID(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	id := strings.TrimSpace(chi.URLParam(r, "id"))
	if id == "" {
		http.Error(w, "id is required", http.StatusBadRequest)
		return
	}

	if err := h.Repo.Delete(r.Context(), id, creatorID); err != nil {
		if snippets.IsNotFound(err) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to delete snippet", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
