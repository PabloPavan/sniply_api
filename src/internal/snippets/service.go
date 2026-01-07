package snippets

import (
	"context"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/PabloPavan/sniply_api/internal"
	"github.com/PabloPavan/sniply_api/internal/apperrors"
	"github.com/PabloPavan/sniply_api/internal/identity"
	"github.com/PabloPavan/sniply_api/internal/users"
)

type Store interface {
	Create(ctx context.Context, s *Snippet) error
	GetByIDPublicOnly(ctx context.Context, id string) (*Snippet, error)
	List(ctx context.Context, f SnippetFilter) ([]*Snippet, error)
	Update(ctx context.Context, s *Snippet) error
	Delete(ctx context.Context, id string, creatorID string) error
}

type UserLookup interface {
	GetByID(ctx context.Context, id string) (*users.User, error)
}

type Service struct {
	Store        Store
	Users        UserLookup
	Cache        Cache
	CacheTTL     time.Duration
	ListCacheTTL time.Duration
	IDGenerator  func() string
}

type ListInput struct {
	Query      string
	Creator    string
	Language   string
	Tag        string
	Visibility Visibility
	Limit      int
	Offset     int
}

func (s *Service) Create(ctx context.Context, req CreateSnippetRequest) (*Snippet, error) {
	if s.Store == nil {
		return nil, apperrors.New(apperrors.KindInternal, "snippets store not configured")
	}
	creatorID, ok := identity.UserID(ctx)
	if !ok || strings.TrimSpace(creatorID) == "" {
		return nil, apperrors.New(apperrors.KindUnauthorized, "unauthorized")
	}

	name := strings.TrimSpace(req.Name)
	content := strings.TrimSpace(req.Content)
	language := strings.TrimSpace(req.Language)
	if name == "" || content == "" {
		return nil, apperrors.New(apperrors.KindInvalidInput, "name and content are required")
	}
	if language == "" {
		language = "txt"
	}
	tags := req.Tags
	if tags == nil {
		tags = []string{}
	}
	visibility := req.Visibility
	if visibility == "" {
		visibility = VisibilityPrivate
	}

	idGen := s.IDGenerator
	if idGen == nil {
		idGen = func() string {
			return "snp_" + internal.RandomHex(12)
		}
	}

	snippet := &Snippet{
		ID:         idGen(),
		Name:       name,
		Content:    content,
		Language:   language,
		Tags:       tags,
		Visibility: visibility,
		CreatorID:  creatorID,
	}

	if err := s.Store.Create(ctx, snippet); err != nil {
		if IsUniqueViolationID(err) {
			return nil, apperrors.New(apperrors.KindConflict, "snippet already exists")
		}
		return nil, apperrors.New(apperrors.KindInternal, "failed to create snippet")
	}

	return snippet, nil
}

func (s *Service) GetByID(ctx context.Context, id string) (*Snippet, error) {
	if s.Store == nil {
		return nil, apperrors.New(apperrors.KindInternal, "snippets store not configured")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, apperrors.New(apperrors.KindInvalidInput, "id is required")
	}

	if s.Cache != nil {
		if cached, ok, err := s.Cache.GetByID(ctx, id); err == nil && ok {
			return cached, nil
		}
	}

	snippet, err := s.Store.GetByIDPublicOnly(ctx, id)
	if err != nil {
		if IsNotFound(err) {
			return nil, apperrors.New(apperrors.KindNotFound, "not found")
		}
		return nil, apperrors.New(apperrors.KindInternal, "failed to load snippet")
	}

	if s.Cache != nil && s.CacheTTL > 0 {
		_ = s.Cache.SetByID(ctx, snippet, s.CacheTTL)
	}

	return snippet, nil
}

func (s *Service) List(ctx context.Context, input ListInput) ([]*Snippet, error) {
	if s.Store == nil {
		return nil, apperrors.New(apperrors.KindInternal, "snippets store not configured")
	}

	input.Query = strings.TrimSpace(input.Query)
	input.Creator = strings.TrimSpace(input.Creator)
	input.Language = strings.TrimSpace(input.Language)
	input.Tag = strings.TrimSpace(input.Tag)

	if input.Creator != "" {
		if s.Users == nil {
			return nil, apperrors.New(apperrors.KindInternal, "users store not configured")
		}
		_, err := s.Users.GetByID(ctx, input.Creator)
		if err != nil {
			if users.IsNotFound(err) {
				return nil, apperrors.New(apperrors.KindInvalidInput, "creator not found")
			}
			return nil, apperrors.New(apperrors.KindInternal, "failed to load creator")
		}
	}

	visibility := input.Visibility
	if visibility != VisibilityPrivate {
		visibility = VisibilityPublic
	}
	if visibility == VisibilityPrivate {
		if input.Creator == "" {
			return nil, apperrors.New(apperrors.KindInvalidInput, "creator is required")
		}
		requesterID, ok := identity.UserID(ctx)
		if !ok || strings.TrimSpace(requesterID) == "" {
			return nil, apperrors.New(apperrors.KindUnauthorized, "unauthorized")
		}
		if !identity.IsAdmin(ctx) && requesterID != input.Creator {
			return nil, apperrors.New(apperrors.KindForbidden, "forbidden")
		}
	}

	limit := 100
	if input.Limit > 0 {
		limit = input.Limit
	}
	offset := 0
	if input.Offset > 0 {
		offset = input.Offset
	}

	var tags []string
	if input.Tag != "" {
		tags = []string{input.Tag}
	}

	filter := SnippetFilter{
		Query:      input.Query,
		Creator:    input.Creator,
		Language:   input.Language,
		Tags:       tags,
		Visibility: visibility,
		Limit:      limit,
		Offset:     offset,
	}

	if s.Cache != nil && visibility == VisibilityPublic {
		cacheKey := listCacheKey(filter)
		if cached, ok, err := s.Cache.GetList(ctx, cacheKey); err == nil && ok {
			return cached, nil
		}
	}

	list, err := s.Store.List(ctx, filter)
	if err != nil {
		if IsNotFound(err) {
			return nil, apperrors.New(apperrors.KindNotFound, "not found any snippets")
		}
		return nil, apperrors.New(apperrors.KindInternal, "failed to list snippets")
	}

	if s.Cache != nil && visibility == VisibilityPublic && s.ListCacheTTL > 0 {
		cacheKey := listCacheKey(filter)
		_ = s.Cache.SetList(ctx, cacheKey, list, s.ListCacheTTL)
	}

	return list, nil
}

func (s *Service) Update(ctx context.Context, id string, req CreateSnippetRequest) (*Snippet, error) {
	if s.Store == nil {
		return nil, apperrors.New(apperrors.KindInternal, "snippets store not configured")
	}
	requesterID, ok := identity.UserID(ctx)
	if !ok || strings.TrimSpace(requesterID) == "" {
		return nil, apperrors.New(apperrors.KindUnauthorized, "unauthorized")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, apperrors.New(apperrors.KindInvalidInput, "id is required")
	}

	name := strings.TrimSpace(req.Name)
	content := strings.TrimSpace(req.Content)
	language := strings.TrimSpace(req.Language)
	if name == "" || content == "" {
		return nil, apperrors.New(apperrors.KindInvalidInput, "name and content are required")
	}
	if language == "" {
		language = "txt"
	}
	tags := req.Tags
	if tags == nil {
		tags = []string{}
	}
	visibility := req.Visibility
	if visibility == "" {
		visibility = VisibilityPrivate
	}

	snippet := &Snippet{
		ID:         id,
		Name:       name,
		Content:    content,
		Language:   language,
		Tags:       tags,
		Visibility: visibility,
		CreatorID:  requesterID,
	}

	if err := s.Store.Update(ctx, snippet); err != nil {
		if IsNotFound(err) {
			return nil, apperrors.New(apperrors.KindNotFound, "not found")
		}
		return nil, apperrors.New(apperrors.KindInternal, "failed to update snippet")
	}

	if s.Cache != nil {
		_ = s.Cache.DeleteByID(ctx, id)
	}

	return snippet, nil
}

func (s *Service) Delete(ctx context.Context, id string) error {
	if s.Store == nil {
		return apperrors.New(apperrors.KindInternal, "snippets store not configured")
	}
	requesterID, ok := identity.UserID(ctx)
	if !ok || strings.TrimSpace(requesterID) == "" {
		return apperrors.New(apperrors.KindUnauthorized, "unauthorized")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return apperrors.New(apperrors.KindInvalidInput, "id is required")
	}

	if err := s.Store.Delete(ctx, id, requesterID); err != nil {
		if IsNotFound(err) {
			return apperrors.New(apperrors.KindNotFound, "not found")
		}
		return apperrors.New(apperrors.KindInternal, "failed to delete snippet")
	}

	if s.Cache != nil {
		_ = s.Cache.DeleteByID(ctx, id)
	}

	return nil
}

func listCacheKey(f SnippetFilter) string {
	v := url.Values{}
	if f.Query != "" {
		v.Set("q", f.Query)
	}
	if f.Creator != "" {
		v.Set("creator", f.Creator)
	}
	if f.Language != "" {
		v.Set("language", f.Language)
	}
	if len(f.Tags) > 0 {
		tags := append([]string(nil), f.Tags...)
		sort.Strings(tags)
		v.Set("tags", strings.Join(tags, ","))
	}
	if f.Visibility != "" {
		v.Set("visibility", string(f.Visibility))
	}
	if f.Limit > 0 {
		v.Set("limit", strconv.Itoa(f.Limit))
	}
	if f.Offset > 0 {
		v.Set("offset", strconv.Itoa(f.Offset))
	}
	return v.Encode()
}
