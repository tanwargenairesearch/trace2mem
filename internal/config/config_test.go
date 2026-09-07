package config

import (
	"context"
	"encoding/base64"
	"testing"
)

func TestCredentialsBoundToSpace(t *testing.T) {
	v, e := NewVault(base64.StdEncoding.EncodeToString(make([]byte, 32)))
	if e != nil {
		t.Fatal(e)
	}
	ctx := context.Background()
	b, e := v.Seal(ctx, []byte("secret"), []byte("tenant/space"))
	if e != nil {
		t.Fatal(e)
	}
	if _, e = v.Open(ctx, b, []byte("other/space")); e == nil {
		t.Fatal("cross-space decryption succeeded")
	}
	b[len(b)-1] ^= 1
	if _, e = v.Open(ctx, b, []byte("tenant/space")); e == nil {
		t.Fatal("tamper accepted")
	}
}
