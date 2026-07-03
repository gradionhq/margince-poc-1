package errs

import (
	"errors"
	"testing"
)

func TestSentinelsDistinct(t *testing.T) {
	if errors.Is(ErrNotFound, ErrConflict) {
		t.Fatal("sentinels must be distinct")
	}
}
