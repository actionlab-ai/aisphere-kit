# Hub 接入方式

Hub 后端第一版建议这样用：

```go
httpSrv := kratosx.NewHTTPServer(cfg, rt)
v1.RegisterSkillServiceHTTPServer(httpSrv, skillSvc)
```

业务层权限判断：

```go
p, _ := principal.MustFromContext(ctx)
err := authz.Require(ctx, rt.Casdoor, resource.AIHubSkill(skillID), action.SkillUpdate)
_ = p
```

上传包：

```go
url, err := rt.S3.PresignPut(ctx, "skills/"+skillID+"/package.zip", 15*time.Minute)
```

Redis 锁：

```go
lock, ok, err := cache.TryLock(ctx, rt.Redis, "hub:skill:publish:"+skillID, time.Minute)
if err != nil || !ok { return err }
defer lock.Unlock(ctx)
```
