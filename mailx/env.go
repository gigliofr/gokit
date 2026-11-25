package mailx

import (
	"os"
	"strconv"
	"strings"
)

// FromEnv sceglie il provider dalle variabili d'ambiente:
//
//  1. Resend se RESEND_API_KEY è impostata (mittente: RESEND_FROM_EMAIL o MAIL_FROM);
//  2. SMTP se SMTP_HOST e MAIL_FROM sono impostate
//     (SMTP_PORT default 587; password da SMTP_PASSWORD, SMTP_PASS o SMTP_KEY);
//  3. altrimenti Noop.
//
// I campi vuoti di o vengono completati con il mittente letto dall'ambiente.
// Il secondo valore descrive la scelta o cosa manca (per i log di avvio).
func FromEnv(o Options) (Sender, string) {
	get := func(keys ...string) string {
		for _, k := range keys {
			if v := strings.TrimSpace(os.Getenv(k)); v != "" {
				return v
			}
		}
		return ""
	}
	if key := get("RESEND_API_KEY"); key != "" {
		ro := o
		if ro.From == "" {
			ro.From = get("RESEND_FROM_EMAIL", "MAIL_FROM")
		}
		if ro.From == "" {
			return Noop{}, "email disattivate: RESEND_API_KEY senza RESEND_FROM_EMAIL/MAIL_FROM"
		}
		return NewResend(key, ro), "email via Resend"
	}
	host := get("SMTP_HOST")
	so := o
	if so.From == "" {
		so.From = get("MAIL_FROM", "SMTP_FROM")
	}
	if host == "" || so.From == "" {
		var missing []string
		if host == "" {
			missing = append(missing, "SMTP_HOST")
		}
		if so.From == "" {
			missing = append(missing, "MAIL_FROM")
		}
		return Noop{}, "email disattivate: imposta RESEND_API_KEY oppure " + strings.Join(missing, ", ")
	}
	port, err := strconv.Atoi(get("SMTP_PORT"))
	if err != nil || port <= 0 {
		port = 587
	}
	s := NewSMTP(host, port, get("SMTP_USER", "SMTP_USERNAME"), get("SMTP_PASSWORD", "SMTP_PASS", "SMTP_KEY"), so)
	return s, "email via SMTP " + host + ":" + strconv.Itoa(port)
}
