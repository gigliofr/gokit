package mw

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"runtime/debug"
)

// PanicHook riceve i panic recuperati (es. notifyx.Ops.PanicHook()).
type PanicHook func(r *http.Request, v any, stack []byte)

// Recover intercetta i panic degli handler, li scrive nel log, chiama i hook
// e risponde 500 {"error":"errore interno del server"}.
// http.ErrAbortHandler viene propagato come previsto da net/http.
func Recover(hooks ...PanicHook) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				v := recover()
				if v == nil {
					return
				}
				if err, ok := v.(error); ok && errors.Is(err, http.ErrAbortHandler) {
					panic(v)
				}
				stack := debug.Stack()
				log.Printf("panic %s %s: %v\n%s", r.Method, r.URL.Path, v, stack)
				for _, h := range hooks {
					if h != nil {
						h(r, v, stack)
					}
				}
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				w.WriteHeader(http.StatusInternalServerError)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "errore interno del server"})
			}()
			next.ServeHTTP(w, r)
		})
	}
}
