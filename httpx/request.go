package httpx

import (
	"net"
	"net/http"
	"strings"
)

// BearerToken estrae il token da "Authorization: Bearer <token>" ("" se assente).
func BearerToken(r *http.Request) string {
	h := strings.TrimSpace(r.Header.Get("Authorization"))
	if len(h) > 7 && strings.EqualFold(h[:7], "Bearer ") {
		return strings.TrimSpace(h[7:])
	}
	return ""
}

// ClientIP restituisce l'IP del client.
//
// Con trustProxy=false usa solo RemoteAddr (corretto senza proxy davanti o dopo
// mw.RealIP, che riscrive RemoteAddr).
//
// Con trustProxy=true, in ordine:
//  1. CF-Connecting-IP, impostato da Cloudflare (che sovrascrive quello del client);
//  2. l'ultimo IP valido di X-Forwarded-For, cioè quello aggiunto dal proxy più
//     vicino: i primi valori della lista li scrive il client e sono falsificabili;
//  3. X-Real-IP;
//  4. RemoteAddr.
//
// Abilitarlo solo se l'app è raggiungibile esclusivamente tramite il proxy
// (Cloudflare → Traefik di Coolify): chi la raggiunge direttamente può
// impostare questi header a piacere.
func ClientIP(r *http.Request, trustProxy bool) string {
	if trustProxy {
		if ip := net.ParseIP(strings.TrimSpace(r.Header.Get("CF-Connecting-IP"))); ip != nil {
			return ip.String()
		}
		if xff := r.Header.Values("X-Forwarded-For"); len(xff) > 0 {
			parts := strings.Split(strings.Join(xff, ","), ",")
			for i := len(parts) - 1; i >= 0; i-- {
				if ip := net.ParseIP(strings.TrimSpace(parts[i])); ip != nil {
					return ip.String()
				}
			}
		}
		if ip := net.ParseIP(strings.TrimSpace(r.Header.Get("X-Real-IP"))); ip != nil {
			return ip.String()
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	return strings.TrimSpace(host)
}
