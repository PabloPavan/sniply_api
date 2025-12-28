package snippets

import (
	"context"
	"fmt"
	"strings"

	"github.com/PabloPavan/sniply_api/internal"
	"github.com/PabloPavan/sniply_api/internal/db"
)

type Repository struct {
	base *db.Base
}

func NewRepository(base *db.Base) *Repository {
	return &Repository{base: base}
}

const (
	sqlSnippetInsert = `INSERT INTO snippets (id, name, content, language, tags, visibility, creator_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING created_at, updated_at;`

	sqlSnippetSelectByID = `SELECT id, name, content, language, tags, visibility, creator_id, created_at, updated_at
		FROM snippets
		WHERE id = $1 AND visibility = 'public'
		LIMIT 1;`

	sqlSnippetListBase = `SELECT id, name, content, language, tags, visibility, creator_id, created_at, updated_at
		FROM snippets
		WHERE %s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d;`

	sqlSnippetUpdate = `UPDATE snippets
		SET name = $1, content = $2, language = $3, tags = $4, visibility = $5, updated_at = now()
		WHERE id = $6
		RETURNING updated_at;`

	sqlSnippetDelete = `DELETE FROM snippets 
		WHERE id = $1 AND creator_id = $2;`
)

func (r *Repository) Create(ctx context.Context, s *Snippet) error {
	ctx, cancel := r.base.WithTimeout(ctx)
	defer cancel()

	return r.base.Q().QueryRow(ctx, sqlSnippetInsert,
		s.ID,
		s.Name,
		s.Content,
		s.Language,
		s.Tags,
		string(s.Visibility),
		s.CreatorID,
	).Scan(&s.CreatedAt, &s.UpdatedAt)
}

func (r *Repository) GetByIDPublicOnly(ctx context.Context, id string) (*Snippet, error) {

	ctx, cancel := r.base.WithTimeout(ctx)
	defer cancel()

	var s Snippet
	var visibility string
	err := r.base.Q().QueryRow(ctx, sqlSnippetSelectByID, id).Scan(
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
		return nil, internal.ErrNotFound
	}
	s.Visibility = Visibility(visibility)
	return &s, nil
}

func (r *Repository) List(ctx context.Context, f SnippetFilter) ([]*Snippet, error) {
	where := []string{"1=1"}
	args := make([]any, 0, 8)
	argPos := 1

	if f.Creator != "" {
		where = append(where, fmt.Sprintf("creator_id = $%d", argPos))
		args = append(args, f.Creator)
		argPos++
	}
	if f.Language != "" {
		where = append(where, fmt.Sprintf("language = $%d", argPos))
		args = append(args, f.Language)
		argPos++
	}
	if f.Query != "" {
		where = append(where, fmt.Sprintf("((search_tsv @@ plainto_tsquery('simple', $%d)) OR (name %% $%d) OR (similarity(name, $%d) > 0.25))", argPos, argPos, argPos))
		qstr := strings.TrimSpace(f.Query)
		args = append(args, qstr)
		argPos += 1
	}
	if len(f.Tags) > 0 {
		where = append(where, fmt.Sprintf("tags && $%d", argPos))
		args = append(args, f.Tags)
		argPos++
	}

	if f.Visibility != "" {
		where = append(where, fmt.Sprintf("visibility = $%d", argPos))
		args = append(args, string(f.Visibility))
		argPos++
	}

	limit := 100
	if f.Limit > 0 && f.Limit <= 1000 {
		limit = f.Limit
	}

	offset := max(f.Offset, 0)

	limitPos := argPos
	offsetPos := argPos + 1
	args = append(args, limit, offset)

	query := fmt.Sprintf(sqlSnippetListBase, strings.Join(where, " AND "), limitPos, offsetPos)

	ctx, cancel := r.base.WithTimeout(ctx)
	defer cancel()

	rows, err := r.base.Q().Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	snippets := make([]*Snippet, 0, min(limit, 128))
	for rows.Next() {
		var s Snippet
		var visibility string
		if err := rows.Scan(
			&s.ID,
			&s.Name,
			&s.Content,
			&s.Language,
			&s.Tags,
			&visibility,
			&s.CreatorID,
			&s.CreatedAt,
			&s.UpdatedAt,
		); err != nil {
			return nil, err
		}
		s.Visibility = Visibility(visibility)
		snippets = append(snippets, &s)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return snippets, nil
}

func (r *Repository) Update(ctx context.Context, s *Snippet) error {

	ctx, cancel := r.base.WithTimeout(ctx)
	defer cancel()

	return r.base.Q().QueryRow(ctx, sqlSnippetUpdate,
		s.Name,
		s.Content,
		s.Language,
		s.Tags,
		string(s.Visibility),
		s.ID,
	).Scan(&s.UpdatedAt)
}

func (r *Repository) Delete(ctx context.Context, id string, creatorID string) error {
	ctx, cancel := r.base.WithTimeout(ctx)
	defer cancel()

	tag, err := r.base.Q().Exec(ctx, sqlSnippetDelete, id, creatorID)

	if err != nil {
		return err
	}

	if tag.RowsAffected() == 0 {
		return internal.ErrNotFound
	}

	return nil
}
