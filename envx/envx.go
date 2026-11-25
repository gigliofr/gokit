// Package envx legge variabili d'ambiente tipizzate con valori di default.
//
// Tutti i valori vengono ripuliti dagli spazi; una variabile vuota è trattata
// come non impostata.
package envx

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// String restituisce il valore di key oppure def se assente o vuoto.
func String(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

// First restituisce il valore della prima variabile non vuota tra keys,
// oppure "" se nessuna è impostata. Utile per alias storici
// (es. MONGODB_URI, MONGO_URL, DATABASE_URL).
func First(keys ...string) string {
	for _, k := range keys {
		if v := strings.TrimSpace(os.Getenv(k)); v != "" {
			return v
		}
	}
	return ""
}

// Int restituisce key come intero, oppure def se assente o non valido.
func Int(key string, def int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

// Int64 restituisce key come int64, oppure def se assente o non valido.
func Int64(key string, def int64) int64 {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return def
	}
	return n
}

// Bool restituisce key come booleano. Accetta 1/0, true/false, yes/no, on/off
// (senza distinzione di maiuscole); altrimenti def.
func Bool(key string, def bool) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(key))) {
	case "1", "true", "yes", "on", "si", "sì":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return def
	}
}

// Duration restituisce key come time.Duration ("15m", "1h30m").
// Un numero senza unità è interpretato come secondi. Altrimenti def.
func Duration(key string, def time.Duration) time.Duration {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	if d, err := time.ParseDuration(v); err == nil {
		return d
	}
	if n, err := strconv.Atoi(v); err == nil {
		return time.Duration(n) * time.Second
	}
	return def
}

// List restituisce key divisa per virgole, senza elementi vuoti; def se assente.
func List(key string, def []string) []string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// Required restituisce il valore di key o un errore se assente.
func Required(key string) (string, error) {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v, nil
	}
	return "", fmt.Errorf("variabile d'ambiente %s obbligatoria", key)
}

// LoadDotEnv carica un file .env (KEY=VALUE per riga, # per i commenti).
// Le variabili già presenti nell'ambiente non vengono sovrascritte.
// Se il file non esiste non fa nulla e restituisce nil.
func LoadDotEnv(path string) error {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		k, v, ok := strings.Cut(line, "=")
		k = strings.TrimSpace(k)
		if !ok || k == "" {
			continue
		}
		v = strings.Trim(strings.TrimSpace(v), `"'`)
		if _, exists := os.LookupEnv(k); !exists || os.Getenv(k) == "" {
			_ = os.Setenv(k, v)
		}
	}
	return sc.Err()
}
