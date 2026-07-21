package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
)

func NewOpaqueToken() (string, []byte, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", nil, err
	}
	token := base64.RawURLEncoding.EncodeToString(bytes)
	return token, Hash(token), nil
}

func NewCode() (string, []byte, error) {
	bytes := make([]byte, 4)
	if _, err := rand.Read(bytes); err != nil {
		return "", nil, err
	}
	value := int(bytes[0])<<24 | int(bytes[1])<<16 | int(bytes[2])<<8 | int(bytes[3])
	code := fmt.Sprintf("%06d", value%1_000_000)
	return code, Hash(code), nil
}

func Hash(value string) []byte { sum := sha256.Sum256([]byte(value)); return sum[:] }
