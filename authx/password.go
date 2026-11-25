package authx

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

// MinPasswordLength è la lunghezza minima accettata da HashPassword.
const MinPasswordLength = 8

// maxPasswordBytes è il limite di bcrypt: oltre, i byte verrebbero ignorati.
const maxPasswordBytes = 72

var (
	ErrPasswordTooShort = fmt.Errorf("la password deve contenere almeno %d caratteri", MinPasswordLength)
	ErrPasswordTooLong  = fmt.Errorf("la password non può superare %d byte", maxPasswordBytes)
)

// HashPassword restituisce l'hash bcrypt (costo di default) della password.
func HashPassword(password string) (string, error) {
	if len([]rune(password)) < MinPasswordLength {
		return "", ErrPasswordTooShort
	}
	if len(password) > maxPasswordBytes {
		return "", ErrPasswordTooLong
	}
	b, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// CheckPassword confronta in tempo costante la password con l'hash bcrypt.
func CheckPassword(hash, password string) bool {
	if hash == "" {
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// SecureToken restituisce n byte casuali in base64 URL-safe senza padding.
// Valori di n sotto 16 vengono portati a 24 (192 bit).
func SecureToken(n int) (string, error) {
	if n < 16 {
		n = 24
	}
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", errors.New("generatore casuale non disponibile: " + err.Error())
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
