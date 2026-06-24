// Package permission defines AI Sphere permission-management abstractions.
//
// The authority for permission decisions is expected to be Casdoor/Casbin via an
// implementation such as casdoor.Adapter. This package intentionally does not
// store permissions in local tables.
package permission

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/actionlab-ai/aisphere-kit/principal"
	"github.com/actionlab-ai/aisphere-kit/resource"
)

var (
	ErrManagerNil       = errors.New("permission manager is nil")
	ErrEmptySubject     = errors.New("permission subject is required")
	ErrEmptyResource    = errors.New("permission resource is required")
	ErrEmptyAction      = errors.New("permission action is required")
	ErrEmptyRole        = errors.New("permission role is required")
	ErrUnsupportedRole  = errors.New("unsupported permission role")
	ErrNotSupported     = errors.New("permission operation is not supported by current backend")
	ErrPermissionDenied = errors.New("permission denied")
)

const (
	SubjectUser    = "user"
	SubjectGroup   = "group"
	SubjectOrg     = "org"
	SubjectProject = "project"
	SubjectService = "service"
	SubjectPublic  = "public"

	RoleViewer = "viewer"
	RoleEditor = "editor"
	RoleOwner  = "owner"
)

// Grant represents one logical permission grant. If Role is set, Actions may be
// empty and the manager can expand the role to one or more concrete actions.
type Grant struct {
	Subject     string         `json:"subject"`
	SubjectType string         `json:"subject_type,omitempty"`
	SubjectID   string         `json:"subject_id,omitempty"`
	Resource    resource.Name  `json:"resource"`
	Action      string         `json:"action,omitempty"`
	Role        string         `json:"role,omitempty"`
	Actions     []string       `json:"actions,omitempty"`
	Domain      string         `json:"domain,omitempty"`
	GrantedBy   string         `json:"granted_by,omitempty"`
	ExpiresAt   *time.Time     `json:"expires_at,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

type ShareRequest struct {
	Resource    resource.Name  `json:"resource"`
	SubjectType string         `json:"subject_type"`
	SubjectID   string         `json:"subject_id"`
	Role        string         `json:"role"`
	Actions     []string       `json:"actions,omitempty"`
	Domain      string         `json:"domain,omitempty"`
	GrantedBy   string         `json:"granted_by,omitempty"`
	ExpiresAt   *time.Time     `json:"expires_at,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

type CheckRequest struct {
	Subject   string               `json:"subject"`
	Principal *principal.Principal `json:"-"`
	Resource  resource.Name        `json:"resource"`
	Action    string               `json:"action"`
	Domain    string               `json:"domain,omitempty"`
	Extra     []any                `json:"extra,omitempty"`
}

type ListFilter struct {
	Subject  string        `json:"subject,omitempty"`
	Resource resource.Name `json:"resource,omitempty"`
	Action   string        `json:"action,omitempty"`
	Domain   string        `json:"domain,omitempty"`
}

type DeleteResourceRequest struct {
	Resource resource.Name `json:"resource"`
	Reason   string        `json:"reason,omitempty"`
	Actor    string        `json:"actor,omitempty"`
}

// Manager is the platform-level permission facade. Implementations should map
// these calls to Casdoor/Casbin policy APIs, not local business tables.
type Manager interface {
	Grant(ctx context.Context, grant Grant) error
	GrantRole(ctx context.Context, grant Grant) error
	Share(ctx context.Context, req ShareRequest) error
	Revoke(ctx context.Context, grant Grant) error
	Check(ctx context.Context, req CheckRequest) (bool, error)
	List(ctx context.Context, filter ListFilter) ([]Grant, error)
	DeleteResourcePolicies(ctx context.Context, resource resource.Name) error
	DeleteResourcePoliciesEx(ctx context.Context, req DeleteResourceRequest) error
}

func Subject(subjectType, id string) string {
	subjectType = strings.TrimSpace(subjectType)
	id = strings.TrimSpace(id)
	if subjectType == "" {
		return id
	}
	if id == "" {
		return subjectType
	}
	return subjectType + ":" + id
}

func SubjectFromPrincipal(p *principal.Principal) string {
	if p == nil {
		return ""
	}
	return p.Subject()
}

func NormalizeGrant(g Grant) (Grant, error) {
	if g.Subject == "" && g.SubjectType != "" {
		g.Subject = Subject(g.SubjectType, g.SubjectID)
	}
	g.Subject = strings.TrimSpace(g.Subject)
	g.Action = strings.TrimSpace(g.Action)
	g.Role = strings.TrimSpace(g.Role)
	if g.Subject == "" {
		return g, ErrEmptySubject
	}
	if g.Resource.String() == "" {
		return g, ErrEmptyResource
	}
	if g.Action == "" && g.Role == "" && len(g.Actions) == 0 {
		return g, ErrEmptyAction
	}
	if g.ExpiresAt != nil {
		// Casbin p, sub, obj, act policies are intentionally timeless in v1.
		// Expiring share links should be modeled in the business component and
		// materialized/revoked through this manager when accepted/expired.
		return g, fmt.Errorf("%w: expires_at on Casbin policy grant", ErrNotSupported)
	}
	return g, nil
}

func NormalizeShare(req ShareRequest) (Grant, error) {
	if req.SubjectType == "" {
		return Grant{}, ErrEmptySubject
	}
	if req.SubjectType != SubjectPublic && req.SubjectID == "" {
		return Grant{}, ErrEmptySubject
	}
	if req.Resource.String() == "" {
		return Grant{}, ErrEmptyResource
	}
	if req.Role == "" && len(req.Actions) == 0 {
		return Grant{}, ErrEmptyRole
	}
	return NormalizeGrant(Grant{
		Subject:     Subject(req.SubjectType, req.SubjectID),
		SubjectType: req.SubjectType,
		SubjectID:   req.SubjectID,
		Resource:    req.Resource,
		Role:        req.Role,
		Actions:     req.Actions,
		Domain:      req.Domain,
		GrantedBy:   req.GrantedBy,
		ExpiresAt:   req.ExpiresAt,
		Metadata:    req.Metadata,
	})
}

func RoleActions(role string, custom []string) ([]string, error) {
	if len(custom) > 0 {
		return cleanActions(custom), nil
	}
	switch role {
	case RoleViewer:
		return []string{"skill.read", "skill.download"}, nil
	case RoleEditor:
		return []string{"skill.read", "skill.download", "skill.update", "skill.upload", "skill.publish"}, nil
	case RoleOwner:
		return []string{"skill.read", "skill.download", "skill.update", "skill.upload", "skill.publish", "skill.share", "skill.delete"}, nil
	case "":
		return nil, ErrEmptyRole
	default:
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedRole, role)
	}
}

func cleanActions(in []string) []string {
	out := make([]string, 0, len(in))
	seen := map[string]struct{}{}
	for _, a := range in {
		a = strings.TrimSpace(a)
		if a == "" {
			continue
		}
		if _, ok := seen[a]; ok {
			continue
		}
		seen[a] = struct{}{}
		out = append(out, a)
	}
	return out
}

func Require(ctx context.Context, mgr Manager, req CheckRequest) error {
	if mgr == nil {
		return ErrManagerNil
	}
	if req.Subject == "" && req.Principal != nil {
		req.Subject = SubjectFromPrincipal(req.Principal)
	}
	if req.Subject == "" {
		return ErrEmptySubject
	}
	if req.Resource.String() == "" {
		return ErrEmptyResource
	}
	if req.Action == "" {
		return ErrEmptyAction
	}
	ok, err := mgr.Check(ctx, req)
	if err != nil {
		return err
	}
	if !ok {
		return ErrPermissionDenied
	}
	return nil
}
