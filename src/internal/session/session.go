package session

import (
	"context"
	"errors"
	"time"

	"github.com/PabloPavan/sniply_api/internal"
)

var ErrNotFound = errors.New("session not found")

type Session struct {
	ID              string    `json:"id"`
	UserID          string    `json:"user_id"`
	Role            string    `json:"role"`
	CreatedAt       time.Time `json:"created_at"`
	LastRefreshedAt time.Time `json:"last_refreshed_at"`
	ExpiresAt       time.Time `json:"expires_at"`
}

type Store interface {
	Set(ctx context.Context, id string, s Session, ttl time.Duration) error
	Get(ctx context.Context, id string) (*Session, error)
	Delete(ctx context.Context, id string) error
}

type Manager struct {
	Store         Store
	TTL           time.Duration
	MaxAge        time.Duration
	RefreshBefore time.Duration
	IDBytes       int
}

func (m *Manager) Create(ctx context.Context, userID, role string) (*Session, error) {
	if m.Store == nil {
		return nil, errors.New("session store not configured")
	}

	idBytes := m.IDBytes
	if idBytes <= 0 {
		idBytes = 32
	}

	now := time.Now()
	exp := now.Add(m.TTL)
	s := Session{
		ID:              "ses_" + internal.RandomHex(idBytes),
		UserID:          userID,
		Role:            role,
		CreatedAt:       now,
		LastRefreshedAt: now,
		ExpiresAt:       exp,
	}

	if err := m.Store.Set(ctx, s.ID, s, m.TTL); err != nil {
		return nil, err
	}
	return &s, nil
}

func (m *Manager) Get(ctx context.Context, id string) (*Session, error) {
	if m.Store == nil {
		return nil, errors.New("session store not configured")
	}
	sess, err := m.Store.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	m.ensureSessionTimestamps(sess, now)
	if m.MaxAge > 0 && now.After(sess.CreatedAt.Add(m.MaxAge)) {
		_ = m.Store.Delete(ctx, id)
		return nil, ErrNotFound
	}

	return sess, nil
}

func (m *Manager) Delete(ctx context.Context, id string) error {
	if m.Store == nil {
		return errors.New("session store not configured")
	}
	return m.Store.Delete(ctx, id)
}

func (m *Manager) Refresh(ctx context.Context, sess *Session) (*Session, bool, error) {
	if m.Store == nil {
		return nil, false, errors.New("session store not configured")
	}
	if sess == nil {
		return nil, false, errors.New("session not provided")
	}
	if m.TTL <= 0 {
		return sess, false, nil
	}

	now := time.Now()
	m.ensureSessionTimestamps(sess, now)
	if m.MaxAge > 0 && now.After(sess.CreatedAt.Add(m.MaxAge)) {
		_ = m.Store.Delete(ctx, sess.ID)
		return nil, false, ErrNotFound
	}

	if m.RefreshBefore > 0 {
		if time.Until(sess.ExpiresAt) > m.RefreshBefore {
			return sess, false, nil
		}
	}

	exp := now.Add(m.TTL)
	sess.ExpiresAt = exp
	sess.LastRefreshedAt = now

	if err := m.Store.Set(ctx, sess.ID, *sess, m.TTL); err != nil {
		return nil, false, err
	}
	return sess, true, nil
}

func (m *Manager) ensureSessionTimestamps(sess *Session, now time.Time) {
	if sess.CreatedAt.IsZero() {
		if m.TTL > 0 && !sess.ExpiresAt.IsZero() {
			sess.CreatedAt = sess.ExpiresAt.Add(-m.TTL)
			if sess.CreatedAt.After(now) {
				sess.CreatedAt = now
			}
		} else {
			sess.CreatedAt = now
		}
	}
	if sess.LastRefreshedAt.IsZero() {
		sess.LastRefreshedAt = sess.CreatedAt
	}
}
