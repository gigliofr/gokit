package notifyx

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

type recorder struct {
	mu   sync.Mutex
	msgs []string
	err  error
}

func (r *recorder) Notify(_ context.Context, text string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.msgs = append(r.msgs, text)
	return r.err
}

func (r *recorder) all() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.msgs...)
}

func TestTelegram(t *testing.T) {
	var got map[string]any
	var path string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&got)
		if got["chat_id"] == "bad" {
			w.WriteHeader(400)
			_, _ = w.Write([]byte(`{"ok":false,"description":"Bad Request: chat not found"}`))
			return
		}
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()
	tg := NewTelegram("123:ABC", "-100")
	tg.BaseURL = srv.URL
	if err := tg.Notify(context.Background(), strings.Repeat("x", 5000)); err != nil {
		t.Fatal(err)
	}
	if path != "/bot123:ABC/sendMessage" || got["chat_id"] != "-100" || len([]rune(got["text"].(string))) != 4096 {
		t.Fatalf("richiesta: %s %v", path, len(got["text"].(string)))
	}
	tg.ChatID = "bad"
	err := tg.Notify(context.Background(), "x")
	if err == nil || !strings.Contains(err.Error(), "chat not found") || strings.Contains(err.Error(), "123:ABC") {
		t.Fatalf("errore: %v", err)
	}
	var nilTG *Telegram
	if nilTG.Enabled() || nilTG.Notify(context.Background(), "x") == nil {
		t.Fatal("nil telegram")
	}
	// errore di rete: il token non deve comparire
	tg = NewTelegram("999:SECRET", "1")
	tg.BaseURL = "http://127.0.0.1:1"
	if err := tg.Notify(context.Background(), "x"); err == nil || strings.Contains(err.Error(), "SECRET") {
		t.Fatalf("token esposto: %v", err)
	}
}

func TestTelegramFromEnv(t *testing.T) {
	t.Setenv("OPS_TELEGRAM_BOT_TOKEN", "t")
	t.Setenv("OPS_TELEGRAM_CHAT_ID", "")
	if TelegramFromEnv("OPS_") != nil {
		t.Fatal("chat mancante")
	}
	t.Setenv("OPS_TELEGRAM_CHAT_ID", "c")
	if tg := TelegramFromEnv("OPS_"); tg == nil || tg.ChatID != "c" {
		t.Fatal("config")
	}
}

func TestMulti(t *testing.T) {
	a, b := &recorder{}, &recorder{err: errors.New("giù")}
	err := Multi{a, nil, b}.Notify(context.Background(), "ciao")
	if err == nil || len(a.all()) != 1 || len(b.all()) != 1 {
		t.Fatal("multi")
	}
}

func TestOpsDedupAndFormat(t *testing.T) {
	r := &recorder{}
	o := NewOps("visto-easy", "production", r)
	now := time.Unix(0, 0)
	o.now = func() time.Time { return now }
	o.Error("mongo", "errore DB", errors.New("timeout"))
	o.Error("mongo", "errore DB", errors.New("timeout"))
	o.Flush()
	if len(r.all()) != 1 {
		t.Fatalf("dedup: %v", r.all())
	}
	msg := r.all()[0]
	if !strings.HasPrefix(msg, "🔴 [visto-easy · production") || !strings.Contains(msg, "errore DB\ntimeout") {
		t.Fatalf("formato: %q", msg)
	}
	now = now.Add(16 * time.Minute)
	o.Error("mongo", "errore DB", nil)
	o.Started("1.2.3")
	o.Panic("GET /x", "boom", []byte("stack"))
	o.Stopping("SIGTERM")
	if n := len(r.all()); n != 5 {
		t.Fatalf("messaggi: %d %v", n, r.all())
	}
	var nilOps *Ops
	nilOps.Started("")
	nilOps.Error("k", "t", nil)
	nilOps.Flush()
	if nilOps.Enabled() || NewOps("a", "", nil).Enabled() || !o.Enabled() {
		t.Fatal("Enabled")
	}
}

func TestOpsWatch(t *testing.T) {
	r := &recorder{}
	o := NewOps("app", "", r)
	var mu sync.Mutex
	fail := true
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	o.Watch(ctx, "MongoDB", 20*time.Millisecond, func(context.Context) error {
		mu.Lock()
		defer mu.Unlock()
		if fail {
			return errors.New("connection refused")
		}
		return nil
	})
	time.Sleep(90 * time.Millisecond)
	mu.Lock()
	fail = false
	mu.Unlock()
	time.Sleep(90 * time.Millisecond)
	cancel()
	o.Flush()
	msgs := r.all()
	if len(msgs) != 2 || !strings.Contains(msgs[0], "MongoDB non raggiungibile") || !strings.Contains(msgs[1], "di nuovo raggiungibile") {
		t.Fatalf("watch: %v", msgs)
	}
}
