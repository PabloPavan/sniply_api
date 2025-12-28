package snippets

import (
	"context"
	"time"
)

type Cache interface {
	GetByID(ctx context.Context, id string) (*Snippet, bool, error)
	SetByID(ctx context.Context, s *Snippet, ttl time.Duration) error
	DeleteByID(ctx context.Context, id string) error
	GetList(ctx context.Context, key string) ([]*Snippet, bool, error)
	SetList(ctx context.Context, key string, snippets []*Snippet, ttl time.Duration) error
}
