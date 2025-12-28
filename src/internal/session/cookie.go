package session

import (
	"net/http"
	"time"
)

type CookieConfig struct {
	Name     string
	Path     string
	Domain   string
	Secure   bool
	SameSite http.SameSite
}

func (c CookieConfig) Write(w http.ResponseWriter, value string, expiresAt time.Time) {
	path := c.Path
	if path == "" {
		path = "/"
	}
	name := c.Name
	if name == "" {
		name = "sniply_session"
	}

	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     path,
		Domain:   c.Domain,
		Expires:  expiresAt,
		MaxAge:   int(time.Until(expiresAt).Seconds()),
		Secure:   c.Secure,
		HttpOnly: true,
		SameSite: c.SameSite,
	})
}

func (c CookieConfig) Clear(w http.ResponseWriter) {
	path := c.Path
	if path == "" {
		path = "/"
	}
	name := c.Name
	if name == "" {
		name = "sniply_session"
	}

	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     path,
		Domain:   c.Domain,
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		Secure:   c.Secure,
		HttpOnly: true,
		SameSite: c.SameSite,
	})
}
