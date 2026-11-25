package totp

import (
	"strings"
	"testing"
	"time"
)

func TestGenerateAndValidate(t *testing.T) {
	secret, uri, err := Generate(" mario@example.it ", "Visto Easy")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(uri, "otpauth://totp/Visto%20Easy:mario@example.it") || secret == "" {
		t.Fatalf("uri: %s", uri)
	}
	now := time.Unix(1_800_000_000, 0)
	code, err := Code(secret, now)
	if err != nil {
		t.Fatal(err)
	}
	if !ValidateAt(code, secret, now) || !ValidateAt(" "+code+" ", secret, now.Add(30*time.Second)) {
		t.Fatal("codice valido rifiutato")
	}
	if ValidateAt(code, secret, now.Add(5*time.Minute)) {
		t.Fatal("codice scaduto accettato")
	}
	if ValidateAt("000000", secret, now) && code != "000000" {
		t.Fatal("codice errato accettato")
	}
	if Validate("123456", "non-base32!") {
		t.Fatal("segreto non valido")
	}
}

// Vettore di prova RFC 6238 (SHA1, segreto "12345678901234567890").
func TestRFC6238Vector(t *testing.T) {
	const secret = "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"
	code, err := Code(secret, time.Unix(59, 0))
	if err != nil || code != "287082" {
		t.Fatalf("codice %s %v, atteso 287082", code, err)
	}
}
