package authx

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type appClaims struct {
	UserID string `json:"uid"`
	Role   string `json:"role"`
	BaseClaims
}

const secret = "0123456789abcdef0123456789abcdef"

func newTM(t *testing.T, opts ...Option) *TokenManager[*appClaims] {
	t.Helper()
	tm, err := NewTokenManager(secret, func() *appClaims { return &appClaims{} }, opts...)
	if err != nil {
		t.Fatal(err)
	}
	return tm
}

func TestNewTokenManagerValidation(t *testing.T) {
	if _, err := NewTokenManager("corto", func() *appClaims { return &appClaims{} }); err == nil {
		t.Fatal("segreto corto deve fallire")
	}
	if _, err := NewTokenManager[*appClaims](secret, nil); err == nil {
		t.Fatal("newClaims nil deve fallire")
	}
}

func TestIssueVerify(t *testing.T) {
	tm := newTM(t, WithIssuer("test-app"), WithTTL(time.Hour, 48*time.Hour))
	build := func() *appClaims {
		c := &appClaims{UserID: "u1", Role: "admin"}
		c.Subject = "u1"
		return c
	}
	access, refresh, err := tm.IssuePair(build)
	if err != nil {
		t.Fatal(err)
	}
	c, err := tm.Verify(access, Access)
	if err != nil || c.UserID != "u1" || c.Role != "admin" || c.Issuer != "test-app" || c.Subject != "u1" {
		t.Fatalf("access: %v %+v", err, c)
	}
	if c.ExpiresAt.Sub(c.IssuedAt.Time) != time.Hour {
		t.Fatal("TTL access errato")
	}
	r, err := tm.Verify(refresh, Refresh)
	if err != nil || r.ID == "" || r.ExpiresAt.Sub(r.IssuedAt.Time) != 48*time.Hour {
		t.Fatalf("refresh: %v %+v", err, r)
	}
	if _, err := tm.Verify(refresh, Access); !errors.Is(err, ErrWrongTokenType) {
		t.Fatalf("refresh come access: %v", err)
	}
	if _, err := tm.Verify(access+"x", Access); !errors.Is(err, ErrInvalidToken) {
		t.Fatal("firma alterata")
	}
	other := newTM(t, WithIssuer("altra-app"))
	if _, err := other.Verify(access, Access); !errors.Is(err, ErrInvalidToken) {
		t.Fatal("issuer diverso deve fallire")
	}
}

func TestWireFormat(t *testing.T) {
	// Il formato JSON deve restare compatibile con i token già emessi dai progetti
	// (uid/role/typ + claims registrate).
	tm := newTM(t)
	tok, _ := tm.Issue(&appClaims{UserID: "u1"}, Access)
	parts := strings.Split(tok, ".")
	payload, _ := jwt.NewParser().DecodeSegment(parts[1])
	for _, k := range []string{`"uid":"u1"`, `"typ":"access"`, `"exp":`, `"iat":`} {
		if !strings.Contains(string(payload), k) {
			t.Fatalf("payload %s senza %s", payload, k)
		}
	}
}

func TestExpiredAndAlgorithms(t *testing.T) {
	now := time.Now()
	tm := newTM(t, WithClock(func() time.Time { return now }), WithTTL(time.Minute, time.Hour))
	tok, _ := tm.Issue(&appClaims{}, Access)
	now = now.Add(2 * time.Minute)
	if _, err := tm.Verify(tok, Access); !errors.Is(err, ErrInvalidToken) {
		t.Fatal("token scaduto accettato")
	}
	// alg "none" e HS512 non sono ammessi
	none, _ := jwt.NewWithClaims(jwt.SigningMethodNone, &appClaims{BaseClaims: BaseClaims{Type: Access}}).SignedString(jwt.UnsafeAllowNoneSignatureType)
	if _, err := tm.Verify(none, Access); err == nil {
		t.Fatal("alg none accettato")
	}
	hs512, _ := jwt.NewWithClaims(jwt.SigningMethodHS512, &appClaims{BaseClaims: BaseClaims{Type: Access, RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour))}}}).SignedString([]byte(secret))
	if _, err := tm.Verify(hs512, Access); err == nil {
		t.Fatal("HS512 accettato")
	}
	// token senza exp rifiutato
	noexp, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, &appClaims{BaseClaims: BaseClaims{Type: Access}}).SignedString([]byte(secret))
	if _, err := tm.Verify(noexp, Access); err == nil {
		t.Fatal("token senza scadenza accettato")
	}
}

func TestPassword(t *testing.T) {
	if _, err := HashPassword("corta"); !errors.Is(err, ErrPasswordTooShort) {
		t.Fatal("password corta")
	}
	if _, err := HashPassword(strings.Repeat("a", 73)); !errors.Is(err, ErrPasswordTooLong) {
		t.Fatal("password lunga")
	}
	h, err := HashPassword("Password123!")
	if err != nil {
		t.Fatal(err)
	}
	if !CheckPassword(h, "Password123!") || CheckPassword(h, "sbagliata") || CheckPassword("", "x") {
		t.Fatal("CheckPassword")
	}
}

func TestSecureToken(t *testing.T) {
	a, _ := SecureToken(0)
	b, _ := SecureToken(32)
	if len(a) != 32 || len(b) != 43 || a == b {
		t.Fatalf("lunghezze %d %d", len(a), len(b))
	}
}
