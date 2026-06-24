package resource

import "testing"

func TestBuild(t *testing.T) {
	if got := AIHubSkill("s1"); got != Name("aihub:skill:s1") {
		t.Fatalf("got %s", got)
	}
	if got := Wildcard("aihub", "skill"); got != Name("aihub:skill:*") {
		t.Fatalf("got %s", got)
	}
}
