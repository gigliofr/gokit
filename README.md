# gokit

Librerie Go condivise dai progetti **heroquest-studio**, **mana-wise**, **qr-menu**, **quota-zero** e **visto-easy**.
Raccolgono le logiche che finora erano state riscritte, con piccole differenze, in ogni progetto.

```
go get github.com/gigliofr/gokit@v0.2.0
```

Ogni sottopackage è indipendente: si importa solo ciò che serve e si compilano solo le relative dipendenze.

| Package  | Contenuto | Sostituisce |
|----------|-----------|-------------|
| `envx`   | `String`, `First`, `Int`, `Int64`, `Bool`, `Duration`, `List`, `Required`, `LoadDotEnv` | `getEnv`/`envInt`/`envOr`/`firstNonEmpty`/`loadDotEnv` sparsi nei `main.go` |
| `mongox` | `Connect` (timeout, ping sul primario, chiusura se il ping fallisce), `Disconnect`, `Ping`, `EnsureIndexes` | le 5 versioni di `mongo.Connect` + ping |
| `httpx`  | `JSON`, `Error`, `ErrorCode`, `Decode` (limite 1 MiB, campi sconosciuti rifiutati), `BearerToken`, `ClientIP`, `NewServer` (timeout anti-slowloris), `Run` (graceful shutdown) | `writeJSON`/`writeErr`/`decodeJSON`/`extractBearer` e avvio server |
| `mw`     | `SecurityHeaders`, `CORS` (allowlist con `Vary: Origin`), `RateLimit`/`Limiter` (token bucket per chiave), `RealIP`, `Recover` (con hook per gli avvisi) | i middleware scritti a mano, i rate limiter e `chi/middleware.RealIP` |
| `authx`  | `TokenManager` generico JWT HS256 access/refresh (verifica algoritmo, scadenza, issuer, tipo; `jti` sui refresh), `HashPassword`, `CheckPassword`, `SecureToken` | i 3 `TokenManager` e gli helper bcrypt |
| `mailx`  | `Sender` con `Resend` (API HTTP, senza SDK), `SMTP` (STARTTLS o TLS su 465, header anti-injection, Message-ID), `Noop`, `Log`; `FromEnv` sceglie il provider | i 3 client SMTP e l'SDK Resend |
| `notifyx`| `Telegram`, `Multi`, `Ops` (avvisi operativi: avvio, arresto, errori, panic, `Watch` sulle dipendenze, con deduplica) | il client Telegram di quota-zero; avvisi prima assenti |

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

## Email

```go
mailer, why := mailx.FromEnv(mailx.Options{DefaultSubject: "Notifica", SubjectPrefix: "[MiaApp] "})
log.Println(why) // "email via Resend" | "email via SMTP host:587" | "email disattivate: ..."
err := mailer.Send("utente@example.com", "Benvenuto", "", "<p>Ciao!</p>") // il testo è ricavato dall'HTML
```

| Variabile | Uso |
|-----------|-----|
| `RESEND_API_KEY`, `RESEND_FROM_EMAIL` | Resend (ha la precedenza) |
| `SMTP_HOST`, `SMTP_PORT` (587), `SMTP_USER`, `SMTP_PASSWORD`/`SMTP_PASS`/`SMTP_KEY`, `MAIL_FROM` | SMTP |

## Avvisi operativi e Telegram

Ogni app crea un `notifyx.Ops` con `OpsFromEnv("nome-app")` e invia su Telegram:
🟢 avvio, 🟡 arresto, 🔴 errori e dipendenze giù (con ✅ al ritorno), 💥 panic.
Lo stesso avviso non viene ripetuto per 15 minuti.

```go
ops := notifyx.OpsFromEnv("mia-app")              // OPS_TELEGRAM_BOT_TOKEN, OPS_TELEGRAM_CHAT_ID, ENVIRONMENT
ops.Watch(ctx, "MongoDB", time.Minute, store.Ping) // avvisa ai cambi di stato
r.Use(mw.Recover(ops.PanicHook()))
ops.Started(version)
// ...
ops.Stopping("SIGTERM")
```

Per ricevere tutto in un unico posto usa **lo stesso bot e la stessa chat configurati in
Uptime Kuma**: Kuma segnala se il sito è raggiungibile dall'esterno, `Ops` cosa succede
dentro l'app (Mongo giù, panic, riavvii). Le variabili `OPS_*` sono separate da
`TELEGRAM_*`, che le app usano per i messaggi agli utenti (es. il gruppo di quota-zero).

Per creare il bot: @BotFather → `/newbot` → token; aggiungi il bot alla chat o al gruppo,
scrivi un messaggio e leggi il `chat.id` da `https://api.telegram.org/bot<TOKEN>/getUpdates`.

## IP del client dietro proxy

`httpx.ClientIP(r, true)` e `mw.RealIP` usano `CF-Connecting-IP` (Cloudflare) e poi
l'**ultimo** valore di `X-Forwarded-For`. Il primo valore lo scrive il client: usarlo
(come fa `chi/middleware.RealIP`) permette di aggirare rate limit e blocchi per IP.

## Sviluppo

```
go test -race ./...
MONGOX_TEST_URI=mongodb://localhost:27017 go test ./mongox   # test di integrazione
```

Versioni: semver con tag `vX.Y.Z`. Fino a `v1` le API possono cambiare tra versioni minori.

## Adozione nei progetti

| Progetto | Package usati |
|----------|---------------|
| quota-zero | tutti |
| visto-easy | authx, envx, httpx, mailx, mongox, mw, notifyx |
| heroquest-studio | envx, httpx, mongox, mw, notifyx |
| mana-wise | mailx, mongox, mw (RealIP), notifyx — JWT e formato errori restano propri (contratto API) |
| qr-menu | httpx, mongox, notifyx |

Prossimi package previsti: `storagex` (file locali e S3/MinIO), `authx/totp`.

## Versioni

- **v0.2.0**: `mailx`, `notifyx`, `mw.RealIP`, `mw.Recover`, `CORSOptions.RejectDisallowedPreflight`;
  `httpx.ClientIP` usa l'ultimo X-Forwarded-For (prima il primo, falsificabile);
  `httpx.NewServer` senza `ReadTimeout` (upload lenti); go.sum ripulito.
- **v0.1.0**: envx, mongox, httpx, mw, authx.
