package mw

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

var ok = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) })

func TestSecurityHeaders(t *testing.T) {
	h := SecurityHeaders(SecurityOptions{FrameOptions: "SAMEORIGIN", ContentSecurityPolicy: "default-src 'self'", HSTSMaxAge: 100})(ok)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	hd := rec.Header()
	if hd.Get("X-Content-Type-Options") != "nosniff" || hd.Get("X-Frame-Options") != "SAMEORIGIN" ||
		hd.Get("Referrer-Policy") != "strict-origin-when-cross-origin" || hd.Get("Content-Security-Policy") == "" {
		t.Fatalf("header: %v", hd)
	}
	if hd.Get("Strict-Transport-Security") != "" {
		t.Fatal("HSTS non va inviato su HTTP")
	}
	rec = httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	r.TLS = &tls.ConnectionState{}
	h.ServeHTTP(rec, r)
	if rec.Header().Get("Strict-Transport-Security") != "max-age=100; includeSubDomains" {
		t.Fatal("HSTS mancante su HTTPS")
	}
	rec = httptest.NewRecorder()
	SecurityHeaders(SecurityOptions{FrameOptions: "-"})(ok).ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if rec.Header().Get("X-Frame-Options") != "" || rec.Header().Get("X-Frame-Options") == "-" {
		t.Fatal(`"-" deve disattivare l'header`)
	}
}

func TestCORS(t *testing.T) {
	h := CORS(CORSOptions{AllowedOrigins: []string{"https://app.test/"}, AllowCredentials: true, MaxAge: 600})(ok)

	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("Origin", "https://APP.test")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	if rec.Header().Get("Access-Control-Allow-Origin") != "https://APP.test" || rec.Header().Get("Access-Control-Allow-Credentials") != "true" || rec.Header().Get("Vary") != "Origin" {
		t.Fatalf("origine ammessa: %v", rec.Header())
	}

	r = httptest.NewRequest("GET", "/", nil)
	r.Header.Set("Origin", "https://evil.test")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	if rec.Header().Get("Access-Control-Allow-Origin") != "" || rec.Code != 200 {
		t.Fatal("origine non ammessa non deve ricevere ACAO")
	}

	r = httptest.NewRequest("OPTIONS", "/", nil)
	r.Header.Set("Origin", "https://app.test")
	r.Header.Set("Access-Control-Request-Method", "POST")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	if rec.Code != 204 || rec.Header().Get("Access-Control-Max-Age") != "600" {
		t.Fatalf("preflight: %d %v", rec.Code, rec.Header())
	}

	rec = httptest.NewRecorder()
	r = httptest.NewRequest("GET", "/", nil)
	r.Header.Set("Origin", "https://x.test")
	CORS(CORSOptions{AllowedOrigins: []string{"*"}})(ok).ServeHTTP(rec, r)
	if rec.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Fatal("wildcard")
	}
	rec = httptest.NewRecorder()
	CORS(CORSOptions{AllowedOrigins: []string{"*"}, AllowCredentials: true})(ok).ServeHTTP(rec, r)
	if rec.Header().Get("Access-Control-Allow-Origin") == "*" {
		t.Fatal("wildcard con credenziali non è ammesso")
	}
}

func TestLimiter(t *testing.T) {
	now := time.Unix(0, 0)
	l := NewLimiter(RateLimitOptions{Requests: 2, Window: time.Second, IdleTTL: 10 * time.Second, Now: func() time.Time { return now }})
	if a, _ := l.Allow("a"); !a {
		t.Fatal("1")
	}
	if a, _ := l.Allow("a"); !a {
		t.Fatal("2")
	}
	a, wait := l.Allow("a")
	if a || wait <= 0 || wait > time.Second {
		t.Fatalf("3: %v %v", a, wait)
	}
	if b, _ := l.Allow("b"); !b {
		t.Fatal("chiavi indipendenti")
	}
	now = now.Add(500 * time.Millisecond)
	if a, _ := l.Allow("a"); !a {
		t.Fatal("ricarica")
	}
	now = now.Add(time.Minute)
	l.Allow("c")
	if l.Len() != 1 {
		t.Fatalf("pulizia chiavi inattive: %d", l.Len())
	}
}

func TestRateLimitMiddleware(t *testing.T) {
	h := RateLimit(RateLimitOptions{Requests: 1, Window: time.Minute})(ok)
	do := func(ip string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", "/login", nil)
		r.RemoteAddr = ip + ":5000"
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, r)
		return rec
	}
	if do("1.1.1.1").Code != 200 {
		t.Fatal("prima richiesta")
	}
	rec := do("1.1.1.1")
	if rec.Code != 429 || rec.Header().Get("Retry-After") == "" {
		t.Fatalf("seconda: %d", rec.Code)
	}
	if do("2.2.2.2").Code != 200 {
		t.Fatal("altro IP")
	}
}

func TestLimiterConcurrent(t *testing.T) {
	l := NewLimiter(RateLimitOptions{Requests: 1000, Window: time.Hour})
	var wg sync.WaitGroup
	var mu sync.Mutex
	allowed := 0
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 40; j++ {
				if a, _ := l.Allow("k"); a {
					mu.Lock()
					allowed++
					mu.Unlock()
				}
			}
		}()
	}
	wg.Wait()
	if allowed < 1000 || allowed > 1001 {
		t.Fatalf("consentite %d, attese ~1000", allowed)
	}
}

func TestRecover(t *testing.T) {
	var gotPath string
	h := Recover(func(r *http.Request, v any, stack []byte) { gotPath = r.URL.Path })(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/x", nil))
	if rec.Code != 500 || gotPath != "/x" {
		t.Fatalf("%d %q", rec.Code, gotPath)
	}
	defer func() {
		if recover() != http.ErrAbortHandler {
			t.Fatal("ErrAbortHandler va propagato")
		}
	}()
	Recover()(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic(http.ErrAbortHandler) })).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
}

func TestRealIPAndRejectPreflight(t *testing.T) {
	var seen string
	h := RealIP(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { seen = r.RemoteAddr }))
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "10.0.0.1:5555"
	r.Header.Set("X-Forwarded-For", "6.6.6.6, 198.51.100.7")
	h.ServeHTTP(httptest.NewRecorder(), r)
	if seen != "198.51.100.7:0" {
		t.Fatalf("RemoteAddr = %s", seen)
	}

	c := CORS(CORSOptions{AllowedOrigins: []string{"https://ok.test"}, RejectDisallowedPreflight: true})(ok)
	r = httptest.NewRequest("OPTIONS", "/", nil)
	r.Header.Set("Origin", "https://evil.test")
	r.Header.Set("Access-Control-Request-Method", "POST")
	rec := httptest.NewRecorder()
	c.ServeHTTP(rec, r)
	if rec.Code != 403 {
		t.Fatalf("preflight non ammesso: %d", rec.Code)
	}
}
