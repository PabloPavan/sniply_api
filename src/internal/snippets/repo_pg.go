package snippets

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("not found")

type RepoPG struct {
	pool *pgxpool.Pool
}

func NewRepoPG(pool *pgxpool.Pool) *RepoPG {
	return &RepoPG{pool: pool}
}

func (r *RepoPG) Create(ctx context.Context, s *Snippet) error {
	// MVP: IDs como texto. Aqui você pode gerar ULID/UUID no app.
	// Para simplificar, vamos usar timestamp+random depois; por enquanto, deixe o caller gerar.
	const q = `
INSERT INTO snippets (id, name, content, language, tags, visibility, creator_id)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING created_at, updated_at;
`
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	return r.pool.QueryRow(ctx, q,
		s.ID,
		s.Name,
		s.Content,
		s.Language,
		s.Tags,
		string(s.Visibility),
		s.CreatorID,
	).Scan(&s.CreatedAt, &s.UpdatedAt)
}

func (r *RepoPG) GetByIDPublicOnly(ctx context.Context, id string) (*Snippet, error) {
	const q = `
SELECT id, name, content, language, tags, visibility, creator_id, created_at, updated_at
FROM snippets
WHERE id = $1 AND visibility = 'public'
LIMIT 1;
`
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	var s Snippet
	var visibility string
	err := r.pool.QueryRow(ctx, q, id).Scan(
		&s.ID,
		&s.Name,
		&s.Content,
		&s.Language,
		&s.Tags,
		&visibility,
		&s.CreatorID,
		&s.CreatedAt,
		&s.UpdatedAt,
	)
	if err != nil {
		return nil, ErrNotFound
	}
	s.Visibility = Visibility(visibility)
	return &s, nil
}
