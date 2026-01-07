package httpapi

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/PabloPavan/sniply_api/internal/apperrors"
)

func writeAppError(w http.ResponseWriter, err error) {
	if err == nil {
		return
	}

	var appErr *apperrors.Error
	if errors.As(err, &appErr) {
		if appErr.Kind == apperrors.KindRateLimited && appErr.RetryAfter > 0 {
			seconds := int(appErr.RetryAfter.Seconds())
			if seconds <= 0 {
				seconds = 1
			}
			w.Header().Set("Retry-After", strconv.Itoa(seconds))
		}
		http.Error(w, errorMessage(appErr), statusFromKind(appErr.Kind))
		return
	}

	http.Error(w, "internal error", http.StatusInternalServerError)
}

func statusFromKind(kind apperrors.Kind) int {
	switch kind {
	case apperrors.KindInvalidInput:
		return http.StatusBadRequest
	case apperrors.KindUnauthorized:
		return http.StatusUnauthorized
	case apperrors.KindForbidden:
		return http.StatusForbidden
	case apperrors.KindNotFound:
		return http.StatusNotFound
	case apperrors.KindConflict:
		return http.StatusConflict
	case apperrors.KindRateLimited:
		return http.StatusTooManyRequests
	default:
		return http.StatusInternalServerError
	}
}

func errorMessage(appErr *apperrors.Error) string {
	if appErr == nil {
		return "internal error"
	}
	if appErr.Message != "" {
		return appErr.Message
	}
	switch appErr.Kind {
	case apperrors.KindUnauthorized:
		return "unauthorized"
	case apperrors.KindForbidden:
		return "forbidden"
	case apperrors.KindNotFound:
		return "not found"
	case apperrors.KindConflict:
		return "conflict"
	case apperrors.KindRateLimited:
		return "too many requests"
	case apperrors.KindInvalidInput:
		return "invalid request"
	default:
		return "internal error"
	}
}
