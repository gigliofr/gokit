// Package storagex salva e legge file (allegati, immagini, modelli 3D) con
// regole comuni: nomi sicuri, estensioni ammesse, dimensione massima e
// scrittura atomica. L'implementazione Local usa una cartella (es. un volume
// Coolify); il modulo separato storagex/s3 aggiunge gli URL firmati S3/MinIO.
package storagex

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

// Storage è l'interfaccia comune ai backend.
type Storage interface {
	// Put scrive (sovrascrivendo) il contenuto di r sotto key.
	Put(ctx context.Context, key string, r io.Reader) (Object, error)
	// Open apre key in lettura; ErrNotFound se non esiste.
	Open(ctx context.Context, key string) (io.ReadCloser, error)
	// Delete elimina key; non è un errore se non esiste.
	Delete(ctx context.Context, key string) error
	// Exists indica se key esiste.
	Exists(ctx context.Context, key string) bool
}

// Object descrive un file salvato.
type Object struct {
	Key  string
	Size int64
}

var (
	ErrNotFound    = errors.New("file non trovato")
	ErrInvalidKey  = errors.New("nome file non valido")
	ErrTooLarge    = errors.New("file troppo grande")
	ErrExtNotAllow = errors.New("estensione non ammessa")
)

// Policy definisce le regole di un archivio.
type Policy struct {
	// AllowedExt: estensioni ammesse in minuscolo con il punto (".png");
	// vuota = qualsiasi estensione.
	AllowedExt []string
	// MaxBytes: dimensione massima di un file; 0 = nessun limite.
	MaxBytes int64
	// FallbackStem: nome usato da SafeName quando non resta nessun carattere
	// valido (default "file").
	FallbackStem string
}

// Allowed indica se l'estensione (es. ".PNG") è ammessa.
func (p Policy) Allowed(ext string) bool {
	if len(p.AllowedExt) == 0 {
		return true
	}
	ext = strings.ToLower(ext)
	for _, a := range p.AllowedExt {
		if strings.ToLower(a) == ext {
			return true
		}
	}
	return false
}

func (p Policy) extErr(ext string) error {
	return fmt.Errorf("%w: %q", ErrExtNotAllow, ext)
}

func (p Policy) sizeErr() error {
	return fmt.Errorf("%w (max %d MB)", ErrTooLarge, p.MaxBytes>>20)
}

// SafeName trasforma un nome scelto dall'utente in un nome sicuro: toglie i
// percorsi, tiene solo lettere, cifre, '-' e '_' (spazi e punti diventano '_')
// e verifica che l'estensione sia ammessa.
func (p Policy) SafeName(name string) (string, error) {
	base := filepath.Base(strings.ReplaceAll(name, "\\", "/"))
	ext := strings.ToLower(filepath.Ext(base))
	if !p.Allowed(ext) {
		return "", p.extErr(ext)
	}
	var b strings.Builder
	for _, r := range strings.TrimSuffix(base, filepath.Ext(base)) {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		case r == ' ' || r == '.':
			b.WriteRune('_')
		}
	}
	stem := b.String()
	if stem == "" {
		stem = p.FallbackStem
		if stem == "" {
			stem = "file"
		}
	}
	return stem + ext, nil
}

// ValidKey indica se key è un nome di file piatto e sicuro: niente percorsi,
// solo lettere, cifre, '.', '-' e '_', non inizia con '.', al massimo 200 caratteri.
func ValidKey(key string) bool {
	if key == "" || len(key) > 200 || key[0] == '.' {
		return false
	}
	for _, r := range key {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_', r == '.':
		default:
			return false
		}
	}
	return !strings.Contains(key, "..")
}

// NewKey genera una chiave univoca con l'estensione indicata (es. ".pdf").
func NewKey(ext string) string { return uuid.NewString() + strings.ToLower(ext) }

// ExtFromMIME ricava l'estensione dai tipi più comuni ("" se sconosciuto).
func ExtFromMIME(mimeType string) string {
	m := strings.ToLower(mimeType)
	switch {
	case strings.Contains(m, "jpeg"), strings.Contains(m, "jpg"):
		return ".jpg"
	case strings.Contains(m, "png"):
		return ".png"
	case strings.Contains(m, "webp"):
		return ".webp"
	case strings.Contains(m, "gif"):
		return ".gif"
	case strings.Contains(m, "heic"):
		return ".heic"
	case strings.Contains(m, "pdf"):
		return ".pdf"
	case strings.Contains(m, "csv"):
		return ".csv"
	case strings.HasPrefix(m, "text/plain"):
		return ".txt"
	}
	return ""
}
