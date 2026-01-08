package users

import (
	"context"
	"strings"

	"github.com/PabloPavan/sniply_api/internal"
	"github.com/PabloPavan/sniply_api/internal/apperrors"
	"github.com/PabloPavan/sniply_api/internal/identity"
)

type Store interface {
	Create(ctx context.Context, u *User) error
	GetByID(ctx context.Context, id string) (*User, error)
	GetByEmail(ctx context.Context, email string) (User, error)
	List(ctx context.Context, f UserFilter) ([]*User, error)
	Update(ctx context.Context, u *UpdateUserRequest) error
	Delete(ctx context.Context, id string) error
}

type Service struct {
	Store          Store
	PasswordHasher func(plain string) (string, error)
	IDGenerator    func() string
}

type UpdateUserInput struct {
	Email    *string
	Password *string
	Role     *string
}

func (s *Service) Create(ctx context.Context, req CreateUserRequest) (*User, error) {
	if s.Store == nil {
		return nil, apperrors.New(apperrors.KindInternal, "users store not configured")
	}

	email := strings.TrimSpace(strings.ToLower(req.Email))
	password := strings.TrimSpace(req.Password)

	hasher := s.PasswordHasher
	if hasher == nil {
		hasher = internal.DefaultPasswordHasher
	}

	hash, err := hasher(password)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "failed to process password")
	}

	idGen := s.IDGenerator
	if idGen == nil {
		idGen = func() string {
			return "usr_" + internal.RandomHex(12)
		}
	}

	u := &User{
		ID:           idGen(),
		Email:        email,
		PasswordHash: hash,
	}

	if err := s.Store.Create(ctx, u); err != nil {
		if IsUniqueViolationEmail(err) {
			return nil, apperrors.New(apperrors.KindConflict, "email already exists")
		}
		return nil, apperrors.New(apperrors.KindInternal, "failed to create user")
	}

	return u, nil
}

func (s *Service) GetByID(ctx context.Context, userID string) (*User, error) {
	if s.Store == nil {
		return nil, apperrors.New(apperrors.KindInternal, "users store not configured")
	}
	if strings.TrimSpace(userID) == "" {
		return nil, apperrors.New(apperrors.KindInvalidInput, "user id is required")
	}

	u, err := s.Store.GetByID(ctx, userID)
	if err != nil {
		if IsNotFound(err) {
			return nil, apperrors.New(apperrors.KindNotFound, "user not found")
		}
		return nil, apperrors.New(apperrors.KindInternal, "failed to load user")
	}
	return u, nil
}

func (s *Service) Me(ctx context.Context) (*User, error) {
	if s.Store == nil {
		return nil, apperrors.New(apperrors.KindInternal, "users store not configured")
	}
	userID, ok := identity.UserID(ctx)
	if !ok || strings.TrimSpace(userID) == "" {
		return nil, apperrors.New(apperrors.KindUnauthorized, "unauthorized")
	}

	u, err := s.Store.GetByID(ctx, userID)
	if err != nil {
		if IsNotFound(err) {
			return nil, apperrors.New(apperrors.KindNotFound, "user not found")
		}
		return nil, apperrors.New(apperrors.KindInternal, "failed to load user")
	}
	return u, nil
}

func (s *Service) List(ctx context.Context, f UserFilter) ([]*User, error) {
	if s.Store == nil {
		return nil, apperrors.New(apperrors.KindInternal, "users store not configured")
	}
	requesterID, ok := identity.UserID(ctx)
	if !ok || strings.TrimSpace(requesterID) == "" {
		return nil, apperrors.New(apperrors.KindUnauthorized, "unauthorized")
	}
	if !identity.IsAdmin(ctx) {
		return nil, apperrors.New(apperrors.KindForbidden, "forbidden")
	}

	limit := 100
	if f.Limit > 0 {
		limit = min(f.Limit, 1000)
	}
	offset := 0
	if f.Offset > 0 {
		offset = f.Offset
	}
	f.Limit = limit
	f.Offset = offset

	list, err := s.Store.List(ctx, f)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "failed to list users")
	}
	return list, nil
}

func (s *Service) UpdateSelf(ctx context.Context, input UpdateUserInput) error {
	if s.Store == nil {
		return apperrors.New(apperrors.KindInternal, "users store not configured")
	}
	requesterID, ok := identity.UserID(ctx)
	if !ok || strings.TrimSpace(requesterID) == "" {
		return apperrors.New(apperrors.KindUnauthorized, "unauthorized")
	}
	return s.updateWithTarget(ctx, requesterID, identity.IsAdmin(ctx), requesterID, input)
}

func (s *Service) UpdateByID(ctx context.Context, targetID string, input UpdateUserInput) error {
	if s.Store == nil {
		return apperrors.New(apperrors.KindInternal, "users store not configured")
	}
	requesterID, ok := identity.UserID(ctx)
	if !ok || strings.TrimSpace(requesterID) == "" {
		return apperrors.New(apperrors.KindUnauthorized, "unauthorized")
	}
	if strings.TrimSpace(targetID) == "" {
		return apperrors.New(apperrors.KindInvalidInput, "id is required")
	}
	return s.updateWithTarget(ctx, requesterID, identity.IsAdmin(ctx), targetID, input)
}

func (s *Service) updateWithTarget(ctx context.Context, requesterID string, isAdmin bool, targetID string, input UpdateUserInput) error {
	if requesterID != targetID && !isAdmin {
		return apperrors.New(apperrors.KindForbidden, "forbidden")
	}

	if input.Role != nil && !isAdmin {
		return apperrors.New(apperrors.KindForbidden, "forbidden")
	}

	req := UpdateUserRequest{
		ID: targetID,
	}

	if input.Email != nil {
		email := strings.TrimSpace(strings.ToLower(*input.Email))
		req.Email = email
	}

	if input.Password != nil {
		pass := strings.TrimSpace(*input.Password)
		hasher := s.PasswordHasher
		if hasher == nil {
			hasher = internal.DefaultPasswordHasher
		}
		hash, err := hasher(pass)
		if err != nil {
			return apperrors.New(apperrors.KindInternal, "failed to process password")
		}
		req.PasswordHash = hash
	}

	if input.Role != nil {
		role, err := ParseUserRole(*input.Role)
		if err != nil {
			return apperrors.New(apperrors.KindInvalidInput, "invalid role")
		}
		req.Role = role
	}

	if req.Email == "" && req.PasswordHash == "" && !req.Role.Valid() {
		return nil
	}

	if err := s.Store.Update(ctx, &req); err != nil {
		if IsNotFound(err) {
			return apperrors.New(apperrors.KindNotFound, "user not found")
		}
		return apperrors.New(apperrors.KindInternal, "internal error")
	}

	return nil
}

func (s *Service) DeleteSelf(ctx context.Context) error {
	if s.Store == nil {
		return apperrors.New(apperrors.KindInternal, "users store not configured")
	}
	requesterID, ok := identity.UserID(ctx)
	if !ok || strings.TrimSpace(requesterID) == "" {
		return apperrors.New(apperrors.KindUnauthorized, "unauthorized")
	}
	return s.deleteWithTarget(ctx, requesterID, identity.IsAdmin(ctx), requesterID)
}

func (s *Service) DeleteByID(ctx context.Context, targetID string) error {
	if s.Store == nil {
		return apperrors.New(apperrors.KindInternal, "users store not configured")
	}
	requesterID, ok := identity.UserID(ctx)
	if !ok || strings.TrimSpace(requesterID) == "" {
		return apperrors.New(apperrors.KindUnauthorized, "unauthorized")
	}
	if strings.TrimSpace(targetID) == "" {
		return apperrors.New(apperrors.KindInvalidInput, "id is required")
	}
	return s.deleteWithTarget(ctx, requesterID, identity.IsAdmin(ctx), targetID)
}

func (s *Service) deleteWithTarget(ctx context.Context, requesterID string, isAdmin bool, targetID string) error {
	if requesterID != targetID && !isAdmin {
		return apperrors.New(apperrors.KindForbidden, "forbidden")
	}

	if err := s.Store.Delete(ctx, targetID); err != nil {
		if IsNotFound(err) {
			return apperrors.New(apperrors.KindNotFound, "user not found")
		}
		return apperrors.New(apperrors.KindInternal, "failed to delete user")
	}
	return nil
}
