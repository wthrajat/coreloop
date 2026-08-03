package tursohttp

import (
	"bytes"
	"context"
	"database/sql/driver"
	"io"
	"net/http"
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
			Body:       io.NopCloser(bytes.NewBufferString(`{"results":[{"type":"ok","response":{"type":"execute","result":{"cols":[],"rows":[],"affected_row_count":1,"last_insert_rowid":"4"}}},{"type":"ok","response":{"type":"close"}}]}`)),
			Header:     make(http.Header),
		}, nil
	})}
	database, err := Open("libsql://example.turso.io", "secret", client)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	result, err := database.ExecContext(context.Background(), "INSERT INTO test VALUES (?, ?)", "value", 7)
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
}

func TestDecodeArgument(t *testing.T) {
	value, err := decodeArgument(argument{Type: "integer", Value: "42"})
	if err != nil || value != driver.Value(int64(42)) {
		t.Fatalf("value=%v err=%v", value, err)
	}
}
