package resource

import "strings"

type Name string

func (n Name) String() string { return string(n) }

func Build(parts ...string) Name {
	clean := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			clean = append(clean, strings.Trim(p, ":"))
		}
	}
	return Name(strings.Join(clean, ":"))
}

func Wildcard(domain, kind string) Name { return Build(domain, kind, "*") }
func AIHubSkill(id string) Name         { return Build("aihub", "skill", id) }
func AIHubSkillVersion(skillID, version string) Name {
	return Build("aihub", "skill-version", skillID, version)
}
func RuntimeSession(id string) Name  { return Build("runtime", "session", id) }
func SandboxInstance(id string) Name { return Build("sandbox", "instance", id) }
func ModelDeployment(id string) Name { return Build("model", "deployment", id) }
