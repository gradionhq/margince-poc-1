// Package keyvault is the Tier-0 key-sealing seam (ADR-0048): callers hand it
// plaintext bytes and get back opaque ciphertext plus the key identifier that
// sealed it; a KeyProvider never returns key material itself. The interface is
// the pluggable point — local.go ships the self-contained on-prem
// implementation; a cloud-KMS backend (AWS KMS / GCP KMS) is a future ticket
// implementing the same interface. No domain imports — a leaf, like blobstore.
package keyvault

import "context"

// KeyProvider seals/unseals opaque byte payloads. kmsKeyID identifies which key
// sealed a given ciphertext so Unseal can locate it (a provider may hold more
// than one key across its lifetime, e.g. after a future rewrap).
type KeyProvider interface {
	Seal(ctx context.Context, plaintext []byte) (ciphertext []byte, kmsKeyID string, err error)
	Unseal(ctx context.Context, ciphertext []byte, kmsKeyID string) ([]byte, error)
}
