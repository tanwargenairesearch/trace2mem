package config

import (
	kms "cloud.google.com/go/kms/apiv1"
	"cloud.google.com/go/kms/apiv1/kmspb"
	"context"
)

type KMSVault struct {
	Client *kms.KeyManagementClient
	Key    string
}

func (v KMSVault) Seal(ctx context.Context, b, aad []byte) ([]byte, error) {
	r, e := v.Client.Encrypt(ctx, &kmspb.EncryptRequest{Name: v.Key, Plaintext: b, AdditionalAuthenticatedData: aad})
	if e != nil {
		return nil, e
	}
	return r.Ciphertext, nil
}
func (v KMSVault) Open(ctx context.Context, b, aad []byte) ([]byte, error) {
	if len(b) == 0 {
		return nil, nil
	}
	r, e := v.Client.Decrypt(ctx, &kmspb.DecryptRequest{Name: v.Key, Ciphertext: b, AdditionalAuthenticatedData: aad})
	if e != nil {
		return nil, e
	}
	return r.Plaintext, nil
}
