package notifyx

import "net/http"

// PanicHook restituisce un hook per mw.Recover che invia un avviso "panic".
//
//	r.Use(mw.Recover(ops.PanicHook()))
func (o *Ops) PanicHook() func(r *http.Request, v any, stack []byte) {
	return func(r *http.Request, v any, stack []byte) {
		o.Panic(r.Method+" "+r.URL.Path, v, stack)
	}
}
