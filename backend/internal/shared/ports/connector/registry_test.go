package connector

import (
	"context"
	"errors"
	"testing"
)

type healthyStub struct{ stubConnector }

func (healthyStub) Descriptor() Descriptor {
	return Descriptor{Name: "healthy", Scopes: []string{"read"}, Tier: "green"}
}
func (healthyStub) HealthCheck(context.Context) error { return nil }

func TestScopeSubset(t *testing.T) {
	if !ScopeSubset([]string{"read"}, []string{"read", "write"}) {
		t.Error("read ⊆ {read,write} should hold")
	}
	if ScopeSubset([]string{"admin"}, []string{"read"}) {
		t.Error("admin ⊄ {read} should be false")
	}
}

func TestRegisterScopedRejectsOverScope(t *testing.T) {
	over := overScoped{}
	err := RegisterScoped(over, []string{"read"})
	if !errors.Is(err, ErrScopeExceeded) {
		t.Fatalf("expected ErrScopeExceeded for over-scoped connector, got %v", err)
	}
	if _, ok := Get("over"); ok {
		t.Fatal("over-scoped connector must NOT be in the registry")
	}
}

func TestRegisterScopedAllowsSubset(t *testing.T) {
	if err := RegisterScoped(healthyStub{}, []string{"read", "write"}); err != nil {
		t.Fatalf("subset registration rejected: %v", err)
	}
	if _, ok := Get("healthy"); !ok {
		t.Fatal("in-scope connector must be registered")
	}
}

func TestHealthOfUnhealthyDoesNotPanic(t *testing.T) {
	if err := HealthOf(context.Background(), unhealthyStub{}); err == nil {
		t.Fatal("expected HealthOf to surface the unhealthy error")
	}
	if err := HealthOf(context.Background(), stubConnector{}); err != nil {
		t.Fatalf("connector without HealthChecker should be healthy, got %v", err)
	}
}

type overScoped struct{ stubConnector }

func (overScoped) Descriptor() Descriptor {
	return Descriptor{Name: "over", Scopes: []string{"read", "admin"}, Tier: "red"}
}

type unhealthyStub struct{ stubConnector }

func (unhealthyStub) Descriptor() Descriptor {
	return Descriptor{Name: "unhealthy", Scopes: []string{"read"}, Tier: "green"}
}

func (unhealthyStub) HealthCheck(context.Context) error {
	return errors.New("connector down")
}
