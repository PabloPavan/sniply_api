package apikeys

import (
	"context"
	"strings"

	"github.com/PabloPavan/sniply_api/internal/db"
)

type Repository struct {
	base *db.Base
}

func NewRepository(base *db.Base) *Repository {
	return &Repository{base: base}
}

const (
	sqlKeyInsert = `INSERT INTO api_keys (id, user_id, name, scope, token_hash, token_prefix)
		VALUES ($1, $2, $3, $4, $5, $6)`

	sqlKeyListByUser = `SELECT id, user_id, name, scope, token_prefix, created_at, revoked_at
		FROM api_keys
		WHERE user_id = $1
		ORDER BY created_at DESC`

	sqlKeyGetByID = `SELECT id, user_id, name, scope, token_prefix, created_at, revoked_at
		FROM api_keys
		WHERE id = $1`

	sqlKeyGetByHash = `SELECT k.id, k.user_id, k.name, k.scope, k.token_prefix, k.created_at, k.revoked_at, u.role
		FROM api_keys k
		JOIN users u ON u.id = k.user_id
		WHERE k.token_hash = $1`

	sqlKeyRevoke = `UPDATE api_keys
		SET revoked_at = now()
		WHERE id = $1`
)

func (r *Repository) Create(ctx context.Context, k *Key) error {
	ctx, cancel := r.base.WithTimeout(ctx)
	defer cancel()

	row := r.base.Q().QueryRow(ctx, sqlKeyInsert+" RETURNING created_at", k.ID, k.UserID, k.Name, k.Scope, k.TokenHash, k.TokenPrefix)
	if err := row.Scan(&k.CreatedAt); err != nil {
		return err
	}
	return nil
}

func (r *Repository) ListByUser(ctx context.Context, userID string) ([]*Key, error) {
	ctx, cancel := r.base.WithTimeout(ctx)
	defer cancel()

	rows, err := r.base.Q().Query(ctx, sqlKeyListByUser, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*Key
	for rows.Next() {
		var k Key
		if err := rows.Scan(&k.ID, &k.UserID, &k.Name, &k.Scope, &k.TokenPrefix, &k.CreatedAt, &k.RevokedAt); err != nil {
			return nil, err
		}
		out = append(out, &k)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *Repository) GetByID(ctx context.Context, id string) (*Key, error) {
	ctx, cancel := r.base.WithTimeout(ctx)
	defer cancel()

	var k Key
	err := r.base.Q().QueryRow(ctx, sqlKeyGetByID, id).Scan(
		&k.ID,
		&k.UserID,
		&k.Name,
		&k.Scope,
		&k.TokenPrefix,
		&k.CreatedAt,
		&k.RevokedAt,
	)
	if IsNotFound(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &k, nil
}

func (r *Repository) GetByTokenHash(ctx context.Context, hash string) (*Key, error) {
	ctx, cancel := r.base.WithTimeout(ctx)
	defer cancel()

	var k Key
	err := r.base.Q().QueryRow(ctx, sqlKeyGetByHash, strings.TrimSpace(hash)).Scan(
		&k.ID,
		&k.UserID,
		&k.Name,
		&k.Scope,
		&k.TokenPrefix,
		&k.CreatedAt,
		&k.RevokedAt,
		&k.UserRole,
	)
	if IsNotFound(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &k, nil
}

func (r *Repository) Revoke(ctx context.Context, id string) (bool, error) {
	ctx, cancel := r.base.WithTimeout(ctx)
	defer cancel()

	tag, err := r.base.Q().Exec(ctx, sqlKeyRevoke, id)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}
