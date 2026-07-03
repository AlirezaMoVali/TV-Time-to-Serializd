package crypto

import (
	"testing"
)

func TestEncryptDecryptRoundTrip(t *testing.T) {
	keyB64, err := GenerateKeyBase64()
	if err != nil {
		t.Fatal(err)
	}

	c, err := NewFromBase64Key(keyB64)
	if err != nil {
		t.Fatal(err)
	}

	plaintext := "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.test.token"
	encrypted, err := c.Encrypt(plaintext)
	if err != nil {
		t.Fatal(err)
	}

	if string(encrypted) == plaintext {
		t.Fatal("ciphertext should not equal plaintext")
	}

	decrypted, err := c.Decrypt(encrypted)
	if err != nil {
		t.Fatal(err)
	}

	if decrypted != plaintext {
		t.Fatalf("got %q, want %q", decrypted, plaintext)
	}
}

func TestNewFromBase64KeyRejectsInvalidSize(t *testing.T) {
	_, err := NewFromBase64Key("dG9vX3Nob3J0")
	if err == nil {
		t.Fatal("expected error for short key")
	}
}
