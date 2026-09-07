package config

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"os"
	"strings"
)

type Config struct {
	DatabaseURL, Addr, BlobDir, BootstrapToken, MasterKey, AllowedEndpoints, OIDCIssuer, OIDCClient, OIDCSecret, PublicURL, OAuthAudience, GCSBucket, KMSKey string
	Scripted                                                                                                                                                 bool
}

func Load() Config {
	get := func(k, d string) string {
		if s := os.Getenv(k); s != "" {
			return s
		}
		return d
	}
	return Config{DatabaseURL: get("DATABASE_URL", "postgres://brain:brain@localhost:5432/brain?sslmode=disable"), Addr: ":" + get("PORT", "8080"), BlobDir: get("BLOB_DIR", ".local/blobs"), BootstrapToken: os.Getenv("BRAIN_BOOTSTRAP_TOKEN"), MasterKey: os.Getenv("BRAIN_MASTER_KEY"), AllowedEndpoints: os.Getenv("BRAIN_MODEL_ENDPOINTS"), OIDCIssuer: os.Getenv("OIDC_ISSUER"), OIDCClient: os.Getenv("OIDC_CLIENT_ID"), OIDCSecret: os.Getenv("OIDC_CLIENT_SECRET"), PublicURL: get("PUBLIC_URL", "http://localhost:8080"), OAuthAudience: os.Getenv("OAUTH_AUDIENCE"), GCSBucket: os.Getenv("GCS_BUCKET"), KMSKey: os.Getenv("KMS_KEY"), Scripted: os.Getenv("BRAIN_ALLOW_SCRIPTED") == "true"}
}

type Vault interface {
	Seal(context.Context, []byte, []byte) ([]byte, error)
	Open(context.Context, []byte, []byte) ([]byte, error)
}
type LocalVault struct{ aead cipher.AEAD }

func NewVault(key string) (Vault, error) {
	b, e := base64.StdEncoding.DecodeString(key)
	if e != nil || len(b) != 32 {
		return nil, errors.New("BRAIN_MASTER_KEY must be a base64 encoded 32-byte key")
	}
	c, e := aes.NewCipher(b)
	if e != nil {
		return nil, e
	}
	a, e := cipher.NewGCM(c)
	return &LocalVault{a}, e
}
func (v *LocalVault) Seal(ctx context.Context, b, aad []byte) ([]byte, error) {
	n := make([]byte, v.aead.NonceSize())
	if _, e := rand.Read(n); e != nil {
		return nil, e
	}
	return v.aead.Seal(n, n, b, aad), nil
}
func (v *LocalVault) Open(ctx context.Context, b, aad []byte) ([]byte, error) {
	if len(b) == 0 {
		return nil, nil
	}
	n := v.aead.NonceSize()
	if len(b) < n {
		return nil, errors.New("invalid encrypted credential")
	}
	return v.aead.Open(nil, b[:n], b[n:], aad)
}
func (c Config) EndpointAllowed(provider, endpoint string) bool {
	if endpoint == "" {
		return provider == "openai" || provider == "scripted" || provider == "gemini" || provider == "vertex"
	}
	if provider == "openai" && strings.TrimRight(endpoint, "/") == "https://api.openai.com/v1" {
		return true
	}
	for _, s := range strings.Split(c.AllowedEndpoints, ",") {
		if s != "" && strings.TrimRight(s, "/") == strings.TrimRight(endpoint, "/") {
			return true
		}
	}
	return false
}
