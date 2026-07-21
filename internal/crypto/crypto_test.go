package crypto

import (
	"crypto/rand"
	"encoding/base64"
	"testing"
)

func testKey(t *testing.T) string {
	t.Helper()
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	return base64.StdEncoding.EncodeToString(key)
}
func TestRoundTripAndTamperDetection(t *testing.T) {
	c, err := New(testKey(t), 1)
	if err != nil {
		t.Fatal(err)
	}
	sealed, err := c.Encrypt([]byte("SENSITIVE_TEST_MARKER"), []byte("entry-id"))
	if err != nil {
		t.Fatal(err)
	}
	plain, err := c.Decrypt(sealed, []byte("entry-id"))
	if err != nil || string(plain) != "SENSITIVE_TEST_MARKER" {
		t.Fatalf("decrypt: %q %v", plain, err)
	}
	sealed[len(sealed)-1] ^= 1
	if _, err := c.Decrypt(sealed, []byte("entry-id")); err == nil {
		t.Fatal("tampered ciphertext decrypted")
	}
}
