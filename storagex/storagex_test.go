package storagex

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSafeName(t *testing.T) {
	p := Policy{AllowedExt: []string{".stl", ".png"}, FallbackStem: "model"}
	cases := map[string]string{
		"../../etc/Drago Rosso.STL": "Drago_Rosso.stl",
		`C:\Users\x\mappa.v2.png`:   "mappa_v2.png",
		"???.png":                   "model.png",
	}
	for in, want := range cases {
		got, err := p.SafeName(in)
		if err != nil || got != want {
			t.Errorf("SafeName(%q) = %q, %v; atteso %q", in, got, err, want)
		}
	}
	if _, err := p.SafeName("virus.exe"); !errors.Is(err, ErrExtNotAllow) {
		t.Fatalf("estensione non ammessa: %v", err)
	}
	if got, _ := (Policy{}).SafeName("x/???"); got != "file" {
		t.Fatalf("fallback: %q", got)
	}
}

func TestValidKeyAndHelpers(t *testing.T) {
	for _, k := range []string{"a.png", "item-1_x.jpg", "3f2a.pdf"} {
		if !ValidKey(k) {
			t.Errorf("%q dovrebbe essere valida", k)
		}
	}
	for _, k := range []string{"", ".env", "../a", "a/b.png", `a\b`, "a..b", strings.Repeat("a", 201), "è.png"} {
		if ValidKey(k) {
			t.Errorf("%q non dovrebbe essere valida", k)
		}
	}
	if k := NewKey(".PDF"); !strings.HasSuffix(k, ".pdf") || !ValidKey(k) {
		t.Fatalf("NewKey: %s", k)
	}
	if ExtFromMIME("image/jpeg") != ".jpg" || ExtFromMIME("application/pdf") != ".pdf" || ExtFromMIME("x/y") != "" {
		t.Fatal("ExtFromMIME")
	}
}

func TestLocal(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	l, err := NewLocal(filepath.Join(dir, "sub"), Policy{AllowedExt: []string{".txt"}, MaxBytes: 10})
	if err != nil {
		t.Fatal(err)
	}
	obj, err := l.Put(ctx, "a.txt", strings.NewReader("ciao"))
	if err != nil || obj.Size != 4 || obj.Key != "a.txt" {
		t.Fatalf("Put: %+v %v", obj, err)
	}
	if !l.Exists(ctx, "a.txt") || l.Exists(ctx, "b.txt") || l.Exists(ctx, "../x") {
		t.Fatal("Exists")
	}
	rc, err := l.Open(ctx, "a.txt")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(rc)
	rc.Close()
	if string(b) != "ciao" {
		t.Fatalf("contenuto %q", b)
	}
	// sovrascrittura atomica
	if _, err := l.Put(ctx, "a.txt", strings.NewReader("nuovo")); err != nil {
		t.Fatal(err)
	}
	if _, err := l.Put(ctx, "big.txt", strings.NewReader(strings.Repeat("x", 11))); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("limite dimensione: %v", err)
	}
	if l.Exists(ctx, "big.txt") {
		t.Fatal("il file troppo grande non deve restare")
	}
	if _, err := l.Put(ctx, "a.exe", strings.NewReader("x")); !errors.Is(err, ErrExtNotAllow) {
		t.Fatal("estensione")
	}
	if _, err := l.Put(ctx, "../evil.txt", strings.NewReader("x")); !errors.Is(err, ErrInvalidKey) {
		t.Fatal("path traversal")
	}
	if _, err := l.Open(ctx, "nope.txt"); !errors.Is(err, ErrNotFound) {
		t.Fatal("ErrNotFound")
	}
	if err := l.Delete(ctx, "a.txt"); err != nil || l.Exists(ctx, "a.txt") {
		t.Fatal("Delete")
	}
	if err := l.Delete(ctx, "a.txt"); err != nil {
		t.Fatal("Delete di un file assente non è un errore")
	}
	entries, _ := os.ReadDir(l.Root())
	if len(entries) != 0 {
		t.Fatalf("file temporanei rimasti: %v", entries)
	}
	cctx, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := l.Put(cctx, "c.txt", strings.NewReader("x")); !errors.Is(err, context.Canceled) {
		t.Fatalf("contesto annullato: %v", err)
	}
	if _, err := NewLocal(" ", Policy{}); err == nil {
		t.Fatal("cartella vuota")
	}
}
