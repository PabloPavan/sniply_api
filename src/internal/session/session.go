package session

import (
	"context"
	"errors"
	"time"

	"github.com/PabloPavan/sniply_api/internal"
)

var ErrNotFound = errors.New("session not found")

type Session struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Role      string    `json:"role"`
	ExpiresAt time.Time `json:"expires_at"`
}

type Store interface {
	Set(ctx context.Context, id string, s Session, ttl time.Duration) error
	Get(ctx context.Context, id string) (*Session, error)
	Delete(ctx context.Context, id string) error
}

type Manager struct {
	Store   Store
	TTL     time.Duration
	IDBytes int
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
		ID:        "ses_" + internal.RandomHex(idBytes),
		UserID:    userID,
		Role:      role,
		ExpiresAt: exp,
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
	return m.Store.Get(ctx, id)
}

func (m *Manager) Delete(ctx context.Context, id string) error {
	if m.Store == nil {
		return errors.New("session store not configured")
	}
	return m.Store.Delete(ctx, id)
}
