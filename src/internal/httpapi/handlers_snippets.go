package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/PabloPavan/Sniply/internal/snippets"
)

type SnippetsRepo interface {
	Create(ctx context.Context, s *snippets.Snippet) error
	GetByIDPublicOnly(ctx context.Context, id string) (*snippets.Snippet, error)
}

type SnippetsHandler struct {
	Repo SnippetsRepo
}

func (h *SnippetsHandler) Create(w http.ResponseWriter, r *http.Request) {
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
		req.Visibility = snippets.VisibilityPublic // para testar no Insomnia
	}

	s := &snippets.Snippet{
		ID:         "snp_" + randomHex(12),
		Name:       req.Name,
		Content:    req.Content,
		Language:   req.Language,
		Tags:       req.Tags,
		Visibility: req.Visibility,

		// MVP sem auth: setamos um creator fixo (ou vazio).
		// Quando entrar auth, isso vem do token.
		CreatorID: "usr_demo",
	}

	if err := h.Repo.Create(r.Context(), s); err != nil {
		http.Error(w, "failed to create snippet", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(s)
}

func (h *SnippetsHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	id = strings.TrimSpace(id)
	if id == "" {
		http.Error(w, "id is required", http.StatusBadRequest)
		return
	}

	s, err := h.Repo.GetByIDPublicOnly(r.Context(), id)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(s)
}

func randomHex(nBytes int) string {
	b := make([]byte, nBytes)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
