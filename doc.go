// Package gokit raccoglie le logiche comuni ai progetti Go di gigliofr
// (heroquest-studio, mana-wise, qr-menu, quota-zero, visto-easy).
//
// Ogni sottopackage è indipendente: un progetto importa solo ciò che usa e
// compila solo le dipendenze di quei package.
//
//   - envx:   lettura di variabili d'ambiente tipizzate e file .env
//   - mongox: connessione MongoDB con ping, timeout e creazione indici
//   - httpx:  risposte/decodifica JSON, IP client, avvio server con graceful shutdown
//   - mw:     middleware HTTP (header di sicurezza, CORS, rate limit, IP reale, recover)
//   - authx:  JWT access/refresh, hash password bcrypt, token casuali
//   - mailx:  email via Resend o SMTP
//   - notifyx: Telegram e avvisi operativi
package gokit
