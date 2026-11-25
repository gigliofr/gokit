package notifyx

import (
	"context"
	"fmt"
	"log"
	"os"
	"runtime/debug"
	"strings"
	"sync"
	"time"
)

// Ops invia gli avvisi operativi di un'applicazione (avvio, arresto, errori,
// panic, dipendenze non raggiungibili) con un prefisso "[app · ambiente]" e
// deduplica: lo stesso avviso non viene ripetuto prima di DedupWindow.
// Tutti i metodi sono nil-safe e non bloccano mai la richiesta in corso.
type Ops struct {
	App         string
	Env         string
	Notifier    Notifier
	DedupWindow time.Duration
	Logger      *log.Logger

	mu   sync.Mutex
	last map[string]time.Time
	now  func() time.Time
	wg   sync.WaitGroup
}

// NewOps crea un Ops. n nil equivale a Noop (gli avvisi finiscono solo nel log).
func NewOps(app, env string, n Notifier) *Ops {
	if n == nil {
		n = Noop{}
	}
	return &Ops{App: app, Env: env, Notifier: n, DedupWindow: 15 * time.Minute, last: map[string]time.Time{}, now: time.Now}
}

// OpsFromEnv crea un Ops che invia su Telegram usando OPS_TELEGRAM_BOT_TOKEN
// e OPS_TELEGRAM_CHAT_ID (gli stessi bot e chat configurati in Uptime Kuma
// raccolgono così tutti gli avvisi). L'ambiente è letto da ENVIRONMENT o APP_ENV.
// Senza configurazione gli avvisi vengono solo scritti nel log.
func OpsFromEnv(app string) *Ops {
	env := strings.TrimSpace(os.Getenv("ENVIRONMENT"))
	if env == "" {
		env = strings.TrimSpace(os.Getenv("APP_ENV"))
	}
	var n Notifier
	if tg := TelegramFromEnv("OPS_"); tg != nil {
		n = tg
	}
	return NewOps(app, env, n)
}

// Enabled indica se gli avvisi vengono inviati su un canale reale.
func (o *Ops) Enabled() bool {
	if o == nil {
		return false
	}
	_, noop := o.Notifier.(Noop)
	return !noop
}

func (o *Ops) prefix() string {
	host, _ := os.Hostname()
	parts := []string{o.App}
	if o.Env != "" {
		parts = append(parts, o.Env)
	}
	if host != "" {
		parts = append(parts, host)
	}
	return "[" + strings.Join(parts, " · ") + "]"
}

func (o *Ops) logf(format string, args ...any) {
	if o.Logger != nil {
		o.Logger.Printf(format, args...)
		return
	}
	log.Printf(format, args...)
}

// allow applica la deduplica per chiave.
func (o *Ops) allow(key string) bool {
	if key == "" {
		return true
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	now := o.now()
	if t, ok := o.last[key]; ok && now.Sub(t) < o.DedupWindow {
		return false
	}
	o.last[key] = now
	return true
}

func (o *Ops) send(sync bool, key, emoji, title, detail string) {
	if o == nil {
		return
	}
	if !o.allow(key) {
		return
	}
	text := emoji + " " + o.prefix() + " " + title
	if detail = strings.TrimSpace(detail); detail != "" {
		text += "\n" + detail
	}
	o.logf("ops: %s", strings.ReplaceAll(text, "\n", " | "))
	do := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := o.Notifier.Notify(ctx, text); err != nil {
			o.logf("ops: invio avviso fallito: %v", err)
		}
	}
	if sync {
		do()
		return
	}
	o.wg.Add(1)
	go func() { defer o.wg.Done(); do() }()
}

// Started segnala l'avvio dell'applicazione (version facoltativa).
func (o *Ops) Started(version string) {
	d := ""
	if version != "" {
		d = "versione " + version
	}
	o.send(false, "", "🟢", "avviata", d)
}

// Stopping segnala l'arresto ordinato e attende l'invio (max 10s):
// va chiamato prima di uscire dal processo.
func (o *Ops) Stopping(reason string) {
	o.send(true, "", "🟡", "in arresto", reason)
	o.Flush()
}

// Error segnala un errore; key raggruppa gli errori simili per la deduplica.
func (o *Ops) Error(key, title string, err error) {
	d := ""
	if err != nil {
		d = err.Error()
	}
	o.send(false, "err:"+key, "🔴", title, d)
}

// Info invia un messaggio informativo (deduplicato per key, se non vuota).
func (o *Ops) Info(key, title, detail string) { o.send(false, key, "ℹ️", title, detail) }

// Panic segnala un panic recuperato; where è il contesto (es. "GET /api/x").
func (o *Ops) Panic(where string, v any, stack []byte) {
	if stack == nil {
		stack = debug.Stack()
	}
	s := string(stack)
	if len(s) > 1500 {
		s = s[:1500] + "…"
	}
	o.send(false, "panic:"+where+fmt.Sprint(v), "💥", "panic in "+where, fmt.Sprintf("%v\n%s", v, s))
}

// Flush attende l'invio degli avvisi asincroni in corso.
func (o *Ops) Flush() {
	if o != nil {
		o.wg.Wait()
	}
}

// Watch controlla check ogni interval e avvisa solo ai cambi di stato:
// "non raggiungibile" al primo errore e "di nuovo raggiungibile" al ritorno.
// Si ferma quando ctx termina. Esempio: o.Watch(ctx, "MongoDB", time.Minute, store.Ping).
func (o *Ops) Watch(ctx context.Context, name string, interval time.Duration, check func(context.Context) error) {
	if o == nil || check == nil {
		return
	}
	if interval <= 0 {
		interval = time.Minute
	}
	go func() {
		down := false
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
			}
			cctx, cancel := context.WithTimeout(ctx, interval/2+time.Second)
			err := check(cctx)
			cancel()
			switch {
			case err != nil && !down:
				down = true
				o.send(false, "", "🔴", name+" non raggiungibile", err.Error())
			case err == nil && down:
				down = false
				o.send(false, "", "✅", name+" di nuovo raggiungibile", "")
			}
		}
	}()
}
