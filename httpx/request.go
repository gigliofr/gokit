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
// Con trustProxy=false usa solo RemoteAddr (corretto quando un middleware come
// chi/middleware.RealIP l'ha già riscritto, o senza proxy davanti).
// Con trustProxy=true usa il primo IP valido di X-Forwarded-For, poi X-Real-IP:
// abilitarlo solo dietro un reverse proxy fidato (Coolify/Traefik, Cloudflare),
// altrimenti il client può falsificare l'header.
func ClientIP(r *http.Request, trustProxy bool) string {
	if trustProxy {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			for _, part := range strings.Split(xff, ",") {
				if ip := net.ParseIP(strings.TrimSpace(part)); ip != nil {
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
