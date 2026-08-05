package store

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"coreloop/backend/internal/database/tursohttp"
)

func TestConsumeAuthFlowIncludesTheBrowserBinding(t *testing.T) {
	var requests [][]byte
	requestCount := 0
	client := &http.Client{Transport: radarManualRoundTripFunc(func(
		request *http.Request,
	) (*http.Response, error) {
		requestCount++
		body, _ := io.ReadAll(request.Body)
		requests = append(requests, body)
		responseBody := transactionExecuteResult("auth-tx", 0)
		if requestCount == 2 {
			responseBody = `{"baton":"auth-tx","results":[{"type":"ok","response":{"type":"execute","result":{"cols":[{"name":"id"},{"name":"invite_id"},{"name":"code_verifier"},{"name":"nonce"},{"name":"return_path"},{"name":"expires_at"}],"rows":[],"affected_row_count":0,"last_insert_rowid":null}}}]}`
		}
		if bytes.Contains(body, []byte(`"sql":"ROLLBACK"`)) {
			responseBody = closedTransactionResult()
		}
		return tursoTestResponse(responseBody), nil
	})}
	database, err := tursohttp.Open("libsql://example.turso.io", "secret", client)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	_, err = New(database).ConsumeAuthFlow(
		context.Background(),
		"state-hash",
		"browser-binding-hash",
		time.Now(),
	)
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("consume error = %v, want sql.ErrNoRows", err)
	}
	combined := bytes.Join(requests, []byte("\n"))
	for _, required := range [][]byte{
		[]byte("browser_binding_hash = ?"),
		[]byte(`"value":"browser-binding-hash"`),
	} {
		if !bytes.Contains(combined, required) {
			t.Fatalf("auth-flow query omitted %q: %s", required, combined)
		}
	}
}

func TestRevokeSessionRejectsANoOp(t *testing.T) {
	client := &http.Client{Transport: radarManualRoundTripFunc(func(
		*http.Request,
	) (*http.Response, error) {
		return tursoTestResponse(
			`{"results":[{"type":"ok","response":{"type":"execute","result":{"cols":[],"rows":[],"affected_row_count":0,"last_insert_rowid":null}}},{"type":"ok","response":{"type":"close"}}]}`,
		), nil
	})}
	database, err := tursohttp.Open("libsql://example.turso.io", "secret", client)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	err = New(database).RevokeSession(context.Background(), "missing", time.Now())
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("revocation error = %v, want sql.ErrNoRows", err)
	}
}

func TestCompleteRadarReplayDoesNotApplyFeedbackAgain(t *testing.T) {
	requestCount := 0
	var requests [][]byte
	client := &http.Client{Transport: radarManualRoundTripFunc(func(
		request *http.Request,
	) (*http.Response, error) {
		requestCount++
		body, _ := io.ReadAll(request.Body)
		requests = append(requests, body)
		responseBody := transactionExecuteResult("radar-tx", 0)
		switch requestCount {
		case 3:
			responseBody = `{"baton":"radar-tx","results":[{"type":"ok","response":{"type":"execute","result":{"cols":[{"name":"status"}],"rows":[[{"type":"text","value":"skipped"}]],"affected_row_count":0,"last_insert_rowid":null}}}]}`
		case 4:
			responseBody = closedTransactionResult()
		}
		return tursoTestResponse(responseBody), nil
	})}
	database, err := tursohttp.Open("libsql://example.turso.io", "secret", client)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	err = New(database).CompleteRadar(
		context.Background(),
		"user",
		"candidate",
		"skipped",
		time.Now(),
	)
	if err != nil {
		t.Fatal(err)
	}
	combined := string(bytes.Join(requests, []byte("\n")))
	if strings.Contains(combined, "feedback_weight") {
		t.Fatalf("replayed skip changed feedback: %s", combined)
	}
}

func transactionExecuteResult(baton string, changed int) string {
	return `{"baton":"` + baton +
		`","results":[{"type":"ok","response":{"type":"execute","result":{"cols":[],"rows":[],"affected_row_count":` +
		fmt.Sprint(changed) + `,"last_insert_rowid":null}}}]}`
}

func closedTransactionResult() string {
	return `{"results":[{"type":"ok","response":{"type":"execute","result":{"cols":[],"rows":[],"affected_row_count":0,"last_insert_rowid":null}}},{"type":"ok","response":{"type":"close"}}]}`
}

func tursoTestResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}
