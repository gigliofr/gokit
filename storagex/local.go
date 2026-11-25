package storagex

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Local salva i file in una cartella. La scrittura passa da un file
// temporaneo rinominato alla fine: chi legge non vede mai file a metà.
type Local struct {
	root   string
	policy Policy
}

var _ Storage = (*Local)(nil)

// NewLocal crea la cartella root se manca.
func NewLocal(root string, p Policy) (*Local, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil, errors.New("storagex: cartella non indicata")
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, fmt.Errorf("storagex: %s: %w", root, err)
	}
	return &Local{root: root, policy: p}, nil
}

// Root restituisce la cartella dell'archivio.
func (l *Local) Root() string { return l.root }

// Policy restituisce le regole dell'archivio.
func (l *Local) Policy() Policy { return l.policy }

func (l *Local) path(key string) (string, error) {
	if !ValidKey(key) {
		return "", ErrInvalidKey
	}
	return filepath.Join(l.root, key), nil
}

func (l *Local) Put(ctx context.Context, key string, r io.Reader) (Object, error) {
	dst, err := l.path(key)
	if err != nil {
		return Object{}, err
	}
	if ext := filepath.Ext(key); !l.policy.Allowed(ext) {
		return Object{}, l.policy.extErr(strings.ToLower(ext))
	}
	tmp, err := os.CreateTemp(l.root, ".upload-*")
	if err != nil {
		return Object{}, err
	}
	src := r
	if l.policy.MaxBytes > 0 {
		src = io.LimitReader(r, l.policy.MaxBytes+1)
	}
	n, err := io.Copy(tmp, ctxReader{ctx, src})
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err == nil && l.policy.MaxBytes > 0 && n > l.policy.MaxBytes {
		err = l.policy.sizeErr()
	}
	if err != nil {
		_ = os.Remove(tmp.Name())
		return Object{}, err
	}
	if err := os.Rename(tmp.Name(), dst); err != nil {
		_ = os.Remove(tmp.Name())
		return Object{}, err
	}
	return Object{Key: key, Size: n}, nil
}

func (l *Local) Open(_ context.Context, key string) (io.ReadCloser, error) {
	p, err := l.path(key)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(p)
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNotFound
	}
	return f, err
}

func (l *Local) Delete(_ context.Context, key string) error {
	p, err := l.path(key)
	if err != nil {
		return err
	}
	if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func (l *Local) Exists(_ context.Context, key string) bool {
	p, err := l.path(key)
	if err != nil {
		return false
	}
	st, err := os.Stat(p)
	return err == nil && st.Mode().IsRegular()
}

// ctxReader interrompe la copia se il contesto viene annullato (upload abbandonati).
type ctxReader struct {
	ctx context.Context
	r   io.Reader
}

func (c ctxReader) Read(p []byte) (int, error) {
	if err := c.ctx.Err(); err != nil {
		return 0, err
	}
	return c.r.Read(p)
}
