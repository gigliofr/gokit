// Package authx contiene le primitive di autenticazione comuni: JWT
// access/refresh firmati HS256, hash delle password con bcrypt e token casuali.
package authx

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// TokenType distingue i token di accesso da quelli di rinnovo.
type TokenType string

const (
	Access  TokenType = "access"
	Refresh TokenType = "refresh"
)

// MinSecretLength è la lunghezza minima del segreto HMAC.
const MinSecretLength = 32

var (
	// ErrInvalidToken indica firma, formato o scadenza non validi.
	ErrInvalidToken = errors.New("token non valido o scaduto")
	// ErrWrongTokenType indica un token valido ma del tipo sbagliato
	// (es. refresh usato come access).
	ErrWrongTokenType = errors.New("tipo di token non valido")
)

// BaseClaims va incorporato nelle claims dell'applicazione: aggiunge il tipo
// di token ("typ") e le claims registrate (sub, exp, iat, iss, jti).
//
//	type Claims struct {
//		UserID string `json:"uid"`
//		Role   string `json:"role"`
//		authx.BaseClaims
//	}
type BaseClaims struct {
	Type TokenType `json:"typ"`
	jwt.RegisteredClaims
}

func (b *BaseClaims) base() *BaseClaims { return b }

// Claims è soddisfatta da qualsiasi puntatore a struct che incorpora BaseClaims.
type Claims interface {
	jwt.Claims
	base() *BaseClaims
}

// TokenManager emette e verifica JWT HS256 con claims di tipo C.
type TokenManager[C Claims] struct {
	secret     []byte
	issuer     string
	accessTTL  time.Duration
	refreshTTL time.Duration
	newClaims  func() C
	now        func() time.Time
}

// Option configura il TokenManager.
type Option func(*options)

type options struct {
	issuer     string
	accessTTL  time.Duration
	refreshTTL time.Duration
	now        func() time.Time
}

// WithIssuer imposta "iss" in emissione e lo richiede in verifica.
func WithIssuer(iss string) Option { return func(o *options) { o.issuer = iss } }

// WithTTL imposta la durata dei token di accesso e di rinnovo.
func WithTTL(access, refresh time.Duration) Option {
	return func(o *options) { o.accessTTL, o.refreshTTL = access, refresh }
}

// WithClock sostituisce l'orologio (per i test).
func WithClock(now func() time.Time) Option { return func(o *options) { o.now = now } }

// NewTokenManager crea un TokenManager. newClaims deve restituire un puntatore
// nuovo e vuoto alle claims dell'applicazione (es. func() *Claims { return &Claims{} }).
// Default: access 15 minuti, refresh 7 giorni.
func NewTokenManager[C Claims](secret string, newClaims func() C, opts ...Option) (*TokenManager[C], error) {
	if len(secret) < MinSecretLength {
		return nil, fmt.Errorf("il segreto JWT deve avere almeno %d caratteri", MinSecretLength)
	}
	if newClaims == nil {
		return nil, errors.New("newClaims è obbligatorio")
	}
	o := options{accessTTL: 15 * time.Minute, refreshTTL: 7 * 24 * time.Hour, now: time.Now}
	for _, fn := range opts {
		fn(&o)
	}
	return &TokenManager[C]{
		secret:     []byte(secret),
		issuer:     o.issuer,
		accessTTL:  o.accessTTL,
		refreshTTL: o.refreshTTL,
		newClaims:  newClaims,
		now:        o.now,
	}, nil
}

// AccessTTL restituisce la durata dei token di accesso.
func (m *TokenManager[C]) AccessTTL() time.Duration { return m.accessTTL }

// RefreshTTL restituisce la durata dei token di rinnovo.
func (m *TokenManager[C]) RefreshTTL() time.Duration { return m.refreshTTL }

// Issue firma claims come token di tipo typ. Imposta tipo, iat, exp e iss;
// per i refresh genera un jti se assente. Subject, se vuoto, resta vuoto.
func (m *TokenManager[C]) Issue(claims C, typ TokenType) (string, error) {
	b := claims.base()
	now := m.now().UTC()
	ttl := m.accessTTL
	if typ == Refresh {
		ttl = m.refreshTTL
		if strings.TrimSpace(b.ID) == "" {
			b.ID = uuid.NewString()
		}
	}
	b.Type = typ
	b.IssuedAt = jwt.NewNumericDate(now)
	b.ExpiresAt = jwt.NewNumericDate(now.Add(ttl))
	if m.issuer != "" {
		b.Issuer = m.issuer
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.secret)
}

// IssuePair emette access e refresh token. build viene chiamata due volte
// e deve restituire ogni volta claims nuove (non condivise).
func (m *TokenManager[C]) IssuePair(build func() C) (access, refresh string, err error) {
	if access, err = m.Issue(build(), Access); err != nil {
		return "", "", err
	}
	if refresh, err = m.Issue(build(), Refresh); err != nil {
		return "", "", err
	}
	return access, refresh, nil
}

// Verify controlla firma (solo HS256), scadenza, issuer e tipo atteso.
func (m *TokenManager[C]) Verify(token string, expected TokenType) (C, error) {
	var zero C
	claims := m.newClaims()
	popts := []jwt.ParserOption{
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithExpirationRequired(),
		jwt.WithTimeFunc(m.now),
	}
	if m.issuer != "" {
		popts = append(popts, jwt.WithIssuer(m.issuer))
	}
	parsed, err := jwt.ParseWithClaims(strings.TrimSpace(token), claims, func(*jwt.Token) (any, error) {
		return m.secret, nil
	}, popts...)
	if err != nil || !parsed.Valid {
		return zero, ErrInvalidToken
	}
	if claims.base().Type != expected {
		return zero, ErrWrongTokenType
	}
	return claims, nil
}
