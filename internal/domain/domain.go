package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"path"
	"strings"
	"time"
)

var (
	ErrNotFound  = errors.New("not found")
	ErrConflict  = errors.New("conflicting state")
	ErrForbidden = errors.New("access denied")
	ErrLease     = errors.New("compilation lease lost")
)

type Principal struct {
	Tenant, Subject string
	Scopes          map[string]bool
	Admin           bool
}
type Space struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Role     string `json:"role"`
	Revision string `json:"revision"`
}
type Record struct {
	ID       string    `json:"id"`
	Session  string    `json:"session"`
	Role     string    `json:"role"`
	Text     string    `json:"text"`
	Occurred time.Time `json:"occurred"`
	Sequence int64     `json:"sequence"`
}
type Page struct {
	Path      string    `json:"path"`
	Content   string    `json:"content"`
	Citations []string  `json:"citations"`
	Hash      string    `json:"hash"`
	Vector    []float32 `json:"vector,omitempty"`
}
type Observation struct {
	Subject    string   `json:"subject"`
	Text       string   `json:"text"`
	Origin     string   `json:"origin"`
	Status     string   `json:"status"`
	Citations  []string `json:"citations"`
	Supersedes []string `json:"supersedes,omitempty"`
}
type Snapshot struct {
	Revision  string `json:"revision"`
	Watermark int64  `json:"watermark"`
	Pages     []Page `json:"pages"`
}
type Lease struct {
	Tenant, Space, Parent   string
	Fence, Watermark, Epoch int64
}
type ModelConfig struct {
	EmbeddingProvider   string `json:"embedding_provider"`
	EmbeddingEndpoint   string `json:"embedding_endpoint,omitempty"`
	EmbeddingKey        string `json:"embedding_key,omitempty"`
	EmbeddingProject    string `json:"embedding_project,omitempty"`
	EmbeddingLocation   string `json:"embedding_location,omitempty"`
	EmbeddingDimensions int    `json:"embedding_dimensions,omitempty"`

	RetrievalPrompt string `json:"retrieval_prompt,omitempty"`
	Provider        string `json:"provider"`
	Endpoint        string `json:"endpoint"`
	Model           string `json:"model"`
	EmbeddingModel  string `json:"embedding_model"`
	MaxSteps        int    `json:"max_steps"`
	MaxTokens       int    `json:"max_tokens"`
	DailyTokens     int64  `json:"daily_tokens"`
	Prompt          string `json:"prompt,omitempty"`
	Key             string `json:"key,omitempty"`
}
type Usage struct {
	Input     int64 `json:"input_tokens"`
	Output    int64 `json:"output_tokens"`
	Estimated bool  `json:"estimated"`
}

func (u Usage) Total() int64 { return u.Input + u.Output }
func Hash(b []byte) string   { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
func ValidPath(p string) bool {
	return p != "" && !strings.ContainsAny(p, "\\\x00") && !strings.HasPrefix(p, "/") && path.Clean(p) == p && p != ".." && !strings.HasPrefix(p, "../")
}
func ValidID(s string) bool {
	if len(s) < 1 || len(s) > 128 {
		return false
	}
	for _, r := range s {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_') {
			return false
		}
	}
	return true
}

func (c ModelConfig) EmbeddingIdentity() string {
	return c.EmbeddingProvider + ":" + c.EmbeddingModel + ":" + fmt.Sprint(c.EmbeddingDimensions)
}
