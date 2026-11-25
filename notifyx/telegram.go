package notifyx

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
	"unicode/utf8"
)

// TelegramAPI è la base dell'API Bot di Telegram.
const TelegramAPI = "https://api.telegram.org"

// maxTelegramText è il limite di caratteri di un messaggio Telegram.
const maxTelegramText = 4096

// Telegram invia messaggi a una chat tramite un bot.
type Telegram struct {
	Token  string
	ChatID string
	// ParseMode: "" (testo semplice, default), "HTML" o "MarkdownV2".
	ParseMode string
	// Silent invia senza suono di notifica.
	Silent bool
	// BaseURL e Client sono sostituibili nei test.
	BaseURL string
	Client  *http.Client
}

// NewTelegram crea un canale Telegram.
func NewTelegram(token, chatID string) *Telegram {
	return &Telegram{Token: strings.TrimSpace(token), ChatID: strings.TrimSpace(chatID)}
}

// TelegramFromEnv legge <prefix>TELEGRAM_BOT_TOKEN e <prefix>TELEGRAM_CHAT_ID.
// Restituisce nil se una delle due manca.
func TelegramFromEnv(prefix string) *Telegram {
	token := strings.TrimSpace(os.Getenv(prefix + "TELEGRAM_BOT_TOKEN"))
	chat := strings.TrimSpace(os.Getenv(prefix + "TELEGRAM_CHAT_ID"))
	if token == "" || chat == "" {
		return nil
	}
	return NewTelegram(token, chat)
}

// Enabled indica se token e chat sono impostati (nil-safe).
func (t *Telegram) Enabled() bool { return t != nil && t.Token != "" && t.ChatID != "" }

// Notify invia text alla chat. I messaggi oltre 4096 caratteri vengono troncati.
func (t *Telegram) Notify(ctx context.Context, text string) error {
	if !t.Enabled() {
		return errors.New("telegram: token o chat non configurati")
	}
	if utf8.RuneCountInString(text) > maxTelegramText {
		r := []rune(text)
		text = string(r[:maxTelegramText-1]) + "…"
	}
	body := map[string]any{
		"chat_id":                  t.ChatID,
		"text":                     text,
		"disable_web_page_preview": true,
		"disable_notification":     t.Silent,
	}
	if t.ParseMode != "" {
		body["parse_mode"] = t.ParseMode
	}
	payload, _ := json.Marshal(body)
	base := t.BaseURL
	if base == "" {
		base = TelegramAPI
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/bot"+t.Token+"/sendMessage", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	client := t.Client
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		// L'URL contiene il token: non va mai riportato negli errori.
		var ue interface{ Unwrap() error }
		if errors.As(err, &ue) {
			err = ue.Unwrap()
		}
		return fmt.Errorf("telegram: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		var e struct {
			Description string `json:"description"`
		}
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 16<<10))
		_ = json.Unmarshal(b, &e)
		return fmt.Errorf("telegram: HTTP %d: %s", resp.StatusCode, e.Description)
	}
	return nil
}
