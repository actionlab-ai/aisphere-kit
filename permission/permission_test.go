package permission

import (
	"testing"

	"github.com/actionlab-ai/aisphere-kit/resource"
)

func TestSubject(t *testing.T) {
	if got := Subject(SubjectUser, "alice"); got != "user:alice" {
		t.Fatalf("unexpected subject: %s", got)
	}
}

func TestNormalizeShare(t *testing.T) {
	g, err := NormalizeShare(ShareRequest{Resource: resource.AIHubSkill("s1"), SubjectType: SubjectUser, SubjectID: "alice", Role: RoleViewer})
	if err != nil {
		t.Fatal(err)
	}
	if g.Subject != "user:alice" || g.Resource.String() != "aihub:skill:s1" {
		t.Fatalf("bad grant: %+v", g)
	}
}

func TestRoleActions(t *testing.T) {
	actions, err := RoleActions(RoleOwner, nil)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, a := range actions {
		if a == "skill.delete" {
			found = true
		}
	}
	if !found {
		t.Fatalf("owner role should include skill.delete: %#v", actions)
	}
}
