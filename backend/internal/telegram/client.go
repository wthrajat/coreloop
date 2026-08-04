package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const maxMessageCharacters = 4096

type Client struct {
	token   string
	baseURL string
	http    *http.Client
}

type APIError struct {
	Method      string
	Code        int
	Description string
}

func (failure *APIError) Error() string {
	return fmt.Sprintf("Telegram %s failed: %s", failure.Method, failure.Description)
}

func IsChatUnavailable(err error) bool {
	var failure *APIError
	if !errors.As(err, &failure) {
		return false
	}
	description := strings.ToLower(failure.Description)
	for _, message := range []string{
		"chat not found",
		"bot was blocked by the user",
		"user is deactivated",
		"bot can't initiate conversation with a user",
	} {
		if strings.Contains(description, message) {
			return true
		}
	}
	return false
}

func New(token string, client *http.Client) *Client {
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	return &Client{token: token, baseURL: "https://api.telegram.org", http: client}
}

func (client *Client) WithBaseURL(baseURL string) *Client {
	copy := *client
	copy.baseURL = strings.TrimRight(baseURL, "/")
	return &copy
}

type Button struct {
	Text string `json:"text"`
	Data string `json:"callback_data"`
}

type MessageOptions struct {
	Buttons [][]Button
}

func (client *Client) SendMessage(ctx context.Context, chatID, text string, options MessageOptions) (string, error) {
	if len([]rune(text)) > maxMessageCharacters {
		return "", errors.New("Telegram message exceeds 4096 characters")
	}
	payload := map[string]any{
		"chat_id":              chatID,
		"text":                 text,
		"parse_mode":           "HTML",
		"link_preview_options": map[string]bool{"is_disabled": true},
	}
	if len(options.Buttons) > 0 {
		payload["reply_markup"] = map[string]any{"inline_keyboard": options.Buttons}
	}
	var response struct {
		OK          bool   `json:"ok"`
		Description string `json:"description"`
		Result      struct {
			MessageID int64 `json:"message_id"`
		} `json:"result"`
	}
	if err := client.call(ctx, "sendMessage", payload, &response); err != nil {
		return "", err
	}
	return strconv.FormatInt(response.Result.MessageID, 10), nil
}

func (client *Client) AnswerCallback(ctx context.Context, callbackID, text string) error {
	payload := map[string]any{"callback_query_id": callbackID, "text": text}
	return client.call(ctx, "answerCallbackQuery", payload, nil)
}

func (client *Client) ValidateChat(ctx context.Context, chatID string) error {
	return client.call(ctx, "getChat", map[string]string{"chat_id": chatID}, nil)
}

func (client *Client) SetWebhook(ctx context.Context, webhookURL, secret string) error {
	if _, err := url.ParseRequestURI(webhookURL); err != nil {
		return fmt.Errorf("invalid webhook URL: %w", err)
	}
	payload := map[string]any{
		"url":                  webhookURL,
		"secret_token":         secret,
		"allowed_updates":      []string{"callback_query"},
		"drop_pending_updates": false,
	}
	return client.call(ctx, "setWebhook", payload, nil)
}

func (client *Client) call(ctx context.Context, method string, payload any, destination any) error {
	if strings.TrimSpace(client.token) == "" {
		return errors.New("Telegram bot token is not configured")
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	endpoint := client.baseURL + "/bot" + client.token + "/" + method
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(encoded))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := client.http.Do(request)
	if err != nil {
		return fmt.Errorf("call Telegram %s: %w", method, err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return err
	}
	var envelope struct {
		OK          bool   `json:"ok"`
		ErrorCode   int    `json:"error_code"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return fmt.Errorf("decode Telegram response: %w", err)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 || !envelope.OK {
		return &APIError{Method: method, Code: envelope.ErrorCode, Description: envelope.Description}
	}
	if destination != nil {
		if err := json.Unmarshal(body, destination); err != nil {
			return fmt.Errorf("decode Telegram result: %w", err)
		}
	}
	return nil
}

type Update struct {
	ID            int64          `json:"update_id"`
	CallbackQuery *CallbackQuery `json:"callback_query,omitempty"`
}

type CallbackQuery struct {
	ID      string `json:"id"`
	Data    string `json:"data"`
	From    User   `json:"from"`
	Message *struct {
		Chat struct {
			ID int64 `json:"id"`
		} `json:"chat"`
	} `json:"message,omitempty"`
}

type User struct {
	ID int64 `json:"id"`
}
