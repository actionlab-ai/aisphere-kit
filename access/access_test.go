package access

import (
	"context"
	"testing"

	"github.com/actionlab-ai/aisphere-kit/audit"
	"github.com/actionlab-ai/aisphere-kit/authz"
	"github.com/actionlab-ai/aisphere-kit/principal"
)

type fakeAuthorizer struct{ ok bool }

func (f fakeAuthorizer) Authorize(ctx context.Context, req authz.Request) (bool, error) {
	return f.ok, nil
}

type fakeRecorder struct{ events []audit.Event }

func (r *fakeRecorder) Record(ctx context.Context, ev audit.Event) error {
	r.events = append(r.events, ev)
	return nil
}

func TestGuardRequireAllows(t *testing.T) {
	ctx := principal.NewContext(context.Background(), &principal.Principal{SubjectType: principal.SubjectUser, SubjectID: "u1"})
	g := NewGuard(Options{Authz: fakeAuthorizer{ok: true}})
	p, err := g.Require(ctx, Check{Resource: "aihub:admin", Action: "admin.access"})
	if err != nil {
		t.Fatalf("Require error: %v", err)
	}
	if p.SubjectID != "u1" {
		t.Fatalf("unexpected principal: %#v", p)
	}
}

func TestGuardRequireDenies(t *testing.T) {
	ctx := principal.NewContext(context.Background(), &principal.Principal{SubjectType: principal.SubjectUser, SubjectID: "u1"})
	g := NewGuard(Options{Authz: fakeAuthorizer{ok: false}})
	_, err := g.Require(ctx, Check{Resource: "aihub:admin", Action: "admin.access"})
	if err == nil {
		t.Fatal("expected deny error")
	}
}

func TestGuardRecordUsesPrincipal(t *testing.T) {
	rec := &fakeRecorder{}
	ctx := principal.NewContext(context.Background(), &principal.Principal{SubjectType: principal.SubjectUser, SubjectID: "u1", OrgID: "org1"})
	g := NewGuard(Options{Audit: rec, Component: "unit"})
	if err := g.Record(ctx, Event{Action: "x", Resource: "r"}); err != nil {
		t.Fatalf("Record: %v", err)
	}
	if len(rec.events) != 1 {
		t.Fatalf("events=%d", len(rec.events))
	}
	if rec.events[0].Actor == nil || rec.events[0].Actor.SubjectID != "u1" {
		t.Fatalf("missing actor: %#v", rec.events[0])
	}
	if rec.events[0].Component != "unit" {
		t.Fatalf("component=%q", rec.events[0].Component)
	}
}
