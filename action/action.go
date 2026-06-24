package action

const (
	Create = "create"
	Read   = "read"
	Update = "update"
	Delete = "delete"
	List   = "list"

	SkillCreate   = "skill.create"
	SkillRead     = "skill.read"
	SkillUpdate   = "skill.update"
	SkillDelete   = "skill.delete"
	SkillPublish  = "skill.publish"
	SkillShare    = "skill.share"
	SkillUpload   = "skill.upload"
	SkillDownload = "skill.download"

	AgentRun              = "agent.run"
	RuntimeSessionCreate  = "runtime.session.create"
	RuntimeSessionDelete  = "runtime.session.delete"
	SandboxInstanceCreate = "sandbox.instance.create"
	SandboxInstanceDelete = "sandbox.instance.delete"
	ModelInvoke           = "model.invoke"
)
