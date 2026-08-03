package apperror

import (
	"errors"
	"testing"
)

func TestWrapPreservesCause(t *testing.T) {
	cause := errors.New("database unavailable")
	err := Wrap(CodeInternal, "health check failed", 500, cause)

	if !errors.Is(err, cause) {
		t.Fatal("wrapped application error must preserve its cause")
	}

	if err.Code != CodeInternal {
		t.Fatalf("expected code %q, got %q", CodeInternal, err.Code)
	}
}
