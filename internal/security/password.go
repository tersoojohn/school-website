// Package security holds password hashing, field encryption and token helpers.
package security

import (
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"unicode"
)

const (
	scheme     = "pbkdf2-sha256"
	iterations = 600_000 // OWASP guidance for PBKDF2-HMAC-SHA256
	saltLen    = 16
	keyLen     = 32

	MinPasswordLen = 10
	MaxPasswordLen = 128
)

var b64 = base64.RawStdEncoding

// HashPassword returns a self-describing PBKDF2-HMAC-SHA256 hash with a random salt.
func HashPassword(password string) (string, error) {
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	dk, err := pbkdf2.Key(sha256.New, password, salt, iterations, keyLen)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s$%d$%s$%s", scheme, iterations, b64.EncodeToString(salt), b64.EncodeToString(dk)), nil
}

// VerifyPassword checks a password against a stored hash in constant time.
func VerifyPassword(password, encoded string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 4 || parts[0] != scheme {
		return false
	}
	iter, err := strconv.Atoi(parts[1])
	if err != nil || iter < 100_000 || iter > 5_000_000 {
		return false
	}
	salt, err := b64.DecodeString(parts[2])
	if err != nil {
		return false
	}
	want, err := b64.DecodeString(parts[3])
	if err != nil || len(want) == 0 {
		return false
	}
	got, err := pbkdf2.Key(sha256.New, password, salt, iter, len(want))
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare(got, want) == 1
}

var dummyHash, _ = HashPassword("this-is-not-a-real-password-1")

// BurnTime spends the same effort as a real verification. Call it when the username
// does not exist so response time does not reveal which usernames are valid.
func BurnTime(password string) {
	VerifyPassword(password, dummyHash)
}

// ValidatePassword enforces the password policy for user-chosen passwords.
func ValidatePassword(password, username string) error {
	n := len([]rune(password))
	if n < MinPasswordLen {
		return fmt.Errorf("Use at least %d characters.", MinPasswordLen)
	}
	if len(password) > MaxPasswordLen {
		return fmt.Errorf("Use at most %d characters.", MaxPasswordLen)
	}
	var letter, digit bool
	for _, r := range password {
		switch {
		case unicode.IsLetter(r):
			letter = true
		case unicode.IsDigit(r):
			digit = true
		}
	}
	if !letter || !digit {
		return errors.New("Include at least one letter and one number.")
	}
	if username != "" && strings.EqualFold(password, username) {
		return errors.New("Your password cannot be your username.")
	}
	return nil
}

const tempAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz23456789"

// GenerateTempPassword creates a random one-time password for newly registered accounts.
// Users must change it at first sign-in.
func GenerateTempPassword() (string, error) {
	for {
		buf := make([]byte, 12)
		for i := range buf {
			n, err := rand.Int(rand.Reader, big.NewInt(int64(len(tempAlphabet))))
			if err != nil {
				return "", err
			}
			buf[i] = tempAlphabet[n.Int64()]
		}
		pw := string(buf)
		if ValidatePassword(pw, "") == nil {
			return pw, nil
		}
	}
}

// UnusablePassword returns a valid-looking hash of a random secret nobody knows.
// New accounts hold it until the person sets their own password from a setup link,
// so no one (including administrators) ever knows or hands over a password.
func UnusablePassword() (string, error) {
	secret, _, err := NewToken()
	if err != nil {
		return "", err
	}
	return HashPassword(secret)
}
