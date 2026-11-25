# gokit

Librerie Go condivise dai progetti **heroquest-studio**, **mana-wise**, **qr-menu**, **quota-zero** e **visto-easy**.
Raccolgono le logiche che finora erano state riscritte, con piccole differenze, in ogni progetto.

```
go get github.com/gigliofr/gokit@v0.1.0
```

Ogni sottopackage è indipendente: si importa solo ciò che serve e si compilano solo le relative dipendenze.

| Package  | Contenuto | Sostituisce |
|----------|-----------|-------------|
| `envx`   | `String`, `First`, `Int`, `Int64`, `Bool`, `Duration`, `List`, `Required`, `LoadDotEnv` | `getEnv`/`envInt`/`envOr`/`firstNonEmpty`/`loadDotEnv` sparsi nei `main.go` |
| `mongox` | `Connect` (timeout, ping sul primario, chiusura se il ping fallisce), `Disconnect`, `Ping`, `EnsureIndexes` | le 5 versioni di `mongo.Connect` + ping |
| `httpx`  | `JSON`, `Error`, `ErrorCode`, `Decode` (limite 1 MiB, campi sconosciuti rifiutati), `BearerToken`, `ClientIP`, `NewServer` (timeout anti-slowloris), `Run` (graceful shutdown) | `writeJSON`/`writeErr`/`decodeJSON`/`extractBearer` e avvio server |
| `mw`     | `SecurityHeaders`, `CORS` (allowlist con `Vary: Origin`), `RateLimit`/`Limiter` (token bucket per chiave con pulizia delle chiavi inattive) | i middleware scritti a mano e i 6 rate limiter |
| `authx`  | `TokenManager` generico JWT HS256 access/refresh (verifica algoritmo, scadenza, issuer, tipo; `jti` sui refresh), `HashPassword`, `CheckPassword`, `SecureToken` | i 3 `TokenManager` e gli helper bcrypt |

## Esempi

```go
// main.go
_ = envx.LoadDotEnv(".env")
client, err := mongox.Connect(ctx, envx.First("MONGODB_URI", "MONGO_URL"), mongox.WithAppName("mia-app"))

r := chi.NewRouter()
r.Use(mw.CORS(mw.CORSOptions{AllowedOrigins: envx.List("CORS_ALLOWED_ORIGINS", nil)}))
r.Use(mw.SecurityHeaders(mw.SecurityOptions{HSTSMaxAge: 31536000}))
r.With(mw.RateLimit(mw.RateLimitOptions{Requests: 10, Window: time.Minute})).Post("/auth/login", login)

ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
defer stop()
log.Fatal(httpx.Run(ctx, httpx.NewServer(":"+envx.String("PORT", "8080"), r), 10*time.Second))
```

```go
// JWT con claims dell'applicazione
type Claims struct {
	UserID string `json:"uid"`
	Role   string `json:"role"`
	authx.BaseClaims // aggiunge "typ" e sub/exp/iat/iss/jti
}

tm, err := authx.NewTokenManager(secret, func() *Claims { return &Claims{} },
	authx.WithIssuer("mia-app"), authx.WithTTL(15*time.Minute, 7*24*time.Hour))
access, refresh, err := tm.IssuePair(func() *Claims { return &Claims{UserID: u.ID, Role: u.Role} })
claims, err := tm.Verify(token, authx.Access) // ErrInvalidToken / ErrWrongTokenType
```

Il formato JSON delle claims resta quello dei progetti (`uid`, `role`, `typ`, …),
quindi i token già emessi restano validi dopo la migrazione.

## Sviluppo

```
go test -race ./...
MONGOX_TEST_URI=mongodb://localhost:27017 go test ./mongox   # test di integrazione
```

Versioni: semver con tag `vX.Y.Z`. Fino a `v1` le API possono cambiare tra versioni minori.

## Adozione nei progetti

| Progetto | Stato |
|----------|-------|
| quota-zero | pilota (branch `refactor/gokit`) |
| visto-easy | da fare: auth, httpapi, middleware, avvio con timeout |
| mana-wise | da fare: unificare i 5 helper di errore JSON e i 2 rate limiter |
| heroquest-studio | da fare: env, mongo, writeJSON, header |
| qr-menu | da fare dopo il riordino interno (3 rate limiter, 2 package di middleware) |

Prossimi package previsti: `mailx` (SMTP/Resend, oggi duplicato tra mana-wise e visto-easy), `storagex`, `authx/totp`.
