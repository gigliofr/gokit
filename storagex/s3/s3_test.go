package s3

import (
	"context"
	"net/url"
	"testing"
	"time"
)

func newTest(t *testing.T) *Presigner {
	t.Helper()
	p, err := New(Config{Endpoint: "minio.local:9000", Bucket: "documenti", AccessKey: "AK", SecretKey: "SK", Region: "eu-south-1", Expires: 10 * time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestPresign(t *testing.T) {
	p := newTest(t)
	ctx := context.Background()
	put, exp, err := p.PresignPut(ctx, "pratiche/42/documenti/a.pdf")
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(put)
	if u.Scheme != "http" || u.Host != "minio.local:9000" || u.Path != "/documenti/pratiche/42/documenti/a.pdf" ||
		u.Query().Get("X-Amz-Signature") == "" || u.Query().Get("X-Amz-Expires") != "600" {
		t.Fatalf("URL upload: %s", put)
	}
	if time.Until(exp) > 10*time.Minute || time.Until(exp) < 9*time.Minute {
		t.Fatal("scadenza")
	}
	get, _, err := p.PresignGet(ctx, "pratiche/42/documenti/a.pdf", `Carta "identità".pdf`)
	if err != nil {
		t.Fatal(err)
	}
	gu, _ := url.Parse(get)
	if cd := gu.Query().Get("response-content-disposition"); cd != `attachment; filename="Carta _identità_.pdf"` {
		t.Fatalf("content-disposition: %q", cd)
	}
	if _, _, err := p.PresignPut(ctx, " "); err == nil {
		t.Fatal("chiave vuota")
	}
}

func TestHelpersAndConfig(t *testing.T) {
	if got := Key("pratiche", "../42", "doc/1.pdf"); got != "pratiche/__42/doc_1.pdf" {
		t.Fatalf("Key: %s", got)
	}
	if SafeFileName("a\r\nb\\c") != "a__b_c" {
		t.Fatal("SafeFileName")
	}
	t.Setenv("S3_ENDPOINT", "e")
	t.Setenv("S3_BUCKET", "b")
	t.Setenv("S3_ACCESS_KEY", "a")
	t.Setenv("S3_SECRET_KEY", "")
	t.Setenv("S3_USE_SSL", "false")
	if _, ok := ConfigFromEnv(); ok {
		t.Fatal("secret mancante")
	}
	t.Setenv("S3_SECRET_KEY", "s")
	cfg, ok := ConfigFromEnv()
	if !ok || cfg.UseSSL {
		t.Fatalf("config: %+v", cfg)
	}
	if _, err := New(Config{}); err == nil {
		t.Fatal("config vuota")
	}
}
