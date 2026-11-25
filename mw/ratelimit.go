package mw

import (
	"encoding/json"
	"math"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// RateLimitOptions configura RateLimit.
type RateLimitOptions struct {
	// Requests ammesse per Window per ogni chiave (es. 10 ogni minuto).
	Requests int
	Window   time.Duration
	// Burst: richieste consecutive ammesse a freddo; default = Requests.
	Burst int
	// Key ricava la chiave dalla richiesta; default: IP da RemoteAddr.
	// Dietro proxy usare chi/middleware.RealIP oppure httpx.ClientIP(r, true).
	Key func(*http.Request) string
	// OnLimit risponde quando il limite è superato; default 429 JSON
	// {"error":"troppe richieste, riprova più tardi"} con Retry-After.
	OnLimit http.Handler
	// IdleTTL: dopo quanto eliminare le chiavi inattive; default 10*Window.
	IdleTTL time.Duration
	// Now permette di iniettare l'orologio nei test.
	Now func() time.Time
}

// Limiter è un token bucket per chiave, sicuro per l'uso concorrente.
type Limiter struct {
	rate     float64 // token al secondo
	burst    float64
	idleTTL  time.Duration
	now      func() time.Time
	mu       sync.Mutex
	buckets  map[string]*bucket
	lastScan time.Time
}

type bucket struct {
	tokens float64
	last   time.Time
}

// NewLimiter crea un limiter da opzioni (usa Requests, Window, Burst, IdleTTL, Now).
func NewLimiter(o RateLimitOptions) *Limiter {
	if o.Requests <= 0 {
		o.Requests = 60
	}
	if o.Window <= 0 {
		o.Window = time.Minute
	}
	if o.Burst <= 0 {
		o.Burst = o.Requests
	}
	if o.IdleTTL <= 0 {
		o.IdleTTL = 10 * o.Window
	}
	if o.Now == nil {
		o.Now = time.Now
	}
	return &Limiter{
		rate:    float64(o.Requests) / o.Window.Seconds(),
		burst:   float64(o.Burst),
		idleTTL: o.IdleTTL,
		now:     o.Now,
		buckets: make(map[string]*bucket),
	}
}

// Allow consuma un token per key. Se negato restituisce l'attesa consigliata.
func (l *Limiter) Allow(key string) (bool, time.Duration) {
	now := l.now()
	l.mu.Lock()
	defer l.mu.Unlock()
	l.sweep(now)
	b, ok := l.buckets[key]
	if !ok {
		b = &bucket{tokens: l.burst, last: now}
		l.buckets[key] = b
	}
	b.tokens = math.Min(l.burst, b.tokens+now.Sub(b.last).Seconds()*l.rate)
	b.last = now
	if b.tokens >= 1 {
		b.tokens--
		return true, 0
	}
	wait := time.Duration((1 - b.tokens) / l.rate * float64(time.Second))
	return false, wait
}

// Len restituisce il numero di chiavi tracciate (utile per test e metriche).
func (l *Limiter) Len() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.buckets)
}

// sweep elimina le chiavi inattive al più una volta per idleTTL/2.
func (l *Limiter) sweep(now time.Time) {
	if now.Sub(l.lastScan) < l.idleTTL/2 {
		return
	}
	l.lastScan = now
	for k, b := range l.buckets {
		if now.Sub(b.last) > l.idleTTL {
			delete(l.buckets, k)
		}
	}
}

// RateLimit limita le richieste per chiave (default: IP del client).
func RateLimit(o RateLimitOptions) func(http.Handler) http.Handler {
	l := NewLimiter(o)
	key := o.Key
	if key == nil {
		key = remoteIP
	}
	onLimit := o.OnLimit
	if onLimit == nil {
		onLimit = http.HandlerFunc(defaultOnLimit)
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ok, wait := l.Allow(key(r))
			if !ok {
				secs := int(math.Ceil(wait.Seconds()))
				if secs < 1 {
					secs = 1
				}
				w.Header().Set("Retry-After", strconv.Itoa(secs))
				onLimit.ServeHTTP(w, r)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func remoteIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func defaultOnLimit(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusTooManyRequests)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": "troppe richieste, riprova più tardi"})
}
