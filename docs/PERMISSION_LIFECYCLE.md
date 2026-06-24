# Permission lifecycle

AI Sphere uses Casdoor + Casbin as the authority for resource permissions.

## Boundary

- Business components own resource lifecycle: Skill, Agent, Runtime session, Sandbox instance, model deployment.
- Casdoor/Casbin owns permission policy: who can perform which action on which resource.
- aisphere-kit only provides a permission facade and Casdoor/Casbin implementation.
- Local component tables must not be used as the final permission authority.

## Sharing a resource

```go
err := rt.Permission.Share(ctx, permission.ShareRequest{
  Resource:    resource.AIHubSkill(skillID),
  SubjectType: permission.SubjectUser,
  SubjectID:   userID,
  Role:        permission.RoleViewer,
  GrantedBy:   actor.SubjectID,
})
```

This writes Casbin policies through Casdoor SDK `AddPolicy`.

## Deleting a resource

When `skill-a` is deleted, Hub must:

1. Verify `skill.delete` with `rt.Authz`.
2. Mark the Skill as deleted in Hub DB.
3. Trigger `rt.Permission.DeleteResourcePolicies(ctx, resource.AIHubSkill(skillID))`.
4. If policy cleanup fails, write an outbox job in Hub and retry.
5. Always check the resource still exists before returning data, even if Casbin still has stale policies.

## Recommended access order

```text
1. Load business resource and verify it is not deleted.
2. Check Casdoor/Casbin permission.
3. Execute business logic.
```

This prevents stale policies from allowing access to deleted resources.
