package principal

// Principal is the identity contract shared with JiuwenSwarm. Provider-specific
// claims must be normalized before they reach the Gateway.
type Principal struct {
	Subject  string   `json:"sub"`
	UserID   string   `json:"user_id"`
	TenantID string   `json:"tenant_id"`
	Username string   `json:"username,omitempty"`
	Email    string   `json:"email,omitempty"`
	Groups   []string `json:"groups,omitempty"`
	Roles    []string `json:"roles,omitempty"`
}
