package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
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
