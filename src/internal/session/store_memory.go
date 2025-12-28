package session

import (
	"context"
	"sync"
	"time"
)

type MemoryStore struct {
	mu    sync.RWMutex
	items map[string]Session
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		items: make(map[string]Session),
	}
}

func (s *MemoryStore) Set(ctx context.Context, id string, sess Session, ttl time.Duration) error {
	_ = ctx
	s.mu.Lock()
	defer s.mu.Unlock()

	s.items[id] = sess
	return nil
}

func (s *MemoryStore) Get(ctx context.Context, id string) (*Session, error) {
	_ = ctx
	s.mu.RLock()
	sess, ok := s.items[id]
	s.mu.RUnlock()
	if !ok {
		return nil, ErrNotFound
	}
	if time.Now().After(sess.ExpiresAt) {
		s.mu.Lock()
		delete(s.items, id)
		s.mu.Unlock()
		return nil, ErrNotFound
	}
	return &sess, nil
}

func (s *MemoryStore) Delete(ctx context.Context, id string) error {
	_ = ctx
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.items, id)
	return nil
}
