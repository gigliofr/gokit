package mw

import (
	"net/http"
	"strconv"
	"strings"
)

// CORSOptions configura CORS.
type CORSOptions struct {
	// AllowedOrigins: origini ammesse (es. "https://app.example.com").
	// "*" ammette qualsiasi origine, ma non è compatibile con AllowCredentials.
	AllowedOrigins []string
	// AllowedMethods: default GET, POST, PUT, PATCH, DELETE, OPTIONS.
	AllowedMethods []string
	// AllowedHeaders: default Authorization, Content-Type.
	AllowedHeaders []string
	// ExposedHeaders: header leggibili dal browser nella risposta.
	ExposedHeaders []string
	// AllowCredentials invia Access-Control-Allow-Credentials: true.
	AllowCredentials bool
	// MaxAge in secondi per la cache del preflight (0 = non inviato).
	MaxAge int
}

// CORS gestisce le richieste cross-origin. Con una lista di origini rimanda
// l'Origin della richiesta solo se ammessa (con Vary: Origin); le richieste
// OPTIONS di preflight ricevono 204 senza raggiungere l'handler.
func CORS(o CORSOptions) func(http.Handler) http.Handler {
	allowAll := false
	allowed := make(map[string]struct{}, len(o.AllowedOrigins))
	for _, origin := range o.AllowedOrigins {
		origin = strings.TrimRight(strings.TrimSpace(origin), "/")
		if origin == "*" {
			allowAll = true
		} else if origin != "" {
			allowed[strings.ToLower(origin)] = struct{}{}
		}
	}
	if allowAll && o.AllowCredentials {
		// "*" con credenziali è vietato dai browser: si passa al rimando dell'origine.
		allowAll = false
		o.AllowedOrigins = nil
	}
	methods := strings.Join(orDefault(o.AllowedMethods, []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"}), ", ")
	headers := strings.Join(orDefault(o.AllowedHeaders, []string{"Authorization", "Content-Type"}), ", ")
	exposed := strings.Join(o.ExposedHeaders, ", ")

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			h := w.Header()
			ok := false
			switch {
			case allowAll:
				h.Set("Access-Control-Allow-Origin", "*")
				ok = true
			case origin != "":
				h.Add("Vary", "Origin")
				if _, found := allowed[strings.ToLower(origin)]; found {
					h.Set("Access-Control-Allow-Origin", origin)
					ok = true
				}
			}
			if ok {
				h.Set("Access-Control-Allow-Methods", methods)
				h.Set("Access-Control-Allow-Headers", headers)
				if exposed != "" {
					h.Set("Access-Control-Expose-Headers", exposed)
				}
				if o.AllowCredentials {
					h.Set("Access-Control-Allow-Credentials", "true")
				}
				if o.MaxAge > 0 {
					h.Set("Access-Control-Max-Age", strconv.Itoa(o.MaxAge))
				}
			}
			if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func orDefault(v, def []string) []string {
	if len(v) == 0 {
		return def
	}
	return v
}
