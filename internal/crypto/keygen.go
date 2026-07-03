package crypto

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
)

func GenerateKeyBase64() (string, error) {
	key := make([]byte, keySize)
	if _, err := rand.Read(key); err != nil {
		return "", fmt.Errorf("generate key: %w", err)
	}
	return base64.StdEncoding.EncodeToString(key), nil
}
