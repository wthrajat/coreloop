// Package tursohttp implements the database/sql interfaces over Turso's
// documented SQL-over-HTTP pipeline API. It keeps the production binary pure
// Go and avoids depending on the deprecated libSQL Go client.
package tursohttp

import (
	"bytes"
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Connector struct {
	endpoint string
	token    string
	client   *http.Client
}

func NewConnector(databaseURL, token string, client *http.Client) (*Connector, error) {
	endpoint, err := pipelineURL(databaseURL)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(token) == "" {
		return nil, errors.New("Turso auth token is required")
	}
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	return &Connector{endpoint: endpoint, token: token, client: client}, nil
}

func Open(databaseURL, token string, client *http.Client) (*sql.DB, error) {
	connector, err := NewConnector(databaseURL, token, client)
	if err != nil {
		return nil, err
	}
	database := sql.OpenDB(connector)
	database.SetMaxOpenConns(8)
	database.SetMaxIdleConns(2)
	database.SetConnMaxIdleTime(30 * time.Second)
	return database, nil
}

func (connector *Connector) Connect(context.Context) (driver.Conn, error) {
	return &connection{connector: connector}, nil
}

func (*Connector) Driver() driver.Driver { return connectorDriver{} }

type connectorDriver struct{}

func (connectorDriver) Open(string) (driver.Conn, error) {
	return nil, errors.New("use tursohttp.NewConnector")
}

type connection struct {
	connector *Connector
	mu        sync.Mutex
	baton     string
	inTx      bool
	closed    bool
}

var (
	_ driver.Conn              = (*connection)(nil)
	_ driver.ExecerContext     = (*connection)(nil)
	_ driver.QueryerContext    = (*connection)(nil)
	_ driver.ConnBeginTx       = (*connection)(nil)
	_ driver.Pinger            = (*connection)(nil)
	_ driver.NamedValueChecker = (*connection)(nil)
)

func (connection *connection) Prepare(string) (driver.Stmt, error) { return nil, driver.ErrSkip }

func (connection *connection) Close() error {
	connection.mu.Lock()
	defer connection.mu.Unlock()
	if connection.closed {
		return nil
	}
	connection.closed = true
	if connection.baton == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := connection.request(ctx, []pipelineRequest{{Type: "close"}}, false)
	connection.baton = ""
	return err
}

func (connection *connection) Begin() (driver.Tx, error) {
	return connection.BeginTx(context.Background(), driver.TxOptions{})
}

func (connection *connection) BeginTx(ctx context.Context, options driver.TxOptions) (driver.Tx, error) {
	if options.ReadOnly {
		return nil, errors.New("read-only transactions are not supported")
	}
	connection.mu.Lock()
	defer connection.mu.Unlock()
	if connection.inTx {
		return nil, errors.New("transaction already active")
	}
	mode := "BEGIN"
	if options.Isolation == driver.IsolationLevel(sql.LevelSerializable) {
		mode = "BEGIN IMMEDIATE"
	}
	if _, err := connection.execute(ctx, mode, nil, true); err != nil {
		return nil, err
	}
	connection.inTx = true
	return &transaction{connection: connection}, nil
}

func (connection *connection) Ping(ctx context.Context) error {
	connection.mu.Lock()
	defer connection.mu.Unlock()
	_, err := connection.execute(ctx, "SELECT 1", nil, false)
	return err
}

func (*connection) CheckNamedValue(value *driver.NamedValue) error {
	switch value.Value.(type) {
	case nil, bool, []byte, string, int, int8, int16, int32, int64,
		uint, uint8, uint16, uint32, uint64, float32, float64, time.Time:
		return nil
	default:
		return driver.ErrSkip
	}
}

func (connection *connection) ExecContext(ctx context.Context, query string, values []driver.NamedValue) (driver.Result, error) {
	connection.mu.Lock()
	defer connection.mu.Unlock()
	result, err := connection.execute(ctx, query, values, connection.inTx)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (connection *connection) QueryContext(ctx context.Context, query string, values []driver.NamedValue) (driver.Rows, error) {
	connection.mu.Lock()
	defer connection.mu.Unlock()
	result, err := connection.execute(ctx, query, values, connection.inTx)
	if err != nil {
		return nil, err
	}
	return newRows(result), nil
}

func (connection *connection) execute(ctx context.Context, query string, values []driver.NamedValue, keepOpen bool) (*executeResult, error) {
	arguments := make([]argument, 0, len(values))
	for _, value := range values {
		argument, err := encodeArgument(value.Value)
		if err != nil {
			return nil, err
		}
		arguments = append(arguments, argument)
	}
	requests := []pipelineRequest{{Type: "execute", Statement: &statement{SQL: query, Arguments: arguments}}}
	if !keepOpen {
		requests = append(requests, pipelineRequest{Type: "close"})
	}
	response, err := connection.request(ctx, requests, keepOpen)
	if err != nil {
		return nil, err
	}
	if len(response.Results) == 0 {
		return nil, errors.New("Turso returned no pipeline results")
	}
	first := response.Results[0]
	if first.Type != "ok" || first.Response == nil || first.Response.Result == nil {
		return nil, first.asError()
	}
	return first.Response.Result, nil
}

func (connection *connection) request(ctx context.Context, requests []pipelineRequest, keepOpen bool) (*pipelineResponse, error) {
	payload := pipelinePayload{Baton: connection.baton, Requests: requests}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encode Turso pipeline: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, connection.connector.endpoint, bytes.NewReader(encoded))
	if err != nil {
		return nil, fmt.Errorf("create Turso request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+connection.connector.token)
	request.Header.Set("Content-Type", "application/json")
	response, err := connection.connector.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("call Turso: %w", err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if err != nil {
		return nil, fmt.Errorf("read Turso response: %w", err)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("Turso returned %s: %s", response.Status, safeMessage(body))
	}
	var decoded pipelineResponse
	if err := json.Unmarshal(body, &decoded); err != nil {
		return nil, fmt.Errorf("decode Turso response: %w", err)
	}
	if keepOpen {
		if decoded.Baton == "" {
			return nil, errors.New("Turso did not return a transaction baton")
		}
		connection.baton = decoded.Baton
	} else {
		connection.baton = ""
	}
	return &decoded, nil
}

type transaction struct {
	connection *connection
	done       bool
}

func (transaction *transaction) Commit() error   { return transaction.finish("COMMIT") }
func (transaction *transaction) Rollback() error { return transaction.finish("ROLLBACK") }

func (transaction *transaction) finish(command string) error {
	transaction.connection.mu.Lock()
	defer transaction.connection.mu.Unlock()
	if transaction.done || !transaction.connection.inTx {
		return sql.ErrTxDone
	}
	transaction.done = true
	requests := []pipelineRequest{
		{Type: "execute", Statement: &statement{SQL: command}},
		{Type: "close"},
	}
	_, err := transaction.connection.request(context.Background(), requests, false)
	transaction.connection.inTx = false
	transaction.connection.baton = ""
	return err
}

type pipelinePayload struct {
	Baton    string            `json:"baton,omitempty"`
	Requests []pipelineRequest `json:"requests"`
}

type pipelineRequest struct {
	Type      string     `json:"type"`
	Statement *statement `json:"stmt,omitempty"`
}

type statement struct {
	SQL       string     `json:"sql"`
	Arguments []argument `json:"args,omitempty"`
}

type argument struct {
	Type string `json:"type"`
	// Value is a wire union: text/integer use strings and float uses a number.
	Value  any     `json:"value,omitempty"`
	Base64 *string `json:"base64,omitempty"`
}

type pipelineResponse struct {
	Baton   string           `json:"baton"`
	Results []pipelineResult `json:"results"`
}

type pipelineResult struct {
	Type     string           `json:"type"`
	Response *responsePayload `json:"response,omitempty"`
	Error    *remoteError     `json:"error,omitempty"`
}

type responsePayload struct {
	Type   string         `json:"type"`
	Result *executeResult `json:"result,omitempty"`
}

type executeResult struct {
	Columns          []column     `json:"cols"`
	Rows             [][]argument `json:"rows"`
	AffectedRowCount int64        `json:"affected_row_count"`
	LastInsertRowID  *string      `json:"last_insert_rowid"`
}

type column struct {
	Name string `json:"name"`
	Type string `json:"decltype,omitempty"`
}

type remoteError struct {
	Message string `json:"message"`
	Code    string `json:"code"`
}

func (result pipelineResult) asError() error {
	if result.Error != nil {
		return fmt.Errorf("Turso query failed (%s): %s", result.Error.Code, result.Error.Message)
	}
	return fmt.Errorf("unexpected Turso pipeline result %q", result.Type)
}

type queryResult struct {
	affected int64
	lastID   int64
}

func (result *executeResult) LastInsertId() (int64, error) {
	if result.LastInsertRowID == nil {
		return 0, nil
	}
	return strconv.ParseInt(*result.LastInsertRowID, 10, 64)
}

func (result *executeResult) RowsAffected() (int64, error) { return result.AffectedRowCount, nil }

type resultRows struct {
	columns []string
	rows    [][]argument
	index   int
}

func newRows(result *executeResult) *resultRows {
	columns := make([]string, len(result.Columns))
	for index, column := range result.Columns {
		columns[index] = column.Name
	}
	return &resultRows{columns: columns, rows: result.Rows}
}

func (rows *resultRows) Columns() []string { return rows.columns }
func (*resultRows) Close() error           { return nil }

func (rows *resultRows) Next(destination []driver.Value) error {
	if rows.index >= len(rows.rows) {
		return io.EOF
	}
	for index, value := range rows.rows[rows.index] {
		decoded, err := decodeArgument(value)
		if err != nil {
			return err
		}
		destination[index] = decoded
	}
	rows.index++
	return nil
}

func encodeArgument(value any) (argument, error) {
	switch typed := value.(type) {
	case nil:
		return argument{Type: "null"}, nil
	case bool:
		if typed {
			return argumentWithValue("integer", "1"), nil
		}
		return argumentWithValue("integer", "0"), nil
	case []byte:
		encoded := base64.RawStdEncoding.EncodeToString(typed)
		return argument{Type: "blob", Base64: &encoded}, nil
	case string:
		return argumentWithValue("text", typed), nil
	case time.Time:
		return argumentWithValue("text", typed.UTC().Format(time.RFC3339Nano)), nil
	case int:
		return argumentWithValue("integer", strconv.FormatInt(int64(typed), 10)), nil
	case int8:
		return argumentWithValue("integer", strconv.FormatInt(int64(typed), 10)), nil
	case int16:
		return argumentWithValue("integer", strconv.FormatInt(int64(typed), 10)), nil
	case int32:
		return argumentWithValue("integer", strconv.FormatInt(int64(typed), 10)), nil
	case int64:
		return argumentWithValue("integer", strconv.FormatInt(typed, 10)), nil
	case uint:
		return argumentWithValue("integer", strconv.FormatUint(uint64(typed), 10)), nil
	case uint8:
		return argumentWithValue("integer", strconv.FormatUint(uint64(typed), 10)), nil
	case uint16:
		return argumentWithValue("integer", strconv.FormatUint(uint64(typed), 10)), nil
	case uint32:
		return argumentWithValue("integer", strconv.FormatUint(uint64(typed), 10)), nil
	case uint64:
		if typed > uint64(^uint64(0)>>1) {
			return argument{}, errors.New("integer exceeds SQLite range")
		}
		return argumentWithValue("integer", strconv.FormatUint(typed, 10)), nil
	case float32:
		return argumentWithValue("float", float64(typed)), nil
	case float64:
		return argumentWithValue("float", typed), nil
	default:
		return argument{}, fmt.Errorf("unsupported SQL argument type %T", value)
	}
}

func argumentWithValue(argumentType string, value any) argument {
	return argument{Type: argumentType, Value: value}
}

func decodeArgument(value argument) (driver.Value, error) {
	if value.Type == "null" {
		return nil, nil
	}

	switch value.Type {
	case "text":
		return stringArgumentValue(value)
	case "integer":
		encoded, err := stringArgumentValue(value)
		if err != nil {
			return nil, err
		}
		return strconv.ParseInt(encoded, 10, 64)
	case "float":
		encoded, ok := value.Value.(float64)
		if !ok {
			return nil, fmt.Errorf("Turso value type %q has an invalid value", value.Type)
		}
		return encoded, nil
	case "blob":
		if value.Base64 == nil {
			return nil, errors.New("Turso value type \"blob\" is missing its base64 value")
		}
		encoded := strings.TrimRight(*value.Base64, "=")
		return base64.RawStdEncoding.DecodeString(encoded)
	default:
		return nil, fmt.Errorf("unsupported Turso value type %q", value.Type)
	}
}

func stringArgumentValue(value argument) (string, error) {
	encoded, ok := value.Value.(string)
	if !ok {
		return "", fmt.Errorf("Turso value type %q has an invalid value", value.Type)
	}
	return encoded, nil
}

func pipelineURL(databaseURL string) (string, error) {
	value := strings.TrimSpace(databaseURL)
	value = strings.Replace(value, "libsql://", "https://", 1)
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" {
		return "", errors.New("invalid Turso database URL")
	}
	if parsed.Scheme != "https" && parsed.Scheme != "http" {
		return "", errors.New("Turso database URL must use libsql or https")
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/v2/pipeline"
	return parsed.String(), nil
}

func safeMessage(body []byte) string {
	message := strings.TrimSpace(string(body))
	if len(message) > 400 {
		message = message[:400]
	}
	return message
}
