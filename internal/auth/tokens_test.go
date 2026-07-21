package auth

import "testing"

func TestTokensAreOpaqueAndHashesMatch(t *testing.T) {
	token, hash, err := NewOpaqueToken()
	if err != nil || len(token) < 40 || string(hash) != string(Hash(token)) {
		t.Fatalf("invalid token result")
	}
	code, codeHash, err := NewCode()
	if err != nil || len(code) != 6 || string(codeHash) != string(Hash(code)) {
		t.Fatalf("invalid code result: %q", code)
	}
}
