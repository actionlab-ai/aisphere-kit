package casdoor

type Config struct {
	Endpoint     string `json:"endpoint" yaml:"endpoint"`
	ClientID     string `json:"client_id" yaml:"client_id"`
	ClientSecret string `json:"client_secret" yaml:"client_secret"`
	Certificate  string `json:"certificate" yaml:"certificate"`
	Organization string `json:"organization" yaml:"organization"`
	Application  string `json:"application" yaml:"application"`
	// Authz selector fields map to Casdoor /api/enforce query selectors.
	// Casdoor expects exactly one selector for each enforce call.
	// Configure PermissionID first in most cases; ModelID/ResourceID/EnforcerID/Owner are fallback selectors.
	PermissionID string `json:"permission_id" yaml:"permission_id"`
	ModelID      string `json:"model_id" yaml:"model_id"`
	ResourceID   string `json:"resource_id" yaml:"resource_id"`
	EnforcerID   string `json:"enforcer_id" yaml:"enforcer_id"`
	Owner        string `json:"owner" yaml:"owner"`
	// PolicyEnforcer and PolicyAdapterID are used by Casdoor/Casbin policy-management APIs.
	// Configure PolicyEnforcer to the Casdoor enforcer name that owns the Casbin adapter.
	PolicyEnforcer  string `json:"policy_enforcer" yaml:"policy_enforcer"`
	PolicyAdapterID string `json:"policy_adapter_id" yaml:"policy_adapter_id"`
	HTTPTimeout     string `json:"http_timeout" yaml:"http_timeout"`
	AllowAnonymous  bool   `json:"allow_anonymous" yaml:"allow_anonymous"`
	AuditAsync      bool   `json:"audit_async" yaml:"audit_async"`
	AuditQueue      int    `json:"audit_queue" yaml:"audit_queue"`
	AuditTimeout    string `json:"audit_timeout" yaml:"audit_timeout"`
	RetryAttempts   int    `json:"retry_attempts" yaml:"retry_attempts"`
	RetryBackoff    string `json:"retry_backoff" yaml:"retry_backoff"`
}
