// Package totp gestisce i codici a tempo (RFC 6238) per l'autenticazione a due
// fattori, compatibili con Google Authenticator, Aegis, 1Password e simili.
package totp

import (
	"strings"
	"time"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

// Generate crea un nuovo segreto per account (es. l'email) e restituisce il
// segreto base32 da salvare e l'URI otpauth:// da mostrare come QR code.
func Generate(account, issuer string) (secret, uri string, err error) {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      strings.TrimSpace(issuer),
		AccountName: strings.TrimSpace(account),
	})
	if err != nil {
		return "", "", err
	}
	return key.Secret(), key.URL(), nil
}

// Validate verifica un codice a 6 cifre (periodo 30s, tolleranza ±1 periodo).
func Validate(code, secret string) bool { return ValidateAt(code, secret, time.Now()) }

// ValidateAt è Validate all'istante t (per i test).
func ValidateAt(code, secret string, t time.Time) bool {
	ok, err := totp.ValidateCustom(strings.TrimSpace(code), strings.TrimSpace(secret), t.UTC(),
		totp.ValidateOpts{Period: 30, Skew: 1, Digits: otp.DigitsSix, Algorithm: otp.AlgorithmSHA1})
	return err == nil && ok
}

// Code genera il codice valido all'istante t (per i test e gli strumenti di supporto).
func Code(secret string, t time.Time) (string, error) {
	return totp.GenerateCodeCustom(strings.TrimSpace(secret), t.UTC(),
		totp.ValidateOpts{Period: 30, Skew: 1, Digits: otp.DigitsSix, Algorithm: otp.AlgorithmSHA1})
}
