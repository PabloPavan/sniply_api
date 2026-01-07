package apperrors

import "time"

type Kind string

const (
	KindInvalidInput Kind = "invalid_input"
	KindUnauthorized Kind = "unauthorized"
	KindForbidden    Kind = "forbidden"
	KindNotFound     Kind = "not_found"
	KindConflict     Kind = "conflict"
	KindRateLimited  Kind = "rate_limited"
	KindInternal     Kind = "internal"
)

type Error struct {
	Kind       Kind
	Message    string
	Err        error
	RetryAfter time.Duration
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Message != "" {
		return e.Message
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return string(e.Kind)
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func New(kind Kind, msg string) *Error {
	return &Error{Kind: kind, Message: msg}
}

func Wrap(kind Kind, msg string, err error) *Error {
	return &Error{Kind: kind, Message: msg, Err: err}
}

func RateLimit(msg string, retryAfter time.Duration) *Error {
	return &Error{Kind: KindRateLimited, Message: msg, RetryAfter: retryAfter}
}
