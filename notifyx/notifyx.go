// Package notifyx invia notifiche su canali di messaggistica (oggi Telegram)
// e gestisce gli avvisi operativi delle applicazioni (avvio, arresto, panic,
// dipendenze non raggiungibili).
package notifyx

import (
	"context"
	"errors"
)

// Notifier invia un messaggio di testo su un canale.
type Notifier interface {
	Notify(ctx context.Context, text string) error
}

// Noop scarta i messaggi.
type Noop struct{}

func (Noop) Notify(context.Context, string) error { return nil }

// Multi invia a tutti i canali e restituisce gli errori uniti.
type Multi []Notifier

func (m Multi) Notify(ctx context.Context, text string) error {
	var errs []error
	for _, n := range m {
		if n == nil {
			continue
		}
		if err := n.Notify(ctx, text); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// Func adatta una funzione a Notifier.
type Func func(ctx context.Context, text string) error

func (f Func) Notify(ctx context.Context, text string) error { return f(ctx, text) }
