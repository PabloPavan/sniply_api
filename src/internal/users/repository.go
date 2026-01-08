package users

import (
	"context"
	"strconv"
	"strings"

	"github.com/PabloPavan/sniply_api/internal/db"
	"github.com/jackc/pgx/v5/pgconn"
)

type Repository struct {
	base *db.Base
}

func NewRepository(base *db.Base) *Repository {
	return &Repository{base: base}
}

const (
	sqlUserInsert = `INSERT INTO users (id, email, password_hash)
		VALUES ($1, $2, $3)`

	sqlUserList = `SELECT id, email, password_hash, role, created_at
		FROM users
		WHERE email ILIKE $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`

	sqlUserGetByEmail = `SELECT id, email, password_hash, role, created_at
		FROM users
		WHERE email = $1`

	sqlUserGetByID = `SELECT id, email, password_hash, role, created_at
		FROM users
		WHERE id = $1`

	sqlUserUpdateBase = `UPDATE users
		SET %s
		WHERE id = $1`

	sqlUserDelete = `DELETE FROM users 
		WHERE id = $1`
)

func (r *Repository) Create(ctx context.Context, u *User) error {
	ctx, cancel := r.base.WithTimeout(ctx)
	defer cancel()

	row := r.base.Q().QueryRow(ctx, sqlUserInsert+" RETURNING created_at, role", u.ID, u.Email, u.PasswordHash)
	if err := row.Scan(&u.CreatedAt, &u.Role); err != nil {
		return err
	}
	return nil
}

func (r *Repository) GetByEmail(ctx context.Context, email string) (User, error) {
	ctx, cancel := r.base.WithTimeout(ctx)
	defer cancel()

	var u User
	err := r.base.Q().QueryRow(ctx, sqlUserGetByEmail, email).Scan(
		&u.ID, &u.Email, &u.PasswordHash, &u.Role, &u.CreatedAt,
	)
	if IsNotFound(err) {
		return User{}, ErrNotFound
	}

	if err != nil {
		return User{}, err
	}
	return u, nil
}

func (r *Repository) GetByID(ctx context.Context, id string) (*User, error) {
	ctx, cancel := r.base.WithTimeout(ctx)
	defer cancel()

	var u User
	err := r.base.Q().QueryRow(ctx, sqlUserGetByID, id).Scan(
		&u.ID,
		&u.Email,
		&u.PasswordHash,
		&u.Role,
		&u.CreatedAt,
	)

	if IsNotFound(err) {
		return nil, ErrNotFound
	}

	if err != nil {
		return nil, err
	}

	return &u, nil
}

func (r *Repository) List(ctx context.Context, f UserFilter) ([]*User, error) {
	ctx, cancel := r.base.WithTimeout(ctx)
	defer cancel()

	q := "%"
	if strings.TrimSpace(f.Query) != "" {
		q = "%" + strings.ReplaceAll(f.Query, "%", "\\%") + "%"
	}

	limit := 100
	if f.Limit > 0 && f.Limit <= 1000 {
		limit = f.Limit
	}
	offset := 0
	if f.Offset > 0 {
		offset = f.Offset
	}

	rows, err := r.base.Q().Query(ctx, sqlUserList, q, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Role, &u.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, &u)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *Repository) Update(ctx context.Context, u *UpdateUserRequest) error {
	set := make([]string, 0, 4)
	args := make([]any, 0, 5)

	args = append(args, u.ID)
	argPos := 2

	if u.Email != "" {
		set = append(set, "email = $"+strconv.Itoa(argPos))
		args = append(args, u.Email)
		argPos++
	}
	if u.PasswordHash != "" {
		set = append(set, "password_hash = $"+strconv.Itoa(argPos))
		args = append(args, u.PasswordHash)
		argPos++
	}
	if u.Role.Valid() {
		set = append(set, "role = $"+strconv.Itoa(argPos))
		args = append(args, u.Role)
		argPos++
	}

	if len(set) == 0 {
		return nil
	}

	query := strings.Replace(sqlUserUpdateBase, "%s", strings.Join(set, ", "), 1)

	ctx, cancel := r.base.WithTimeout(ctx)
	defer cancel()

	var tag pgconn.CommandTag
	var err error

	tag, err = r.base.Q().Exec(ctx, query, args...)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) Delete(ctx context.Context, id string) error {
	ctx, cancel := r.base.WithTimeout(ctx)
	defer cancel()

	tag, err := r.base.Q().Exec(ctx, sqlUserDelete, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
