package mw

import (
	"net"
	"net/http"

	"github.com/gigliofr/gokit/httpx"
)

// RealIP riscrive r.RemoteAddr con l'IP del client ricavato da
// httpx.ClientIP(r, true): CF-Connecting-IP, poi l'ultimo X-Forwarded-For.
// Sostituisce chi/middleware.RealIP, che usa il primo valore di
// X-Forwarded-For (scelto dal client, quindi falsificabile).
// Da usare solo se l'app è raggiungibile esclusivamente tramite il proxy.
func RealIP(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ip := httpx.ClientIP(r, true); ip != "" {
			r.RemoteAddr = net.JoinHostPort(ip, "0")
		}
		next.ServeHTTP(w, r)
	})
}
