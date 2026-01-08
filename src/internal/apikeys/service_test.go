package apikeys

import (
	"context"
	"errors"
	"testing"

	"github.com/PabloPavan/sniply_api/internal/apperrors"
	"github.com/PabloPavan/sniply_api/internal/identity"
)

type storeStub struct {
	createFn func(ctx context.Context, k *Key) error
	listFn   func(ctx context.Context, userID string) ([]*Key, error)
	getIDFn  func(ctx context.Context, id string) (*Key, error)
	revokeFn func(ctx context.Context, id string) (bool, error)
	getFn    func(ctx context.Context, hash string) (*Key, error)
}

func (s *storeStub) Create(ctx context.Context, k *Key) error {
	if s.createFn != nil {
		return s.createFn(ctx, k)
	}
	return nil
}

func (s *storeStub) ListByUser(ctx context.Context, userID string) ([]*Key, error) {
	if s.listFn != nil {
		return s.listFn(ctx, userID)
	}
	return nil, nil
}

func (s *storeStub) GetByID(ctx context.Context, id string) (*Key, error) {
	if s.getIDFn != nil {
		return s.getIDFn(ctx, id)
	}
	return nil, ErrNotFound
}

func (s *storeStub) Revoke(ctx context.Context, id string) (bool, error) {
	if s.revokeFn != nil {
		return s.revokeFn(ctx, id)
	}
	return false, nil
}

func (s *storeStub) GetByTokenHash(ctx context.Context, hash string) (*Key, error) {
	if s.getFn != nil {
		return s.getFn(ctx, hash)
	}
	return nil, ErrNotFound
}

func TestServiceCreateDefaults(t *testing.T) {
	store := &storeStub{}
	svc := &Service{
		Store:          store,
		IDGenerator:    func() string { return "key_test" },
		TokenGenerator: func() string { return "token" },
		TokenHasher:    func(token string) string { return "hash" },
		TokenPrefixer:  func(token string) string { return "token" },
	}

	var got *Key
	store.createFn = func(ctx context.Context, k *Key) error {
		got = k
		return nil
	}

	ctx := identity.WithUser(context.Background(), "usr_1", "member")
	key, token, err := svc.Create(ctx, CreateInput{Name: "key"})
	if err != nil {
		t.Fatalf("create error: %v", err)
	}
	if token != "token" {
		t.Fatalf("unexpected token: %s", token)
	}
	if key.Scope != ScopeReadWrite {
		t.Fatalf("unexpected scope: %s", key.Scope)
	}
	if got == nil || got.UserID != "usr_1" {
		t.Fatalf("unexpected stored key: %+v", got)
	}
}

func TestServiceCreateInvalidScope(t *testing.T) {
	store := &storeStub{}
	svc := &Service{Store: store}

	ctx := identity.WithUser(context.Background(), "usr_1", "member")
	_, _, err := svc.Create(ctx, CreateInput{Scope: "nope"})
	assertKind(t, err, apperrors.KindInvalidInput)
}

func TestServiceRevokeNotFound(t *testing.T) {
	store := &storeStub{}
	svc := &Service{Store: store}

	store.getIDFn = func(ctx context.Context, id string) (*Key, error) {
		return nil, ErrNotFound
	}

	ctx := identity.WithUser(context.Background(), "usr_1", "member")
	err := svc.Revoke(ctx, "key_1")
	assertKind(t, err, apperrors.KindNotFound)
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
