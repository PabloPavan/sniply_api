package users

import (
	"context"
	"errors"
	"testing"

	"github.com/PabloPavan/sniply_api/internal/apperrors"
	"github.com/PabloPavan/sniply_api/internal/identity"
)

type storeStub struct {
	createFn func(ctx context.Context, u *User) error
	getFn    func(ctx context.Context, id string) (*User, error)
	listFn   func(ctx context.Context, f UserFilter) ([]*User, error)
	updateFn func(ctx context.Context, u *UpdateUserRequest) error
	deleteFn func(ctx context.Context, id string) error
}

func (s *storeStub) Create(ctx context.Context, u *User) error {
	if s.createFn != nil {
		return s.createFn(ctx, u)
	}
	return nil
}

func (s *storeStub) GetByID(ctx context.Context, id string) (*User, error) {
	if s.getFn != nil {
		return s.getFn(ctx, id)
	}
	return nil, ErrNotFound
}

func (s *storeStub) GetByEmail(ctx context.Context, email string) (User, error) {
	return User{}, errors.New("not used")
}

func (s *storeStub) List(ctx context.Context, f UserFilter) ([]*User, error) {
	if s.listFn != nil {
		return s.listFn(ctx, f)
	}
	return nil, nil
}

func (s *storeStub) Update(ctx context.Context, u *UpdateUserRequest) error {
	if s.updateFn != nil {
		return s.updateFn(ctx, u)
	}
	return nil
}

func (s *storeStub) Delete(ctx context.Context, id string) error {
	if s.deleteFn != nil {
		return s.deleteFn(ctx, id)
	}
	return nil
}

func TestServiceCreateUser(t *testing.T) {
	store := &storeStub{}
	svc := &Service{
		Store: store,
		PasswordHasher: func(plain string) (string, error) {
			if plain == "" {
				return "", errors.New("empty")
			}
			return "hash", nil
		},
		IDGenerator: func() string {
			return "usr_test"
		},
	}

	var got *User
	store.createFn = func(ctx context.Context, u *User) error {
		got = u
		return nil
	}

	u, err := svc.Create(context.Background(), CreateUserRequest{
		Email:    "TEST@LOCAL",
		Password: "secret",
	})
	if err != nil {
		t.Fatalf("create user error: %v", err)
	}
	if u.ID != "usr_test" {
		t.Fatalf("unexpected id: %s", u.ID)
	}
	if got == nil || got.Email != "test@local" {
		t.Fatalf("unexpected stored email: %+v", got)
	}
	if got.PasswordHash != "hash" {
		t.Fatalf("unexpected password hash: %s", got.PasswordHash)
	}
}

func TestServiceListRequiresAdmin(t *testing.T) {
	store := &storeStub{}
	svc := &Service{Store: store}

	ctx := identity.WithUser(context.Background(), "usr_1", "member")
	_, err := svc.List(ctx, UserFilter{})
	assertKind(t, err, apperrors.KindForbidden)
}

func TestServiceUpdateSelf(t *testing.T) {
	store := &storeStub{}
	svc := &Service{Store: store}

	var got UpdateUserRequest
	store.updateFn = func(ctx context.Context, u *UpdateUserRequest) error {
		got = *u
		return nil
	}

	ctx := identity.WithUser(context.Background(), "usr_1", "member")
	email := "Updated@Local"
	err := svc.UpdateSelf(ctx, UpdateUserInput{Email: &email})
	if err != nil {
		t.Fatalf("update self error: %v", err)
	}
	if got.ID != "usr_1" {
		t.Fatalf("unexpected target id: %s", got.ID)
	}
	if got.Email != "updated@local" {
		t.Fatalf("unexpected email: %s", got.Email)
	}
}

func TestServiceUpdateRoleForbidden(t *testing.T) {
	store := &storeStub{}
	svc := &Service{Store: store}

	ctx := identity.WithUser(context.Background(), "usr_1", "member")
	role := "admin"
	err := svc.UpdateSelf(ctx, UpdateUserInput{Role: &role})
	assertKind(t, err, apperrors.KindForbidden)
}

func TestServiceDeleteForbidden(t *testing.T) {
	store := &storeStub{}
	svc := &Service{Store: store}

	ctx := identity.WithUser(context.Background(), "usr_1", "member")
	err := svc.DeleteByID(ctx, "usr_2")
	assertKind(t, err, apperrors.KindForbidden)
}

func TestServiceUpdateByIDRequiresTarget(t *testing.T) {
	store := &storeStub{}
	svc := &Service{Store: store}

	ctx := identity.WithUser(context.Background(), "usr_1", "admin")
	err := svc.UpdateByID(ctx, "", UpdateUserInput{})
	assertKind(t, err, apperrors.KindInvalidInput)
}

func assertKind(t *testing.T, err error, kind apperrors.Kind) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error kind %s", kind)
	}
	var appErr *apperrors.Error
	if !errors.As(err, &appErr) {
		t.Fatalf("expected app error, got: %v", err)
	}
	if appErr.Kind != kind {
		t.Fatalf("unexpected kind: %s", appErr.Kind)
	}
}
