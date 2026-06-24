package principal

import (
	"context"
	"testing"
)

func TestPrincipalAuthentication(t *testing.T) {
	if (Principal{}).IsAuthenticated() {
		t.Fatal("zero principal must not be authenticated")
	}
	if Anonymous().IsAuthenticated() {
		t.Fatal("anonymous principal must not be authenticated")
	}
	p := Principal{SubjectType: SubjectUser, SubjectID: "u1"}
	if !p.IsAuthenticated() || p.Subject() != "user:u1" {
		t.Fatalf("unexpected principal: %#v subject=%s", p, p.Subject())
	}
}

func TestContext(t *testing.T) {
	_, err := RequireFromContext(context.Background())
	if err != ErrMissingPrincipal {
		t.Fatalf("expected missing principal, got %v", err)
	}
	p := &Principal{SubjectType: SubjectUser, SubjectID: "u1"}
	ctx := NewContext(context.Background(), p)
	got, err := RequireFromContext(ctx)
	if err != nil || got != p {
		t.Fatalf("unexpected context principal: %v %v", got, err)
	}
}
