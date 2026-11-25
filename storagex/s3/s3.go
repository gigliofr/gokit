// Package s3 genera URL firmati (presigned) per caricare e scaricare file da
// S3 o MinIO direttamente dal browser, senza far passare i file dall'app.
//
// È un modulo separato da gokit perché minio-go richiede Go 1.23:
//
//	go get github.com/gigliofr/gokit/storagex/s3@v0.1.0
package s3

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// Config descrive la connessione al bucket.
type Config struct {
	Endpoint  string // es. "s3.eu-central-1.amazonaws.com" o "minio.example.it:9000"
	Bucket    string
	AccessKey string
	SecretKey string
	// Region: se impostata la firma degli URL non richiede chiamate di rete.
	Region string
	UseSSL bool
	// Expires: validità degli URL firmati (default 15 minuti).
	Expires time.Duration
}

// ConfigFromEnv legge S3_ENDPOINT, S3_BUCKET, S3_ACCESS_KEY, S3_SECRET_KEY,
// S3_REGION e S3_USE_SSL (default true). ok è false se manca uno dei primi quattro.
func ConfigFromEnv() (cfg Config, ok bool) {
	get := func(k string) string { return strings.TrimSpace(os.Getenv(k)) }
	cfg = Config{
		Endpoint:  get("S3_ENDPOINT"),
		Bucket:    get("S3_BUCKET"),
		AccessKey: get("S3_ACCESS_KEY"),
		SecretKey: get("S3_SECRET_KEY"),
		Region:    get("S3_REGION"),
		UseSSL:    true,
	}
	if v, err := strconv.ParseBool(get("S3_USE_SSL")); err == nil {
		cfg.UseSSL = v
	}
	ok = cfg.Endpoint != "" && cfg.Bucket != "" && cfg.AccessKey != "" && cfg.SecretKey != ""
	return cfg, ok
}

// Presigner firma gli URL per un bucket.
type Presigner struct {
	client  *minio.Client
	bucket  string
	expires time.Duration
}

// New crea il Presigner (non contatta il server: usare CheckBucket per verificarlo).
func New(cfg Config) (*Presigner, error) {
	if cfg.Endpoint == "" || cfg.Bucket == "" {
		return nil, errors.New("s3: endpoint e bucket sono obbligatori")
	}
	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
		Region: cfg.Region,
	})
	if err != nil {
		return nil, fmt.Errorf("s3: client: %w", err)
	}
	exp := cfg.Expires
	if exp <= 0 {
		exp = 15 * time.Minute
	}
	return &Presigner{client: client, bucket: cfg.Bucket, expires: exp}, nil
}

// CheckBucket verifica che il bucket esista e sia raggiungibile.
func (p *Presigner) CheckBucket(ctx context.Context) error {
	ok, err := p.client.BucketExists(ctx, p.bucket)
	if err != nil {
		return fmt.Errorf("s3: verifica bucket: %w", err)
	}
	if !ok {
		return fmt.Errorf("s3: bucket %q non trovato", p.bucket)
	}
	return nil
}

// Expires restituisce la validità degli URL firmati.
func (p *Presigner) Expires() time.Duration { return p.expires }

// PresignPut restituisce l'URL per caricare key con una PUT e la sua scadenza.
func (p *Presigner) PresignPut(ctx context.Context, key string) (string, time.Time, error) {
	if strings.TrimSpace(key) == "" {
		return "", time.Time{}, errors.New("s3: chiave obbligatoria")
	}
	u, err := p.client.PresignedPutObject(ctx, p.bucket, key, p.expires)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("s3: firma upload: %w", err)
	}
	return u.String(), time.Now().UTC().Add(p.expires), nil
}

// PresignGet restituisce l'URL per scaricare key. Con downloadName il browser
// salva il file con quel nome (Content-Disposition: attachment).
func (p *Presigner) PresignGet(ctx context.Context, key, downloadName string) (string, time.Time, error) {
	if strings.TrimSpace(key) == "" {
		return "", time.Time{}, errors.New("s3: chiave obbligatoria")
	}
	params := url.Values{}
	if safe := SafeFileName(downloadName); safe != "" {
		params.Set("response-content-disposition", `attachment; filename="`+safe+`"`)
	}
	u, err := p.client.PresignedGetObject(ctx, p.bucket, key, p.expires, params)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("s3: firma download: %w", err)
	}
	return u.String(), time.Now().UTC().Add(p.expires), nil
}

// SafeFileName rende un nome file scelto dall'utente sicuro come ultimo
// elemento di una chiave e dentro Content-Disposition: niente separatori di
// percorso, "..", virgolette o caratteri di controllo.
func SafeFileName(name string) string {
	name = strings.TrimSpace(name)
	var b strings.Builder
	for _, r := range name {
		switch {
		case r == '/' || r == '\\' || r == '"' || r < 0x20 || r == 0x7f:
			b.WriteRune('_')
		default:
			b.WriteRune(r)
		}
	}
	return strings.ReplaceAll(b.String(), "..", "_")
}

// Key unisce gli elementi in una chiave "a/b/c", ripulendo ciascun elemento.
func Key(parts ...string) string {
	clean := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = SafeFileName(p); p != "" {
			clean = append(clean, p)
		}
	}
	return path.Join(clean...)
}
