package apikeys

import (
	"context"
	"strings"

	"github.com/PabloPavan/sniply_api/internal"
	"github.com/PabloPavan/sniply_api/internal/apperrors"
	"github.com/PabloPavan/sniply_api/internal/identity"
)

type Store interface {
	Create(ctx context.Context, k *Key) error
	ListByUser(ctx context.Context, userID string) ([]*Key, error)
	GetByID(ctx context.Context, id string) (*Key, error)
	Revoke(ctx context.Context, id string) (bool, error)
	GetByTokenHash(ctx context.Context, hash string) (*Key, error)
}

type Service struct {
	Store          Store
	IDGenerator    func() string
	TokenGenerator func() string
	TokenHasher    func(token string) string
	TokenPrefixer  func(token string) string
}

type CreateInput struct {
	Name  string
	Scope string
}

func (s *Service) Create(ctx context.Context, input CreateInput) (*Key, string, error) {
	if s.Store == nil {
		return nil, "", apperrors.New(apperrors.KindInternal, "api keys store not configured")
	}
	userID, ok := identity.UserID(ctx)
	if !ok || strings.TrimSpace(userID) == "" {
		return nil, "", apperrors.New(apperrors.KindUnauthorized, "unauthorized")
	}

	name := strings.TrimSpace(input.Name)
	scope := Scope(strings.TrimSpace(input.Scope))
	if scope == "" {
		scope = ScopeReadWrite
	}
	if !scope.Valid() {
		return nil, "", apperrors.New(apperrors.KindInvalidInput, "invalid scope")
	}

	idGen := s.IDGenerator
	if idGen == nil {
		idGen = func() string {
			return "key_" + internal.RandomHex(12)
		}
	}
	tokenGen := s.TokenGenerator
	if tokenGen == nil {
		tokenGen = GenerateToken
	}
	hashToken := s.TokenHasher
	if hashToken == nil {
		hashToken = HashToken
	}
	prefixer := s.TokenPrefixer
	if prefixer == nil {
		prefixer = TokenPrefix
	}

	token := tokenGen()
	key := &Key{
		ID:          idGen(),
		UserID:      userID,
		Name:        name,
		Scope:       scope,
		TokenHash:   hashToken(token),
		TokenPrefix: prefixer(token),
	}

	if err := s.Store.Create(ctx, key); err != nil {
		return nil, "", apperrors.New(apperrors.KindInternal, "failed to create api key")
	}
	return key, token, nil
}

func (s *Service) List(ctx context.Context) ([]*Key, error) {
	if s.Store == nil {
		return nil, apperrors.New(apperrors.KindInternal, "api keys store not configured")
	}
	userID, ok := identity.UserID(ctx)
	if !ok || strings.TrimSpace(userID) == "" {
		return nil, apperrors.New(apperrors.KindUnauthorized, "unauthorized")
	}

	keys, err := s.Store.ListByUser(ctx, userID)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "failed to list api keys")
	}
	return keys, nil
}

func (s *Service) Revoke(ctx context.Context, id string) error {
	if s.Store == nil {
		return apperrors.New(apperrors.KindInternal, "api keys store not configured")
	}
	userID, ok := identity.UserID(ctx)
	if !ok || strings.TrimSpace(userID) == "" {
		return apperrors.New(apperrors.KindUnauthorized, "unauthorized")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return apperrors.New(apperrors.KindInvalidInput, "invalid id")
	}

	key, err := s.Store.GetByID(ctx, id)
	if err != nil {
		if IsNotFound(err) {
			return apperrors.New(apperrors.KindNotFound, "api key not found")
		}
		return apperrors.New(apperrors.KindInternal, "failed to load api key")
	}
	if key.UserID != userID || key.RevokedAt != nil {
		return apperrors.New(apperrors.KindNotFound, "api key not found")
	}

	revoked, err := s.Store.Revoke(ctx, id)
	if err != nil {
		return apperrors.New(apperrors.KindInternal, "failed to revoke api key")
	}
	if !revoked {
		return apperrors.New(apperrors.KindNotFound, "api key not found")
	}
	return nil
}
