package snippets

import "time"

type Visibility string

const (
	VisibilityPublic  Visibility = "public"
	VisibilityPrivate Visibility = "private"
)

type Snippet struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Content    string     `json:"content"`
	Language   string     `json:"language"`
	Tags       []string   `json:"tags"`
	Visibility Visibility `json:"visibility"`

	// MVP sem auth: deixa vazio. Quando entrar auth, preencher.
	CreatorID string `json:"creator_id,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type CreateSnippetRequest struct {
	Name       string     `json:"name"`
	Content    string     `json:"content"`
	Language   string     `json:"language"`
	Tags       []string   `json:"tags"`
	Visibility Visibility `json:"visibility"`
}

type SnippetFilter struct {
	Query    string // full-text or simple substring search
	Creator  string
	Language string
	Limit    int
	Offset   int
}
