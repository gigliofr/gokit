// Package mailx invia email transazionali tramite Resend (API HTTP) o SMTP,
// scelti dalle variabili d'ambiente, con la stessa interfaccia per tutti i progetti.
package mailx

import (
	"errors"
	"fmt"
	"html"
	"log"
	"regexp"
	"strings"
)

// Sender invia un'email con parte testuale e HTML. Se una delle due è vuota
// viene ricavata dall'altra. La firma coincide con domain.EmailSender di
// mana-wise e con notifications.EmailSender di visto-easy.
type Sender interface {
	Send(to, subject, textBody, htmlBody string) error
}

// Options sono comuni a tutti i provider.
type Options struct {
	// From: mittente, es. "ManaWise <noreply@example.com>".
	From string
	// DefaultSubject: usato quando l'oggetto è vuoto.
	DefaultSubject string
	// SubjectPrefix: anteposto a ogni oggetto, es. "[QuotaZero] ".
	SubjectPrefix string
}

var (
	// ErrNoRecipient indica un destinatario vuoto.
	ErrNoRecipient = errors.New("destinatario email obbligatorio")
	// ErrEmptyBody indica che sia il testo sia l'HTML sono vuoti.
	ErrEmptyBody = errors.New("corpo dell'email vuoto")
)

// message è l'email normalizzata e validata, pronta per il provider.
type message struct {
	from, to, subject, text, html string
}

func (o Options) prepare(to, subject, textBody, htmlBody string) (message, error) {
	to = strings.TrimSpace(to)
	if to == "" {
		return message{}, ErrNoRecipient
	}
	if hasCRLF(to) || hasCRLF(o.From) {
		return message{}, errors.New("indirizzo email non valido")
	}
	subject = strings.TrimSpace(subject)
	if subject == "" {
		subject = o.DefaultSubject
	}
	subject = strings.Join(strings.Fields(o.SubjectPrefix+subject), " ") // niente a capo nell'oggetto
	if strings.TrimSpace(textBody) == "" && strings.TrimSpace(htmlBody) == "" {
		return message{}, ErrEmptyBody
	}
	if strings.TrimSpace(textBody) == "" {
		textBody = HTMLToText(htmlBody)
	}
	if strings.TrimSpace(htmlBody) == "" {
		htmlBody = TextToHTML(textBody)
	}
	return message{from: o.From, to: to, subject: subject, text: textBody, html: htmlBody}, nil
}

func hasCRLF(s string) bool { return strings.ContainsAny(s, "\r\n") }

var (
	reBreak = regexp.MustCompile(`(?i)<br\s*/?>|</p>|</div>|</h[1-6]>|</li>|</tr>`)
	reTags  = regexp.MustCompile(`<[^>]*>`)
	reBlank = regexp.MustCompile(`\n{3,}`)
)

// HTMLToText produce una versione testuale leggibile di un corpo HTML.
func HTMLToText(s string) string {
	s = reBreak.ReplaceAllString(s, "\n")
	s = reTags.ReplaceAllString(s, "")
	s = html.UnescapeString(s)
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimSpace(l)
	}
	return strings.TrimSpace(reBlank.ReplaceAllString(strings.Join(lines, "\n"), "\n\n"))
}

// TextToHTML converte un testo in HTML minimale con escape e a capo.
func TextToHTML(s string) string {
	return "<p>" + strings.ReplaceAll(html.EscapeString(s), "\n", "<br>") + "</p>"
}

// Noop non invia nulla: usato quando nessun provider è configurato.
type Noop struct{}

func (Noop) Send(_, _, _, _ string) error { return nil }

// Log scrive le email nel log invece di inviarle (utile in sviluppo).
type Log struct{ Logger *log.Logger }

func (l Log) Send(to, subject, textBody, _ string) error {
	lg := l.Logger
	if lg == nil {
		lg = log.Default()
	}
	lg.Printf("mailx: email non inviata (nessun provider) a=%s oggetto=%q\n%s", to, subject, textBody)
	return nil
}

// Describe restituisce il nome del provider di s (per log e diagnostica).
func Describe(s Sender) string {
	switch v := s.(type) {
	case *Resend:
		return "resend"
	case *SMTP:
		return fmt.Sprintf("smtp (%s:%d)", v.Host, v.Port)
	case Log:
		return "log"
	case Noop, nil:
		return "nessuno"
	default:
		return fmt.Sprintf("%T", s)
	}
}
