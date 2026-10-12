package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
)

// Encrypt seals plaintext with AES-256-GCM. aad binds the ciphertext to its owner
// (for example the user id) so a value copied to another row will not decrypt.
func Encrypt(key []byte, plaintext, aad string) (string, error) {
	gcm, err := newGCM(key)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	sealed := gcm.Seal(nonce, nonce, []byte(plaintext), []byte(aad))
	return base64.StdEncoding.EncodeToString(sealed), nil
}

func Decrypt(key []byte, encoded, aad string) (string, error) {
	gcm, err := newGCM(key)
	if err != nil {
		return "", err
	}
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", err
	}
	if len(raw) < gcm.NonceSize() {
		return "", errors.New("ciphertext too short")
	}
	nonce, ct := raw[:gcm.NonceSize()], raw[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, ct, []byte(aad))
	if err != nil {
		return "", errors.New("decryption failed")
	}
	return string(plain), nil
}

func newGCM(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}
