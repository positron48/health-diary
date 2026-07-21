package crypto

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"

	"golang.org/x/crypto/chacha20poly1305"
)

const nonceSize = chacha20poly1305.NonceSizeX

type Cipher struct {
	key     []byte
	version int
}

func New(encodedKey string, version int) (*Cipher, error) {
	key, err := base64.StdEncoding.DecodeString(encodedKey)
	if err != nil || len(key) != chacha20poly1305.KeySize {
		return nil, fmt.Errorf("DATA_ENCRYPTION_KEY must be base64 encoded 32 bytes")
	}
	if version < 1 {
		return nil, fmt.Errorf("encryption key version must be positive")
	}
	return &Cipher{key: key, version: version}, nil
}

func (c *Cipher) Version() int { return c.version }

func (c *Cipher) Encrypt(plaintext, additionalData []byte) ([]byte, error) {
	aead, err := chacha20poly1305.NewX(c.key)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, nonceSize)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	return append(nonce, aead.Seal(nil, nonce, plaintext, additionalData)...), nil
}

func (c *Cipher) Decrypt(ciphertext, additionalData []byte) ([]byte, error) {
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext is too short")
	}
	aead, err := chacha20poly1305.NewX(c.key)
	if err != nil {
		return nil, err
	}
	return aead.Open(nil, ciphertext[:nonceSize], ciphertext[nonceSize:], additionalData)
}
