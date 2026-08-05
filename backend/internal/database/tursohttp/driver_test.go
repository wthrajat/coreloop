package tursohttp

import (
	"bytes"
	"context"
	"database/sql/driver"
	"encoding/json"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func TestOpenExecutesPipelineWithTypedArguments(t *testing.T) {
	var received []byte
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		received, _ = io.ReadAll(request.Body)
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString(`{"results":[{"type":"ok","response":{"type":"execute","result":{"cols":[],"rows":[],"affected_row_count":0}}},{"type":"ok","response":{"type":"execute","result":{"cols":[],"rows":[],"affected_row_count":1,"last_insert_rowid":"4"}}},{"type":"ok","response":{"type":"close"}}]}`)),
			Header:     make(http.Header),
		}, nil
	})}
	database, err := Open("libsql://example.turso.io", "secret", client)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	result, err := database.ExecContext(context.Background(), "INSERT INTO test VALUES (?, ?, ?)", "value", 7, "")
	if err != nil {
		t.Fatal(err)
	}
	affected, _ := result.RowsAffected()
	if affected != 1 {
		t.Fatalf("affected rows = %d", affected)
	}
	if !bytes.Contains(received, []byte(`"type":"integer","value":"7"`)) {
		t.Fatalf("typed integer was not encoded: %s", received)
	}
	if !bytes.Contains(received, []byte(`"type":"text","value":""`)) {
		t.Fatalf("empty text was not encoded: %s", received)
	}
	var payload pipelinePayload
	if err := json.Unmarshal(received, &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Requests) != 3 || payload.Requests[0].Statement == nil ||
		payload.Requests[0].Statement.SQL != "PRAGMA foreign_keys=ON" {
		t.Fatalf("foreign keys were not enabled first: %s", received)
	}
}

func TestOpenFailsWhenForeignKeysCannotBeEnabled(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body: io.NopCloser(bytes.NewBufferString(
				`{"results":[{"type":"error","error":{"message":"pragma rejected"}}]}`,
			)),
			Header: make(http.Header),
		}, nil
	})}
	database, err := Open("libsql://example.turso.io", "secret", client)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	_, err = database.ExecContext(context.Background(), "DELETE FROM users")
	if err == nil || !strings.Contains(err.Error(), "enable Turso foreign keys") {
		t.Fatalf("foreign-key initialization error = %v", err)
	}
}

func TestDecodeArgument(t *testing.T) {
	testCases := []struct {
		name string
		json string
		want driver.Value
	}{
		{name: "integer", json: `{"type":"integer","value":"42"}`, want: int64(42)},
		{name: "float", json: `{"type":"float","value":1.25}`, want: float64(1.25)},
		{name: "empty text", json: `{"type":"text","value":""}`, want: ""},
		{name: "blob", json: `{"type":"blob","base64":"+w"}`, want: []byte{251}},
		{name: "null", json: `{"type":"null"}`, want: nil},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			var encoded argument
			if err := json.Unmarshal([]byte(testCase.json), &encoded); err != nil {
				t.Fatal(err)
			}
			value, err := decodeArgument(encoded)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(value, testCase.want) {
				t.Fatalf("value = %#v, want %#v", value, testCase.want)
			}
		})
	}
}

func TestEncodeArgumentUsesHranaWireFormat(t *testing.T) {
	testCases := []struct {
		name  string
		value any
		want  string
	}{
		{name: "text", value: "", want: `{"type":"text","value":""}`},
		{name: "empty blob", value: []byte{}, want: `{"type":"blob","base64":""}`},
		{name: "blob", value: []byte{251}, want: `{"type":"blob","base64":"+w"}`},
		{name: "float", value: 1.25, want: `{"type":"float","value":1.25}`},
		{name: "zero float", value: 0.0, want: `{"type":"float","value":0}`},
		{name: "null", value: nil, want: `{"type":"null"}`},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			argumentValue, err := encodeArgument(testCase.value)
			if err != nil {
				t.Fatal(err)
			}
			encoded, err := json.Marshal(argumentValue)
			if err != nil {
				t.Fatal(err)
			}
			if string(encoded) != testCase.want {
				t.Fatalf("encoded argument = %s, want %s", encoded, testCase.want)
			}
		})
	}
}
