package snippets

import (
	"context"
	"errors"
	"testing"

	"github.com/PabloPavan/sniply_api/internal/apperrors"
	"github.com/PabloPavan/sniply_api/internal/identity"
	"github.com/PabloPavan/sniply_api/internal/users"
)

type storeStub struct {
	createFn func(ctx context.Context, s *Snippet) error
	getFn    func(ctx context.Context, id string) (*Snippet, error)
	listFn   func(ctx context.Context, f SnippetFilter) ([]*Snippet, error)
	updateFn func(ctx context.Context, s *Snippet) error
	deleteFn func(ctx context.Context, id string) error
}

func (s *storeStub) Create(ctx context.Context, sn *Snippet) error {
	if s.createFn != nil {
		return s.createFn(ctx, sn)
	}
	return nil
}

func (s *storeStub) GetByID(ctx context.Context, id string) (*Snippet, error) {
	if s.getFn != nil {
		return s.getFn(ctx, id)
	}
	return nil, ErrNotFound
}

func (s *storeStub) List(ctx context.Context, f SnippetFilter) ([]*Snippet, error) {
	if s.listFn != nil {
		return s.listFn(ctx, f)
	}
	return nil, ErrNotFound
}

func (s *storeStub) Update(ctx context.Context, sn *Snippet) error {
	if s.updateFn != nil {
		return s.updateFn(ctx, sn)
	}
	return nil
}

func (s *storeStub) Delete(ctx context.Context, id string) error {
	if s.deleteFn != nil {
		return s.deleteFn(ctx, id)
	}
	return nil
}

type userStub struct {
	getFn func(ctx context.Context, id string) (*users.User, error)
}

func (u *userStub) GetByID(ctx context.Context, id string) (*users.User, error) {
	if u.getFn != nil {
		return u.getFn(ctx, id)
	}
	return nil, users.ErrNotFound
}

func TestServiceCreateDefaults(t *testing.T) {
	store := &storeStub{}
	svc := &Service{Store: store, IDGenerator: func() string { return "snp_test" }}

	var got *Snippet
	store.createFn = func(ctx context.Context, s *Snippet) error {
		got = s
		return nil
	}

	ctx := identity.WithUser(context.Background(), "usr_1", "member")
	snippet, err := svc.Create(ctx, CreateSnippetRequest{
		Name:    "hello",
		Content: "print('hi')",
	})
	if err != nil {
		t.Fatalf("create error: %v", err)
	}
	if got == nil {
		t.Fatal("snippet not persisted")
	}
	if snippet.Language != "txt" {
		t.Fatalf("expected default language, got %s", snippet.Language)
	}
	if snippet.Visibility != VisibilityPrivate {
		t.Fatalf("expected default visibility, got %s", snippet.Visibility)
	}
	if len(snippet.Tags) != 0 {
		t.Fatalf("expected empty tags, got %v", snippet.Tags)
	}
	if snippet.CreatorID != "usr_1" {
		t.Fatalf("unexpected creator id: %s", snippet.CreatorID)
	}
}

func TestServiceCreateUnauthorized(t *testing.T) {
	store := &storeStub{}
	svc := &Service{Store: store}

	_, err := svc.Create(context.Background(), CreateSnippetRequest{Name: "a", Content: "b"})
	assertKind(t, err, apperrors.KindUnauthorized)
}

func TestServiceListPrivateForbidden(t *testing.T) {
	store := &storeStub{}
	usersSvc := &userStub{getFn: func(ctx context.Context, id string) (*users.User, error) {
		return &users.User{ID: id}, nil
	}}
	svc := &Service{Store: store, Users: usersSvc}

	ctx := identity.WithUser(context.Background(), "usr_1", "member")
	_, err := svc.List(ctx, ListInput{Creator: "usr_2", Visibility: VisibilityPrivate})
	assertKind(t, err, apperrors.KindForbidden)
}

func TestServiceListPrivateAdminOK(t *testing.T) {
	store := &storeStub{}
	usersSvc := &userStub{getFn: func(ctx context.Context, id string) (*users.User, error) {
		return &users.User{ID: id}, nil
	}}
	svc := &Service{Store: store, Users: usersSvc}

	store.listFn = func(ctx context.Context, f SnippetFilter) ([]*Snippet, error) {
		return []*Snippet{{ID: "s1"}}, nil
	}

	ctx := identity.WithUser(context.Background(), "usr_1", "admin")
	list, err := svc.List(ctx, ListInput{Creator: "usr_2", Visibility: VisibilityPrivate})
	if err != nil {
		t.Fatalf("list error: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("unexpected list size: %d", len(list))
	}
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
