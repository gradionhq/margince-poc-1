package workflow

import (
	"context"
	"testing"
)

type h struct{}

func (h) Match(string) bool                       { return true }
func (h) Plan(context.Context, string, any) error { return nil }

func TestHandler(t *testing.T) {
	var _ Handler = h{}
}
