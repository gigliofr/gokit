// Package mw raccoglie i middleware HTTP comuni, compatibili con net/http,
// chi e gorilla/mux (firma func(http.Handler) http.Handler).
package mw

import (
	"fmt"
	"net/http"
)

// SecurityOptions configura SecurityHeaders. I campi vuoti usano i default.
type SecurityOptions struct {
	// FrameOptions: "DENY" (default) o "SAMEORIGIN"; "-" per non inviarlo.
	FrameOptions string
	// ReferrerPolicy: default "strict-origin-when-cross-origin"; "-" per non inviarlo.
	ReferrerPolicy string
	// ContentSecurityPolicy: inviata solo se non vuota.
	ContentSecurityPolicy string
	// PermissionsPolicy: inviata solo se non vuota.
	PermissionsPolicy string
	// HSTSMaxAge in secondi: se >0 invia Strict-Transport-Security
	// (solo su richieste HTTPS, anche dietro proxy con X-Forwarded-Proto=https).
	HSTSMaxAge int
}

// SecurityHeaders imposta gli header di sicurezza standard.
func SecurityHeaders(o SecurityOptions) func(http.Handler) http.Handler {
	frame := pick(o.FrameOptions, "DENY")
	referrer := pick(o.ReferrerPolicy, "strict-origin-when-cross-origin")
	hsts := ""
	if o.HSTSMaxAge > 0 {
		hsts = fmt.Sprintf("max-age=%d; includeSubDomains", o.HSTSMaxAge)
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := w.Header()
			h.Set("X-Content-Type-Options", "nosniff")
			if frame != "" {
				h.Set("X-Frame-Options", frame)
			}
			if referrer != "" {
				h.Set("Referrer-Policy", referrer)
			}
			if o.ContentSecurityPolicy != "" {
				h.Set("Content-Security-Policy", o.ContentSecurityPolicy)
			}
			if o.PermissionsPolicy != "" {
				h.Set("Permissions-Policy", o.PermissionsPolicy)
			}
			if hsts != "" && (r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https") {
				h.Set("Strict-Transport-Security", hsts)
			}
			next.ServeHTTP(w, r)
		})
	}
}

func pick(v, def string) string {
	switch v {
	case "":
		return def
	case "-":
		return ""
	default:
		return v
	}
}
