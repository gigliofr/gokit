package httpx

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestJSONAndError(t *testing.T) {
	rec := httptest.NewRecorder()
	JSON(rec, http.StatusCreated, map[string]int{"n": 1})
	if rec.Code != 201 || !strings.HasPrefix(rec.Header().Get("Content-Type"), "application/json") || strings.TrimSpace(rec.Body.String()) != `{"n":1}` {
		t.Fatalf("JSON: %d %q", rec.Code, rec.Body.String())
	}
	rec = httptest.NewRecorder()
	ErrorCode(rec, 400, "bad_input", "dati non validi")
	var b ErrorBody
	_ = json.Unmarshal(rec.Body.Bytes(), &b)
	if rec.Code != 400 || b.Error != "dati non validi" || b.Code != "bad_input" {
		t.Fatalf("ErrorCode: %+v", b)
	}
	rec = httptest.NewRecorder()
	Error(rec, 404, "x")
	if strings.Contains(rec.Body.String(), "code") {
		t.Fatal("code vuoto non deve comparire")
	}
	rec = httptest.NewRecorder()
	JSON(rec, http.StatusNoContent, map[string]int{"n": 1})
	if rec.Body.Len() != 0 {
		t.Fatal("204 non deve avere body")
	}
}

func TestDecode(t *testing.T) {
	type in struct {
		Name string `json:"name"`
	}
	req := func(body string) *http.Request { return httptest.NewRequest("POST", "/", strings.NewReader(body)) }
	var v in
	if err := Decode(nil, req(`{"name":"a"}`), &v); err != nil || v.Name != "a" {
		t.Fatalf("ok: %v %+v", err, v)
	}
	if err := Decode(nil, req(`{"name":"a","x":1}`), &v); err == nil {
		t.Fatal("campo sconosciuto deve fallire")
	}
	if err := Decode(nil, req(`{"name":"a","x":1}`), &v, DecodeOptions{AllowUnknownFields: true}); err != nil {
		t.Fatal(err)
	}
	if err := Decode(nil, req(``), &v); err == nil {
		t.Fatal("body vuoto deve fallire")
	}
	if err := Decode(nil, req(`{"name":"a"}{"name":"b"}`), &v); err == nil {
		t.Fatal("due oggetti devono fallire")
	}
	big := `{"name":"` + strings.Repeat("a", 100) + `"}`
	if err := Decode(httptest.NewRecorder(), req(big), &v, DecodeOptions{MaxBytes: 20}); !errors.Is(err, ErrBodyTooLarge) {
		t.Fatalf("atteso ErrBodyTooLarge, ottenuto %v", err)
	}
}

func TestBearerAndClientIP(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("Authorization", "bearer  abc ")
	if BearerToken(r) != "abc" {
		t.Fatal("BearerToken")
	}
	r.Header.Set("Authorization", "Basic x")
	if BearerToken(r) != "" {
		t.Fatal("Basic non è bearer")
	}
	r.RemoteAddr = "10.0.0.1:1234"
	r.Header.Set("X-Forwarded-For", "nonip, 203.0.113.9, 10.0.0.2")
	if ClientIP(r, false) != "10.0.0.1" {
		t.Fatal("senza trustProxy deve usare RemoteAddr")
	}
	if ClientIP(r, true) != "203.0.113.9" {
		t.Fatalf("XFF: %s", ClientIP(r, true))
	}
	r.Header.Del("X-Forwarded-For")
	r.Header.Set("X-Real-IP", "198.51.100.1")
	if ClientIP(r, true) != "198.51.100.1" {
		t.Fatal("X-Real-IP")
	}
}

func TestRunGracefulShutdown(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	ln.Close()
	srv := NewServer(addr, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(204) }))
	if srv.ReadHeaderTimeout == 0 {
		t.Fatal("ReadHeaderTimeout mancante")
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- Run(ctx, srv, 2*time.Second) }()
	var resp *http.Response
	for i := 0; i < 50; i++ {
		if resp, err = http.Get("http://" + addr); err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err != nil || resp.StatusCode != 204 {
		t.Fatalf("server non raggiungibile: %v", err)
	}
	resp.Body.Close()
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("shutdown non completato")
	}
}

func TestRunListenError(t *testing.T) {
	srv := NewServer("256.0.0.1:99999", http.NotFoundHandler())
	if err := Run(context.Background(), srv, time.Second); err == nil {
		t.Fatal("atteso errore di avvio")
	}
}
