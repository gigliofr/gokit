package mailx

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net"
	"net/http"
	"net/http/httptest"
	"net/mail"
	"strconv"
	"strings"
	"testing"
)

func TestPrepare(t *testing.T) {
	o := Options{From: "App <no@app.it>", DefaultSubject: "Notifica", SubjectPrefix: "[App] "}
	m, err := o.prepare(" a@b.it ", "", "", "<p>Ciao &amp; benvenuto</p><p>riga 2</p>")
	if err != nil {
		t.Fatal(err)
	}
	if m.to != "a@b.it" || m.subject != "[App] Notifica" || m.text != "Ciao & benvenuto\nriga 2" {
		t.Fatalf("%+v", m)
	}
	m, _ = o.prepare("a@b.it", "Oggetto\r\nBcc: x@y.it", "<b>testo</b>\n2", "")
	if strings.ContainsAny(m.subject, "\r\n") {
		t.Fatal("a capo nell'oggetto")
	}
	if m.html != "<p>&lt;b&gt;testo&lt;/b&gt;<br>2</p>" {
		t.Fatalf("html: %s", m.html)
	}
	if _, err := o.prepare("", "s", "t", ""); !errors.Is(err, ErrNoRecipient) {
		t.Fatal("destinatario vuoto")
	}
	if _, err := o.prepare("a@b.it", "s", " ", ""); !errors.Is(err, ErrEmptyBody) {
		t.Fatal("corpo vuoto")
	}
	if _, err := o.prepare("a@b.it\r\nBcc: z@z.it", "s", "t", ""); err == nil {
		t.Fatal("injection nel destinatario")
	}
}

func TestResend(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer re_test" {
			w.WriteHeader(401)
			_, _ = w.Write([]byte(`{"message":"API key non valida"}`))
			return
		}
		_ = json.NewDecoder(r.Body).Decode(&got)
		_, _ = w.Write([]byte(`{"id":"abc"}`))
	}))
	defer srv.Close()
	r := NewResend("re_test", Options{From: "App <no@app.it>"})
	r.Endpoint = srv.URL
	if err := r.Send("a@b.it", "Ciao", "testo", ""); err != nil {
		t.Fatal(err)
	}
	if got["subject"] != "Ciao" || got["from"] != "App <no@app.it>" || got["html"] != "<p>testo</p>" {
		t.Fatalf("payload: %v", got)
	}
	r.APIKey = "sbagliata"
	if err := r.Send("a@b.it", "Ciao", "testo", ""); err == nil || !strings.Contains(err.Error(), "API key non valida") {
		t.Fatalf("errore atteso: %v", err)
	}
}

// fakeSMTP accetta un messaggio senza TLS né AUTH e lo restituisce sul canale.
func fakeSMTP(t *testing.T) (port int, msgs chan string) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	msgs = make(chan string, 1)
	go func() {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		defer c.Close()
		rd := bufio.NewReader(c)
		say := func(s string) { _, _ = io.WriteString(c, s+"\r\n") }
		say("220 fake")
		var data strings.Builder
		inData := false
		for {
			line, err := rd.ReadString('\n')
			if err != nil {
				return
			}
			if inData {
				if line == ".\r\n" {
					inData = false
					msgs <- data.String()
					say("250 ok")
					continue
				}
				data.WriteString(line)
				continue
			}
			switch cmd := strings.ToUpper(strings.TrimSpace(line)); {
			case strings.HasPrefix(cmd, "EHLO"):
				say("250 fake")
			case strings.HasPrefix(cmd, "DATA"):
				inData = true
				say("354 go")
			case strings.HasPrefix(cmd, "QUIT"):
				say("221 bye")
				return
			default:
				say("250 ok")
			}
		}
	}()
	return ln.Addr().(*net.TCPAddr).Port, msgs
}

func TestSMTP(t *testing.T) {
	port, msgs := fakeSMTP(t)
	s := NewSMTP("127.0.0.1", port, "", "", Options{From: "Visto Easy <no@visto.it>", SubjectPrefix: "[VE] "})
	if err := s.Send("Mario Rossi <mario@example.it>", "Pratica approvata ✅", "", "<p>Ciao <b>Mario</b></p>"); err != nil {
		t.Fatal(err)
	}
	raw := <-msgs
	m, err := mail.ReadMessage(strings.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	subj, _ := new(mime.WordDecoder).DecodeHeader(m.Header.Get("Subject"))
	if subj != "[VE] Pratica approvata ✅" || m.Header.Get("Message-Id") == "" || m.Header.Get("Date") == "" {
		t.Fatalf("header: %v / %q", m.Header, subj)
	}
	_, params, _ := mime.ParseMediaType(m.Header.Get("Content-Type"))
	mr := multipart.NewReader(m.Body, params["boundary"])
	var parts []string
	for {
		p, err := mr.NextPart()
		if err != nil {
			break
		}
		b, _ := io.ReadAll(p) // il reader decodifica quoted-printable
		parts = append(parts, p.Header.Get("Content-Type")+"|"+string(b))
	}
	if len(parts) != 2 || !strings.Contains(parts[0], "Ciao Mario") || !strings.Contains(parts[1], "<b>Mario</b>") {
		t.Fatalf("parti: %q", parts)
	}
}

func TestFromEnv(t *testing.T) {
	clear := func() {
		for _, k := range []string{"RESEND_API_KEY", "RESEND_FROM_EMAIL", "MAIL_FROM", "SMTP_HOST", "SMTP_PORT", "SMTP_USER", "SMTP_KEY", "SMTP_PASS", "SMTP_PASSWORD", "SMTP_FROM", "SMTP_USERNAME"} {
			t.Setenv(k, "")
		}
	}
	clear()
	if s, why := FromEnv(Options{}); Describe(s) != "nessuno" || !strings.Contains(why, "SMTP_HOST") {
		t.Fatalf("%s %s", Describe(s), why)
	}
	t.Setenv("SMTP_HOST", "smtp.test")
	t.Setenv("MAIL_FROM", "no@test.it")
	t.Setenv("SMTP_KEY", "k")
	t.Setenv("SMTP_PORT", "465")
	s, _ := FromEnv(Options{DefaultSubject: "x"})
	sm, ok := s.(*SMTP)
	if !ok || sm.Port != 465 || sm.Password != "k" || sm.From != "no@test.it" || sm.DefaultSubject != "x" {
		t.Fatalf("smtp: %#v", s)
	}
	t.Setenv("RESEND_API_KEY", "re_x")
	s, _ = FromEnv(Options{})
	if Describe(s) != "resend" || s.(*Resend).From != "no@test.it" {
		t.Fatalf("resend: %#v", s)
	}
	_ = strconv.Itoa
}

func TestNoopAndLog(t *testing.T) {
	if (Noop{}).Send("a", "b", "c", "d") != nil || (Log{}).Send("a", "b", "c", "d") != nil {
		t.Fatal()
	}
}
