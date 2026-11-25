package mailx

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ResendAPI è l'endpoint predefinito dell'API Resend.
const ResendAPI = "https://api.resend.com/emails"

// Resend invia tramite l'API HTTP di Resend (nessun SDK necessario).
type Resend struct {
	APIKey string
	Options
	// Endpoint e Client sono sostituibili nei test.
	Endpoint string
	Client   *http.Client
}

// NewResend crea un Sender Resend.
func NewResend(apiKey string, o Options) *Resend {
	return &Resend{APIKey: apiKey, Options: o}
}

func (r *Resend) Send(to, subject, textBody, htmlBody string) error {
	m, err := r.prepare(to, subject, textBody, htmlBody)
	if err != nil {
		return err
	}
	if strings.TrimSpace(r.APIKey) == "" || strings.TrimSpace(m.from) == "" {
		return errors.New("resend: API key o mittente mancanti")
	}
	payload, _ := json.Marshal(map[string]any{
		"from": m.from, "to": []string{m.to}, "subject": m.subject, "text": m.text, "html": m.html,
	})
	endpoint := r.Endpoint
	if endpoint == "" {
		endpoint = ResendAPI
	}
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+r.APIKey)
	req.Header.Set("Content-Type", "application/json")
	client := r.Client
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("resend: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if resp.StatusCode/100 != 2 {
		var e struct {
			Message string `json:"message"`
		}
		_ = json.Unmarshal(body, &e)
		if e.Message == "" {
			e.Message = strings.TrimSpace(string(body))
		}
		return fmt.Errorf("resend: HTTP %d: %s", resp.StatusCode, e.Message)
	}
	var ok struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(body, &ok); err != nil || ok.ID == "" {
		return errors.New("resend: risposta senza id")
	}
	return nil
}
