package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/PabloPavan/sniply_api/internal/snippets"
)

type SnippetsService interface {
	Create(ctx context.Context, req snippets.CreateSnippetRequest) (*snippets.Snippet, error)
	GetByID(ctx context.Context, id string) (*snippets.Snippet, error)
	List(ctx context.Context, input snippets.ListInput) ([]*snippets.Snippet, error)
	Update(ctx context.Context, id string, req snippets.CreateSnippetRequest) (*snippets.Snippet, error)
	Delete(ctx context.Context, id string) error
}

type SnippetsHandler struct {
	Service SnippetsService
}

// Create Snippet
// @Summary Create snippet
// @Tags snippets
// @Accept json
// @Produce json
// @Security SessionAuth
// @Security ApiKeyAuth
// @Param body body SnippetCreateDTO true "snippet"
// @Param X-CSRF-Token header string false "CSRF token (required for SessionAuth)"
// @Success 201 {object} snippets.Snippet
// @Failure 400 {string} string
// @Failure 401 {string} string
// @Failure 409 {string} string
// @Failure 500 {string} string
// @Router /snippets [post]
func (h *SnippetsHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req SnippetCreateDTO
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if err := req.Validate(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	snippet, err := h.Service.Create(r.Context(), snippets.CreateSnippetRequest{
		Name:       req.Name,
		Content:    req.Content,
		Language:   req.Language,
		Tags:       req.Tags,
		Visibility: req.Visibility,
	})
	if err != nil {
		writeAppError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(snippet)
}

// GetByID Snippet
// @Summary Get snippet by id
// @Tags snippets
// @Produce json
// @Security SessionAuth
// @Security ApiKeyAuth
// @Param id path string true "snippet id"
// @Success 200 {object} snippets.Snippet
// @Failure 400 {string} string
// @Failure 404 {string} string
// @Failure 500 {string} string
// @Router /snippets/{id} [get]
func (h *SnippetsHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(chi.URLParam(r, "id"))

	snippet, err := h.Service.GetByID(r.Context(), id)
	if err != nil {
		writeAppError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(snippet)
}

// List Snippets
// @Summary List snippets
// @Tags snippets
// @Produce json
// @Security SessionAuth
// @Security ApiKeyAuth
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

	visibility := snippets.Visibility(strings.TrimSpace(r.URL.Query().Get("visibility")))

	limit := 0
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

	input := snippets.ListInput{
		Query:      q,
		Creator:    creator,
		Language:   language,
		Tag:        tag,
		Visibility: visibility,
		Limit:      limit,
		Offset:     offset,
	}

	list, err := h.Service.List(r.Context(), input)
	if err != nil {
		writeAppError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(list)
}

// Update Snippet
// @Summary Update snippet
// @Tags snippets
// @Accept json
// @Security SessionAuth
// @Security ApiKeyAuth
// @Param id path string true "snippet id"
// @Param body body SnippetCreateDTO true "snippet"
// @Param X-CSRF-Token header string false "CSRF token (required for SessionAuth)"
// @Success 204
// @Failure 400 {string} string
// @Failure 401 {string} string
// @Failure 404 {string} string
// @Failure 500 {string} string
// @Router /snippets/{id} [put]
func (h *SnippetsHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(chi.URLParam(r, "id"))

	var req SnippetCreateDTO
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if err := req.Validate(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	snippet, err := h.Service.Update(r.Context(), id, snippets.CreateSnippetRequest{
		Name:       req.Name,
		Content:    req.Content,
		Language:   req.Language,
		Tags:       req.Tags,
		Visibility: req.Visibility,
	})
	if err != nil {
		writeAppError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(snippet)
}

// Delete Snippet
// @Summary Delete snippet
// @Tags snippets
// @Security SessionAuth
// @Security ApiKeyAuth
// @Param id path string true "snippet id"
// @Param X-CSRF-Token header string false "CSRF token (required for SessionAuth)"
// @Success 204
// @Failure 400 {string} string
// @Failure 401 {string} string
// @Failure 404 {string} string
// @Failure 500 {string} string
// @Router /snippets/{id} [delete]
func (h *SnippetsHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(chi.URLParam(r, "id"))

	if err := h.Service.Delete(r.Context(), id); err != nil {
		writeAppError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
