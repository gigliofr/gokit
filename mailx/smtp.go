package mailx

import (
	"bytes"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/smtp"
	"net/textproto"
	"strconv"
	"strings"
	"time"
)

// SMTP invia tramite un server SMTP. Porta 465: TLS implicito;
// altre porte (587, 25): STARTTLS se offerto dal server.
type SMTP struct {
	Host     string
	Port     int
	User     string
	Password string
	Options
	// Timeout di connessione; default 15s.
	Timeout time.Duration
}

// NewSMTP crea un Sender SMTP.
func NewSMTP(host string, port int, user, password string, o Options) *SMTP {
	return &SMTP{Host: host, Port: port, User: user, Password: password, Options: o}
}

func (s *SMTP) Send(to, subject, textBody, htmlBody string) error {
	m, err := s.prepare(to, subject, textBody, htmlBody)
	if err != nil {
		return err
	}
	fromAddr, err := mail.ParseAddress(m.from)
	if err != nil {
		return fmt.Errorf("smtp: mittente non valido: %w", err)
	}
	toAddr, err := mail.ParseAddress(m.to)
	if err != nil {
		return fmt.Errorf("smtp: destinatario non valido: %w", err)
	}
	raw, err := buildMIME(m, fromAddr, toAddr, time.Now())
	if err != nil {
		return err
	}
	if err := s.deliver(fromAddr.Address, toAddr.Address, raw); err != nil {
		return fmt.Errorf("smtp: invio fallito: %w", err)
	}
	return nil
}

func (s *SMTP) deliver(from, to string, raw []byte) error {
	timeout := s.Timeout
	if timeout == 0 {
		timeout = 15 * time.Second
	}
	addr := net.JoinHostPort(s.Host, strconv.Itoa(s.Port))
	dialer := &net.Dialer{Timeout: timeout}
	var conn net.Conn
	var err error
	if s.Port == 465 {
		conn, err = tls.DialWithDialer(dialer, "tcp", addr, &tls.Config{ServerName: s.Host, MinVersion: tls.VersionTLS12})
	} else {
		conn, err = dialer.Dial("tcp", addr)
	}
	if err != nil {
		return err
	}
	_ = conn.SetDeadline(time.Now().Add(2 * timeout))
	c, err := smtp.NewClient(conn, s.Host)
	if err != nil {
		conn.Close()
		return err
	}
	defer c.Close()
	if s.Port != 465 {
		if ok, _ := c.Extension("STARTTLS"); ok {
			if err := c.StartTLS(&tls.Config{ServerName: s.Host, MinVersion: tls.VersionTLS12}); err != nil {
				return err
			}
		}
	}
	if s.User != "" {
		if ok, _ := c.Extension("AUTH"); ok {
			// smtp.PlainAuth rifiuta di inviare credenziali in chiaro fuori da localhost.
			if err := c.Auth(smtp.PlainAuth("", s.User, s.Password, s.Host)); err != nil {
				return err
			}
		}
	}
	if err := c.Mail(from); err != nil {
		return err
	}
	if err := c.Rcpt(to); err != nil {
		return err
	}
	w, err := c.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write(raw); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return c.Quit()
}

func buildMIME(m message, from, to *mail.Address, now time.Time) ([]byte, error) {
	var buf bytes.Buffer
	var rnd [12]byte
	_, _ = rand.Read(rnd[:])
	id := hex.EncodeToString(rnd[:])
	domain := "localhost"
	if i := strings.LastIndex(from.Address, "@"); i >= 0 {
		domain = from.Address[i+1:]
	}
	boundary := "alt-" + id
	h := func(k, v string) { buf.WriteString(k + ": " + v + "\r\n") }
	h("From", from.String())
	h("To", to.String())
	h("Subject", mime.QEncoding.Encode("UTF-8", m.subject))
	h("Date", now.Format(time.RFC1123Z))
	h("Message-ID", "<"+id+"@"+domain+">")
	h("MIME-Version", "1.0")
	h("Content-Type", "multipart/alternative; boundary="+boundary)
	buf.WriteString("\r\n")

	mw := multipart.NewWriter(&buf)
	if err := mw.SetBoundary(boundary); err != nil {
		return nil, err
	}
	for _, part := range []struct{ ctype, body string }{
		{"text/plain; charset=UTF-8", m.text},
		{"text/html; charset=UTF-8", m.html},
	} {
		ph := textproto.MIMEHeader{}
		ph.Set("Content-Type", part.ctype)
		ph.Set("Content-Transfer-Encoding", "quoted-printable")
		pw, err := mw.CreatePart(ph)
		if err != nil {
			return nil, err
		}
		qp := quotedprintable.NewWriter(pw)
		if _, err := qp.Write([]byte(part.body)); err != nil {
			return nil, err
		}
		if err := qp.Close(); err != nil {
			return nil, err
		}
	}
	if err := mw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
