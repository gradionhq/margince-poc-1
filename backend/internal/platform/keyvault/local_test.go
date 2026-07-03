package keyvault_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"testing"

	"github.com/gradionhq/margince/backend/internal/platform/keyvault"
)

func randomKey(t *testing.T) []byte {
	t.Helper()
	k := make([]byte, 32)
	if _, err := rand.Read(k); err != nil {
		t.Fatalf("rand: %v", err)
	}
	return k
}

func TestLocalProvider_SealUnseal_RoundTrip(t *testing.T) {
	p, err := keyvault.NewLocalProvider(randomKey(t))
	if err != nil {
		t.Fatalf("NewLocalProvider: %v", err)
	}
	plaintext := []byte("hubspot-refresh-token-abc123")

	ciphertext, kmsKeyID, err := p.Seal(context.Background(), plaintext)
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}
	if bytes.Contains(ciphertext, plaintext) {
		t.Fatalf("ciphertext must not contain the plaintext verbatim")
	}
	if kmsKeyID == "" {
		t.Fatalf("kmsKeyID must be non-empty")
	}

	got, err := p.Unseal(context.Background(), ciphertext, kmsKeyID)
	if err != nil {
		t.Fatalf("Unseal: %v", err)
	}
	if !bytes.Equal(got, plaintext) {
		t.Fatalf("Unseal = %q, want %q", got, plaintext)
	}
}

func TestLocalProvider_Seal_NonDeterministic(t *testing.T) {
	// AES-GCM uses a random nonce per call; two seals of the same plaintext
	// must not produce identical ciphertext (nonce reuse would be a real bug).
	p, err := keyvault.NewLocalProvider(randomKey(t))
	if err != nil {
		t.Fatalf("NewLocalProvider: %v", err)
	}
	pt := []byte("same-plaintext")
	c1, _, err := p.Seal(context.Background(), pt)
	if err != nil {
		t.Fatalf("Seal 1: %v", err)
	}
	c2, _, err := p.Seal(context.Background(), pt)
	if err != nil {
		t.Fatalf("Seal 2: %v", err)
	}
	if bytes.Equal(c1, c2) {
		t.Fatalf("two seals of the same plaintext produced identical ciphertext (nonce reuse)")
	}
}

func TestLocalProvider_Unseal_WrongKeyFails(t *testing.T) {
	p1, err := keyvault.NewLocalProvider(randomKey(t))
	if err != nil {
		t.Fatalf("NewLocalProvider p1: %v", err)
	}
	p2, err := keyvault.NewLocalProvider(randomKey(t))
	if err != nil {
		t.Fatalf("NewLocalProvider p2: %v", err)
	}
	ciphertext, kmsKeyID, err := p1.Seal(context.Background(), []byte("secret"))
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}
	if _, err := p2.Unseal(context.Background(), ciphertext, kmsKeyID); err == nil {
		t.Fatalf("Unseal with the wrong key must fail, got nil error")
	}
}

func TestNewLocalProvider_RejectsWrongKeyLength(t *testing.T) {
	if _, err := keyvault.NewLocalProvider(make([]byte, 16)); err == nil {
		t.Fatalf("NewLocalProvider must reject a non-32-byte key")
	}
}
