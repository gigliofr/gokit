// Package mongox centralizza la connessione a MongoDB: timeout, ping sul
// primario, chiusura del client se il ping fallisce e creazione degli indici.
package mongox

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

// DefaultConnectTimeout è il tempo massimo concesso a connessione e ping.
const DefaultConnectTimeout = 10 * time.Second

type config struct {
	timeout time.Duration
	appName string
	extra   []*options.ClientOptions
}

// Option configura Connect.
type Option func(*config)

// WithTimeout imposta il timeout di connessione e ping.
func WithTimeout(d time.Duration) Option { return func(c *config) { c.timeout = d } }

// WithAppName imposta il nome dell'applicazione visibile nei log di MongoDB/Atlas.
func WithAppName(name string) Option { return func(c *config) { c.appName = name } }

// WithClientOptions aggiunge opzioni del driver (applicate dopo l'URI).
func WithClientOptions(o *options.ClientOptions) Option {
	return func(c *config) { c.extra = append(c.extra, o) }
}

// Connect apre la connessione e verifica il primario con un ping.
// Se il ping fallisce il client viene chiuso prima di restituire l'errore.
func Connect(ctx context.Context, uri string, opts ...Option) (*mongo.Client, error) {
	uri = strings.TrimSpace(uri)
	if uri == "" {
		return nil, errors.New("mongox: URI vuoto")
	}
	cfg := config{timeout: DefaultConnectTimeout}
	for _, o := range opts {
		o(&cfg)
	}
	clientOpts := []*options.ClientOptions{options.Client().ApplyURI(uri)}
	if cfg.appName != "" {
		clientOpts = append(clientOpts, options.Client().SetAppName(cfg.appName))
	}
	clientOpts = append(clientOpts, cfg.extra...)

	cctx, cancel := context.WithTimeout(ctx, cfg.timeout)
	defer cancel()

	client, err := mongo.Connect(cctx, clientOpts...)
	if err != nil {
		return nil, fmt.Errorf("mongox: connessione: %w", err)
	}
	if err := client.Ping(cctx, readpref.Primary()); err != nil {
		Disconnect(client, 5*time.Second)
		return nil, fmt.Errorf("mongox: ping: %w", err)
	}
	return client, nil
}

// Disconnect chiude il client entro timeout ignorando gli errori; nil è ammesso.
func Disconnect(client *mongo.Client, timeout time.Duration) {
	if client == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	_ = client.Disconnect(ctx)
}

// Ping verifica che il primario risponda: pensato per gli endpoint /healthz.
func Ping(ctx context.Context, client *mongo.Client) error {
	if client == nil {
		return errors.New("mongox: client non inizializzato")
	}
	return client.Ping(ctx, readpref.Primary())
}

// Indexes associa a ogni collezione gli indici da garantire.
type Indexes map[string][]mongo.IndexModel

// EnsureIndexes crea gli indici mancanti (CreateMany è idempotente per
// indici con la stessa definizione). L'errore indica la collezione coinvolta.
func EnsureIndexes(ctx context.Context, db *mongo.Database, idx Indexes) error {
	for coll, models := range idx {
		if len(models) == 0 {
			continue
		}
		if _, err := db.Collection(coll).Indexes().CreateMany(ctx, models); err != nil {
			return fmt.Errorf("mongox: indici su %s: %w", coll, err)
		}
	}
	return nil
}
