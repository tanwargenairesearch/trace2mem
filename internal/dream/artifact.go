package dream

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/trace2mem/trace2mem/internal/domain"
	"unicode/utf8"
)

func (e *Engine) artifact(ctx context.Context, l domain.Lease, args json.RawMessage) (string, error) {
	var a struct {
		ID     string `json:"artifact_id"`
		Offset int    `json:"offset"`
		Limit  int    `json:"limit"`
	}
	if err := json.Unmarshal(args, &a); err != nil {
		return "", err
	}
	if a.Offset < 0 || a.Limit < 1 || a.Limit > 16384 {
		return "", errors.New("artifact range must be 1–16384 bytes with nonnegative offset")
	}
	var key, media string
	err := e.Store.DB.QueryRow(ctx, "SELECT key,media_type FROM artifacts WHERE tenant=$1 AND space=$2 AND id=$3", l.Tenant, l.Space, a.ID).Scan(&key, &media)
	if err != nil {
		return "", err
	}
	if e.Blob == nil {
		return "", errors.New("artifact storage unavailable")
	}
	b, err := e.Blob.Get(ctx, key)
	if err != nil {
		return "", err
	}
	if a.Offset >= len(b) {
		return "", errors.New("artifact offset out of range")
	}
	end := a.Offset + a.Limit
	if end > len(b) {
		end = len(b)
	}
	part := b[a.Offset:end]
	if !utf8.Valid(part) {
		return "", errors.New("artifact range must align to UTF-8 characters")
	}
	return fmt.Sprintf("Artifact %s bytes [%d,%d), type %s:\n%s", a.ID, a.Offset, end, media, string(part)), nil
}
