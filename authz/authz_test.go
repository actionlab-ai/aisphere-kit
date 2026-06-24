package authz

import (
	"context"
	"testing"

	"github.com/actionlab-ai/aisphere-kit/principal"
)

type fakeAuthorizer struct {
	ok  bool
	err error
}

func (f fakeAuthorizer) Authorize(ctx context.Context, req Request) (bool, error) { return f.ok, f.err }

func TestRequireDenyEmpty(t *testing.T) {
	ctx := principal.NewContext(context.Background(), &principal.Principal{SubjectType: principal.SubjectUser, SubjectID: "u1"})
	if err := Require(ctx, fakeAuthorizer{ok: true}, "", "read"); err == nil {
		t.Fatal("expected empty resource denied")
	}
	if err := Require(ctx, fakeAuthorizer{ok: true}, "r1", ""); err == nil {
		t.Fatal("expected empty action denied")
	}
}

func TestRequireAllowed(t *testing.T) {
	ctx := principal.NewContext(context.Background(), &principal.Principal{SubjectType: principal.SubjectUser, SubjectID: "u1"})
	if err := Require(ctx, fakeAuthorizer{ok: true}, "r1", "read"); err != nil {
		t.Fatalf("expected allow, got %v", err)
	}
}
