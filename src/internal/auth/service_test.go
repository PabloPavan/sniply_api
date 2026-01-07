package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/PabloPavan/sniply_api/internal/apperrors"
	"github.com/PabloPavan/sniply_api/internal/session"
	"github.com/PabloPavan/sniply_api/internal/users"
)

type userStoreStub struct {
	getFn func(ctx context.Context, email string) (users.User, error)
}

func (u *userStoreStub) GetByEmail(ctx context.Context, email string) (users.User, error) {
	if u.getFn != nil {
		return u.getFn(ctx, email)
	}
	return users.User{}, users.ErrNotFound
}

type sessionStub struct {
	createFn  func(ctx context.Context, userID, role string) (*session.Session, error)
	getFn     func(ctx context.Context, id string) (*session.Session, error)
	refreshFn func(ctx context.Context, sess *session.Session) (*session.Session, bool, error)
	deleteFn  func(ctx context.Context, id string) error
}

func (s *sessionStub) Create(ctx context.Context, userID, role string) (*session.Session, error) {
	if s.createFn != nil {
		return s.createFn(ctx, userID, role)
	}
	return nil, errors.New("not implemented")
}

func (s *sessionStub) Get(ctx context.Context, id string) (*session.Session, error) {
	if s.getFn != nil {
		return s.getFn(ctx, id)
	}
	return nil, session.ErrNotFound
}

func (s *sessionStub) Refresh(ctx context.Context, sess *session.Session) (*session.Session, bool, error) {
	if s.refreshFn != nil {
		return s.refreshFn(ctx, sess)
	}
	return sess, false, nil
}

func (s *sessionStub) Delete(ctx context.Context, id string) error {
	if s.deleteFn != nil {
		return s.deleteFn(ctx, id)
	}
	return nil
}

func TestServiceLoginInvalidEmail(t *testing.T) {
	store := &userStoreStub{}
	sessions := &sessionStub{}
	svc := &Service{Users: store, Sessions: sessions}

	_, err := svc.Login(context.Background(), LoginInput{Email: "invalid", Password: "x"})
	assertKind(t, err, apperrors.KindInvalidInput)
}

func TestServiceLoginSuccess(t *testing.T) {
	store := &userStoreStub{}
	sessions := &sessionStub{}

	store.getFn = func(ctx context.Context, email string) (users.User, error) {
		return users.User{ID: "usr_1", Email: "user@local", PasswordHash: "hash", Role: users.RoleAdmin}, nil
	}

	expiresAt := time.Now().Add(time.Hour)
	sessions.createFn = func(ctx context.Context, userID, role string) (*session.Session, error) {
		return &session.Session{
			ID:        "ses_1",
			UserID:    userID,
			Role:      role,
			CSRFToken: "csrf",
			ExpiresAt: expiresAt,
		}, nil
	}

	svc := &Service{
		Users:    store,
		Sessions: sessions,
		PasswordVerifier: func(hashed, plain string) error {
			if hashed != "hash" || plain != "pass" {
				return errors.New("mismatch")
			}
			return nil
		},
	}

	res, err := svc.Login(context.Background(), LoginInput{Email: "USER@LOCAL", Password: "pass"})
	if err != nil {
		t.Fatalf("login error: %v", err)
	}
	if res.UserID != "usr_1" {
		t.Fatalf("unexpected user id: %s", res.UserID)
	}
	if res.Session.CSRFToken != "csrf" {
		t.Fatalf("unexpected csrf token: %s", res.Session.CSRFToken)
	}
}

func TestServiceAuthenticateSessionForbidden(t *testing.T) {
	sessions := &sessionStub{}
	svc := &Service{Sessions: sessions}

	sessions.getFn = func(ctx context.Context, id string) (*session.Session, error) {
		return &session.Session{ID: id, UserID: "usr_1", Role: "member", CSRFToken: "csrf"}, nil
	}
	sessions.refreshFn = func(ctx context.Context, sess *session.Session) (*session.Session, bool, error) {
		return sess, false, nil
	}

	_, _, err := svc.AuthenticateSession(context.Background(), "ses_1", "bad", "POST")
	assertKind(t, err, apperrors.KindForbidden)
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
