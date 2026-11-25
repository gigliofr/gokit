// Package httpx contiene gli helper HTTP comuni: risposte e decodifica JSON,
// estrazione di IP e token, avvio del server con graceful shutdown.
package httpx

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

// ErrorBody è il formato standard degli errori: {"error": "...", "code": "..."}.
// Code è facoltativo e serve ai client che vogliono distinguere i casi.
type ErrorBody struct {
	Error string `json:"error"`
	Code  string `json:"code,omitempty"`
}

// JSON scrive v come JSON con lo status indicato.
func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if v == nil || status == http.StatusNoContent {
		return
	}
	_ = json.NewEncoder(w).Encode(v)
}

// Error scrive {"error": msg}.
func Error(w http.ResponseWriter, status int, msg string) {
	JSON(w, status, ErrorBody{Error: msg})
}

// ErrorCode scrive {"error": msg, "code": code}.
func ErrorCode(w http.ResponseWriter, status int, code, msg string) {
	JSON(w, status, ErrorBody{Error: msg, Code: code})
}

// DefaultMaxBodyBytes è il limite predefinito del body accettato da Decode (1 MiB).
const DefaultMaxBodyBytes int64 = 1 << 20

// DecodeOptions regola Decode.
type DecodeOptions struct {
	// MaxBytes limita la dimensione del body; 0 = DefaultMaxBodyBytes, <0 = nessun limite.
	MaxBytes int64
	// AllowUnknownFields accetta campi non presenti nella struct di destinazione.
	AllowUnknownFields bool
}

// ErrBodyTooLarge è restituito quando il body supera MaxBytes.
var ErrBodyTooLarge = errors.New("payload troppo grande")

// Decode legge un singolo oggetto JSON dal body in dst.
// Per default rifiuta campi sconosciuti, body vuoti, contenuti dopo l'oggetto
// e body oltre 1 MiB. w serve a http.MaxBytesReader e può essere nil.
func Decode(w http.ResponseWriter, r *http.Request, dst any, opts ...DecodeOptions) error {
	var o DecodeOptions
	if len(opts) > 0 {
		o = opts[0]
	}
	limit := o.MaxBytes
	if limit == 0 {
		limit = DefaultMaxBodyBytes
	}
	body := r.Body
	if limit > 0 {
		body = http.MaxBytesReader(w, r.Body, limit)
	}
	dec := json.NewDecoder(body)
	if !o.AllowUnknownFields {
		dec.DisallowUnknownFields()
	}
	if err := dec.Decode(dst); err != nil {
		var mbe *http.MaxBytesError
		switch {
		case errors.As(err, &mbe):
			return ErrBodyTooLarge
		case errors.Is(err, io.EOF):
			return errors.New("body vuoto")
		default:
			return err
		}
	}
	if dec.More() {
		return fmt.Errorf("il body deve contenere un solo oggetto JSON")
	}
	return nil
}
