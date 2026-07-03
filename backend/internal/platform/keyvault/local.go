package keyvault

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
)

// LocalProvider is the ADR-0048 self-contained on-prem KeyProvider: a single
// AES-256-GCM key held in process memory, sourced from KEYVAULT_MASTER_KEY at
// startup (see config.go). Ciphertext layout is nonce||sealed (GCM appends its
// own auth tag), so Unseal needs only the ciphertext + the kmsKeyID it was
// sealed under.
type LocalProvider struct {
	key      []byte // 32 bytes, AES-256
	kmsKeyID string // sha256(key) hex-encoded, stable per key — never the key itself
}

// NewLocalProvider returns a LocalProvider keyed by masterKey, which must be
// exactly 32 bytes (AES-256). The provider's kmsKeyID is a deterministic
// fingerprint of the key (sha256), so Unseal can detect a key mismatch.
func NewLocalProvider(masterKey []byte) (*LocalProvider, error) {
	if len(masterKey) != 32 {
		return nil, fmt.Errorf("keyvault: master key must be 32 bytes, got %d", len(masterKey))
	}
	sum := sha256.Sum256(masterKey)
	return &LocalProvider{key: masterKey, kmsKeyID: hex.EncodeToString(sum[:])}, nil
}

// Seal encrypts plaintext with AES-256-GCM under a fresh random nonce,
// prepended to the returned ciphertext. kmsKeyID is this provider's key
// fingerprint, not secret material.
func (p *LocalProvider) Seal(_ context.Context, plaintext []byte) (ciphertext []byte, kmsKeyID string, err error) {
	block, err := aes.NewCipher(p.key)
	if err != nil {
		return nil, "", fmt.Errorf("keyvault: new cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, "", fmt.Errorf("keyvault: new gcm: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, "", fmt.Errorf("keyvault: nonce: %w", err)
	}
	sealed := gcm.Seal(nonce, nonce, plaintext, nil)
	return sealed, p.kmsKeyID, nil
}

// Unseal decrypts ciphertext produced by Seal. It fails closed: a kmsKeyID
// mismatch (wrong provider/key) or a corrupted/tampered ciphertext (GCM auth
// tag check) both return a non-nil error, never a best-effort partial result.
func (p *LocalProvider) Unseal(_ context.Context, ciphertext []byte, kmsKeyID string) ([]byte, error) {
	if kmsKeyID != p.kmsKeyID {
		return nil, fmt.Errorf("keyvault: kmsKeyID %q does not match this provider's key", kmsKeyID)
	}
	block, err := aes.NewCipher(p.key)
	if err != nil {
		return nil, fmt.Errorf("keyvault: new cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("keyvault: new gcm: %w", err)
	}
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("keyvault: ciphertext too short")
	}
	nonce, sealed := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, sealed, nil)
	if err != nil {
		return nil, fmt.Errorf("keyvault: unseal: %w", err)
	}
	return plaintext, nil
}
