package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

type telegramRoundTripFunc func(*http.Request) (*http.Response, error)

func (function telegramRoundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func TestSendMessageClassifiesAnUnavailableChat(t *testing.T) {
	client := New("test-token", &http.Client{Transport: telegramRoundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusBadRequest,
			Body: io.NopCloser(bytes.NewBufferString(
				`{"ok":false,"error_code":400,"description":"Bad Request: chat not found"}`,
			)),
			Header: make(http.Header),
		}, nil
	})})

	_, err := client.SendMessage(context.Background(), "987654321", "test", MessageOptions{})
	if !IsChatUnavailable(err) {
		t.Fatalf("error was not classified as an unavailable chat: %v", err)
	}
}

func TestSendMessageDoesNotMisclassifyContentErrors(t *testing.T) {
	client := New("test-token", &http.Client{Transport: telegramRoundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusBadRequest,
			Body: io.NopCloser(bytes.NewBufferString(
				`{"ok":false,"error_code":400,"description":"Bad Request: can't parse entities"}`,
			)),
			Header: make(http.Header),
		}, nil
	})})

	_, err := client.SendMessage(context.Background(), "987654321", "<broken", MessageOptions{})
	if err == nil || IsChatUnavailable(err) {
		t.Fatalf("content error classification = %v", err)
	}
}

func TestSendMessageSupportsSourceURLButtons(t *testing.T) {
	client := New("test-token", &http.Client{Transport: telegramRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		var payload struct {
			ReplyMarkup struct {
				Keyboard [][]map[string]string `json:"inline_keyboard"`
			} `json:"reply_markup"`
		}
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		button := payload.ReplyMarkup.Keyboard[0][0]
		if button["url"] != "https://go.dev/blog/source" || button["callback_data"] != "" {
			t.Fatalf("source button = %#v", button)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString(`{"ok":true,"result":{"message_id":42}}`)),
			Header:     make(http.Header),
		}, nil
	})})
	_, err := client.SendMessage(context.Background(), "987654321", "Source", MessageOptions{
		Buttons: [][]Button{{{Text: "Open source", URL: "https://go.dev/blog/source"}}},
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestSendMessageDoesNotExposeTokenOnTransportFailure(t *testing.T) {
	token := "super-secret-bot-token"
	client := New(token, &http.Client{Transport: telegramRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		return nil, &url.Error{Op: "Post", URL: request.URL.String(), Err: errors.New("network unavailable")}
	})})

	_, err := client.SendMessage(context.Background(), "987654321", "test", MessageOptions{})
	if err == nil {
		t.Fatal("expected transport failure")
	}
	if strings.Contains(err.Error(), token) || strings.Contains(err.Error(), "/bot") {
		t.Fatalf("Telegram credential leaked in error: %v", err)
	}
}

func TestSendMessageDoesNotExposeMalformedTokenInRequestError(t *testing.T) {
	token := "secret-token-with-invalid-%-escape"
	client := New(token, nil)

	_, err := client.SendMessage(context.Background(), "987654321", "test", MessageOptions{})
	if err == nil {
		t.Fatal("expected request construction failure")
	}
	if strings.Contains(err.Error(), token) || strings.Contains(err.Error(), "/bot") {
		t.Fatalf("Telegram credential leaked in error: %v", err)
	}
}

func TestSendMessageRejectsUnsafeButtonURL(t *testing.T) {
	client := New("test-token", nil)
	_, err := client.SendMessage(context.Background(), "987654321", "Source", MessageOptions{
		Buttons: [][]Button{{{Text: "Open source", URL: "https://127.0.0.1/admin"}}},
	})
	if err == nil {
		t.Fatal("expected private destination to be rejected")
	}
}

func TestValidateChatUsesTelegramGetChat(t *testing.T) {
	client := New("test-token", &http.Client{Transport: telegramRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path != "/bottest-token/getChat" {
			t.Fatalf("request path = %q", request.URL.Path)
		}
		var payload map[string]string
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if payload["chat_id"] != "987654321" {
			t.Fatalf("chat ID = %q", payload["chat_id"])
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString(`{"ok":true,"result":{"id":987654321}}`)),
			Header:     make(http.Header),
		}, nil
	})})

	if err := client.ValidateChat(context.Background(), "987654321"); err != nil {
		t.Fatal(err)
	}
}
