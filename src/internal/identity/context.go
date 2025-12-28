package identity

import "context"

type ctxKey string

const (
	ctxUserIDKey ctxKey = "user_id"
	ctxRoleKey   ctxKey = "role"
)

func WithUser(ctx context.Context, userID string, role string) context.Context {
	ctx = context.WithValue(ctx, ctxUserIDKey, userID)
	ctx = context.WithValue(ctx, ctxRoleKey, role)
	return ctx
}

func UserID(ctx context.Context) (string, bool) {
	v := ctx.Value(ctxUserIDKey)
	id, ok := v.(string)
	return id, ok
}

func Role(ctx context.Context) (string, bool) {
	v := ctx.Value(ctxRoleKey)
	role, ok := v.(string)
	return role, ok
}

func IsAdmin(ctx context.Context) bool {
	role, _ := Role(ctx)
	return role == "admin"
}
