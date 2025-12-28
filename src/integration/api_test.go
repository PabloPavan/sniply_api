package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/PabloPavan/sniply_api/internal"
	"github.com/PabloPavan/sniply_api/internal/db"
	"github.com/PabloPavan/sniply_api/internal/httpapi"
	"github.com/PabloPavan/sniply_api/internal/session"
	"github.com/PabloPavan/sniply_api/internal/snippets"
	"github.com/PabloPavan/sniply_api/internal/users"
)

type testEnv struct {
	baseURL  string
	server   *httptest.Server
	users    *users.Repository
	snippets *snippets.Repository
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL not set")
	}

	ctx := context.Background()
	pool, err := db.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("db connect: %v", err)
	}
	t.Cleanup(pool.Close)

	base := db.NewBase(pool.Pool, 3*time.Second)
	snRepo := snippets.NewRepository(base)
	usrRepo := users.NewRepository(base)

	sessionManager := &session.Manager{
		Store:   session.NewMemoryStore(),
		TTL:     5 * time.Minute,
		IDBytes: 16,
	}
	cookieCfg := session.CookieConfig{
		Name: "sniply_session",
		Path: "/",
	}

	app := &httpapi.App{
		Health:   &httpapi.HealthHandler{DB: pool.Pool},
		Snippets: &httpapi.SnippetsHandler{Repo: snRepo, RepoUser: usrRepo},
		Users:    &httpapi.UsersHandler{Repo: usrRepo},
		Auth: &httpapi.AuthHandler{
			Users:    usrRepo,
			Sessions: sessionManager,
			Cookie:   cookieCfg,
		},
	}

	srv := httptest.NewServer(httpapi.NewRouter(app))
	t.Cleanup(srv.Close)

	return &testEnv{
		baseURL:  srv.URL,
		server:   srv,
		users:    usrRepo,
		snippets: snRepo,
	}
}

func newClient(t *testing.T) *http.Client {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookie jar: %v", err)
	}
	return &http.Client{Jar: jar}
}

func createUser(t *testing.T, client *http.Client, baseURL, email, password string) users.UserResponse {
	t.Helper()

	payload := users.CreateUserRequest{Email: email, Password: password}
	res := doJSON(t, client, http.MethodPost, baseURL+"/v1/users", payload)
	defer res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("create user status: %d", res.StatusCode)
	}

	var out users.UserResponse
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatalf("decode create user: %v", err)
	}
	if out.ID == "" {
		t.Fatal("create user missing id")
	}
	return out
}

func login(t *testing.T, client *http.Client, baseURL, email, password string) {
	t.Helper()

	payload := map[string]string{
		"email":    email,
		"password": password,
	}
	res := doJSON(t, client, http.MethodPost, baseURL+"/v1/auth/login", payload)
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("login status: %d", res.StatusCode)
	}

	base, err := url.Parse(baseURL)
	if err != nil {
		t.Fatalf("parse base url: %v", err)
	}
	cookies := client.Jar.Cookies(base)
	found := false
	for _, c := range cookies {
		if c.Name == "sniply_session" && c.Value != "" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("missing session cookie after login")
	}
}

func logout(t *testing.T, client *http.Client, baseURL string) {
	t.Helper()
	res := doJSON(t, client, http.MethodPost, baseURL+"/v1/auth/logout", nil)
	defer res.Body.Close()
	if res.StatusCode != http.StatusNoContent {
		t.Fatalf("logout status: %d", res.StatusCode)
	}
}

func createAdminUser(t *testing.T, env *testEnv, email, password string) {
	t.Helper()

	hash, err := internal.DefaultPasswordHasher(password)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	u := &users.User{
		ID:           "usr_" + internal.RandomHex(12),
		Email:        email,
		PasswordHash: hash,
	}
	if err := env.users.Create(context.Background(), u); err != nil {
		t.Fatalf("create admin user: %v", err)
	}
	t.Cleanup(func() { _ = env.users.Delete(context.Background(), u.ID) })

	role := users.RoleAdmin
	update := &users.UpdateUserRequest{ID: u.ID, Role: role}
	if err := env.users.Update(context.Background(), update); err != nil {
		t.Fatalf("set admin role: %v", err)
	}
}

func doJSON(t *testing.T, client *http.Client, method, url string, body any) *http.Response {
	t.Helper()

	var buf *bytes.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal json: %v", err)
		}
		buf = bytes.NewReader(b)
	} else {
		buf = bytes.NewReader(nil)
	}

	req, err := http.NewRequest(method, url, buf)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	res, err := client.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	return res
}

func TestHealthEndpoint(t *testing.T) {
	env := newTestEnv(t)

	res, err := http.Get(env.baseURL + "/v1/health")
	if err != nil {
		t.Fatalf("health request: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("health status: %d", res.StatusCode)
	}
}

func TestAuthLoginLogout(t *testing.T) {
	env := newTestEnv(t)
	client := newClient(t)

	email := fmt.Sprintf("ci_%s@local", internal.RandomHex(6))
	password := "secret123"
	created := createUser(t, client, env.baseURL, email, password)
	t.Cleanup(func() { _ = env.users.Delete(context.Background(), created.ID) })

	login(t, client, env.baseURL, email, password)

	res := doJSON(t, client, http.MethodGet, env.baseURL+"/v1/users/me", nil)
	_ = res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("me status after login: %d", res.StatusCode)
	}

	logout(t, client, env.baseURL)

	res = doJSON(t, client, http.MethodGet, env.baseURL+"/v1/users/me", nil)
	_ = res.Body.Close()
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("me status after logout: %d", res.StatusCode)
	}
}

func TestUsersEndpoints(t *testing.T) {
	env := newTestEnv(t)
	client := newClient(t)

	email := fmt.Sprintf("ci_%s@local", internal.RandomHex(6))
	password := "secret123"
	user := createUser(t, client, env.baseURL, email, password)

	login(t, client, env.baseURL, email, password)

	res := doJSON(t, client, http.MethodGet, env.baseURL+"/v1/users/me", nil)
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("me status: %d", res.StatusCode)
	}

	var me users.UserResponse
	if err := json.NewDecoder(res.Body).Decode(&me); err != nil {
		t.Fatalf("decode me: %v", err)
	}
	if me.ID != user.ID {
		t.Fatalf("me id mismatch: %s != %s", me.ID, user.ID)
	}

	newEmail := fmt.Sprintf("updated_%s@local", internal.RandomHex(6))
	update := map[string]string{
		"email":    newEmail,
		"password": "newpass123",
	}
	res = doJSON(t, client, http.MethodPut, env.baseURL+"/v1/users/me", update)
	_ = res.Body.Close()
	if res.StatusCode != http.StatusNoContent {
		t.Fatalf("update me status: %d", res.StatusCode)
	}

	res = doJSON(t, client, http.MethodGet, env.baseURL+"/v1/users/me", nil)
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("me status after update: %d", res.StatusCode)
	}

	var meUpdated users.UserResponse
	if err := json.NewDecoder(res.Body).Decode(&meUpdated); err != nil {
		t.Fatalf("decode me updated: %v", err)
	}
	if meUpdated.Email != newEmail {
		t.Fatalf("me email not updated: %s", meUpdated.Email)
	}

	res = doJSON(t, client, http.MethodDelete, env.baseURL+"/v1/users/me", nil)
	_ = res.Body.Close()
	if res.StatusCode != http.StatusNoContent {
		t.Fatalf("delete me status: %d", res.StatusCode)
	}

	res = doJSON(t, client, http.MethodGet, env.baseURL+"/v1/users/me", nil)
	_ = res.Body.Close()
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("me status after delete: %d", res.StatusCode)
	}
}

func TestUsersAdminEndpoints(t *testing.T) {
	env := newTestEnv(t)
	adminClient := newClient(t)
	userClient := newClient(t)

	adminEmail := fmt.Sprintf("admin_%s@local", internal.RandomHex(6))
	adminPassword := "adminpass"
	createAdminUser(t, env, adminEmail, adminPassword)

	login(t, adminClient, env.baseURL, adminEmail, adminPassword)

	userEmail := fmt.Sprintf("user_%s@local", internal.RandomHex(6))
	userPassword := "userpass"
	user := createUser(t, userClient, env.baseURL, userEmail, userPassword)

	res := doJSON(t, userClient, http.MethodGet, env.baseURL+"/v1/users", nil)
	_ = res.Body.Close()
	if res.StatusCode != http.StatusForbidden {
		t.Fatalf("users list status (non-admin): %d", res.StatusCode)
	}

	res = doJSON(t, adminClient, http.MethodGet, env.baseURL+"/v1/users", nil)
	_ = res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("users list status (admin): %d", res.StatusCode)
	}

	updatedEmail := fmt.Sprintf("updated_%s@local", internal.RandomHex(6))
	role := "admin"
	update := map[string]string{
		"email":    updatedEmail,
		"password": "resetpass",
		"role":     role,
	}
	res = doJSON(t, adminClient, http.MethodPut, env.baseURL+"/v1/users/"+user.ID, update)
	_ = res.Body.Close()
	if res.StatusCode != http.StatusNoContent {
		t.Fatalf("admin update status: %d", res.StatusCode)
	}

	updated, err := env.users.GetByID(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("get updated user: %v", err)
	}
	if updated.Email != updatedEmail {
		t.Fatalf("updated email mismatch: %s", updated.Email)
	}
	if updated.Role != users.RoleAdmin {
		t.Fatalf("updated role mismatch: %s", updated.Role)
	}

	res = doJSON(t, adminClient, http.MethodDelete, env.baseURL+"/v1/users/"+user.ID, nil)
	_ = res.Body.Close()
	if res.StatusCode != http.StatusNoContent {
		t.Fatalf("admin delete status: %d", res.StatusCode)
	}

	_, err = env.users.GetByID(context.Background(), user.ID)
	if err == nil || !users.IsNotFound(err) {
		t.Fatalf("expected user to be deleted")
	}
}

func TestSnippetsEndpoints(t *testing.T) {
	env := newTestEnv(t)
	client := newClient(t)

	email := fmt.Sprintf("ci_%s@local", internal.RandomHex(6))
	password := "secret123"
	created := createUser(t, client, env.baseURL, email, password)
	t.Cleanup(func() { _ = env.users.Delete(context.Background(), created.ID) })

	res, err := http.Get(env.baseURL + "/v1/snippets")
	if err != nil {
		t.Fatalf("snippets list no auth: %v", err)
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("snippets list no auth status: %d", res.StatusCode)
	}

	login(t, client, env.baseURL, email, password)

	meRes := doJSON(t, client, http.MethodGet, env.baseURL+"/v1/users/me", nil)
	defer meRes.Body.Close()
	if meRes.StatusCode != http.StatusOK {
		t.Fatalf("me status: %d", meRes.StatusCode)
	}
	var me users.UserResponse
	if err := json.NewDecoder(meRes.Body).Decode(&me); err != nil {
		t.Fatalf("decode me: %v", err)
	}

	createReq := snippets.CreateSnippetRequest{
		Name:       "Example",
		Content:    "print('hi')",
		Language:   "python",
		Tags:       []string{"dev"},
		Visibility: snippets.VisibilityPublic,
	}
	res = doJSON(t, client, http.MethodPost, env.baseURL+"/v1/snippets", createReq)
	defer res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("create snippet status: %d", res.StatusCode)
	}

	var newSnippet snippets.Snippet
	if err := json.NewDecoder(res.Body).Decode(&newSnippet); err != nil {
		t.Fatalf("decode snippet: %v", err)
	}
	if newSnippet.ID == "" {
		t.Fatal("snippet missing id")
	}

	res = doJSON(t, client, http.MethodGet, env.baseURL+"/v1/snippets/"+newSnippet.ID, nil)
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("get snippet status: %d", res.StatusCode)
	}

	res = doJSON(t, client, http.MethodGet, env.baseURL+"/v1/snippets?limit=10", nil)
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("list snippets status: %d", res.StatusCode)
	}

	var list []snippets.Snippet
	if err := json.NewDecoder(res.Body).Decode(&list); err != nil {
		t.Fatalf("decode snippet list: %v", err)
	}
	if len(list) == 0 {
		t.Fatal("expected snippets list")
	}

	updateReq := snippets.CreateSnippetRequest{
		Name:       "Updated",
		Content:    "print('updated')",
		Language:   "python",
		Tags:       []string{"dev", "updated"},
		Visibility: snippets.VisibilityPublic,
	}
	res = doJSON(t, client, http.MethodPut, env.baseURL+"/v1/snippets/"+newSnippet.ID, updateReq)
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("update snippet status: %d", res.StatusCode)
	}

	var updated snippets.Snippet
	if err := json.NewDecoder(res.Body).Decode(&updated); err != nil {
		t.Fatalf("decode updated snippet: %v", err)
	}
	if updated.Name != "Updated" {
		t.Fatalf("snippet name not updated: %s", updated.Name)
	}

	privateReq := snippets.CreateSnippetRequest{
		Name:       "Private",
		Content:    "secret",
		Language:   "txt",
		Visibility: snippets.VisibilityPrivate,
	}
	res = doJSON(t, client, http.MethodPost, env.baseURL+"/v1/snippets", privateReq)
	defer res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("create private snippet status: %d", res.StatusCode)
	}
	var privateSnippet snippets.Snippet
	if err := json.NewDecoder(res.Body).Decode(&privateSnippet); err != nil {
		t.Fatalf("decode private snippet: %v", err)
	}

	privateListURL := fmt.Sprintf("%s/v1/snippets?visibility=private&creator=%s", env.baseURL, url.QueryEscape(me.ID))
	res = doJSON(t, client, http.MethodGet, privateListURL, nil)
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("private snippets list status: %d", res.StatusCode)
	}

	res = doJSON(t, client, http.MethodDelete, env.baseURL+"/v1/snippets/"+newSnippet.ID, nil)
	_ = res.Body.Close()
	if res.StatusCode != http.StatusNoContent {
		t.Fatalf("delete snippet status: %d", res.StatusCode)
	}

	if privateSnippet.ID != "" {
		res = doJSON(t, client, http.MethodDelete, env.baseURL+"/v1/snippets/"+privateSnippet.ID, nil)
		_ = res.Body.Close()
		if res.StatusCode != http.StatusNoContent {
			t.Fatalf("delete private snippet status: %d", res.StatusCode)
		}
	}

	res = doJSON(t, client, http.MethodGet, env.baseURL+"/v1/snippets/"+newSnippet.ID, nil)
	_ = res.Body.Close()
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("get deleted snippet status: %d", res.StatusCode)
	}
}
