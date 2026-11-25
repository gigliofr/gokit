package httpx

import (
	"context"
	"errors"
	"net/http"
	"time"
)

// NewServer crea un http.Server con timeout prudenti:
// ReadHeaderTimeout 10s (protegge da slowloris) e IdleTimeout 120s.
// ReadTimeout e WriteTimeout restano 0 per non interrompere upload lenti,
// SSE e download lunghi: limitare il body con httpx.Decode o http.MaxBytesReader.
func NewServer(addr string, h http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           h,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}
}

// Run avvia srv e lo arresta in modo ordinato quando ctx viene annullato
// (tipicamente da signal.NotifyContext), attendendo al massimo shutdownTimeout.
// Restituisce nil dopo uno shutdown regolare, altrimenti l'errore di avvio.
func Run(ctx context.Context, srv *http.Server, shutdownTimeout time.Duration) error {
	errCh := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()
	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}
	sctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(sctx); err != nil {
		return err
	}
	return <-errCh
}
