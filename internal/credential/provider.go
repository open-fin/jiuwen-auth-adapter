package credential

import (
	"context"
	"time"

	"github.com/open-fin/jiuwen-auth-adapter/internal/principal"
)

// Request identifies the downstream service and security scope for which a
// credential is required. Cache keys must include every isolation dimension.
type Request struct {
	ServiceID string
	Audience  string
	Scopes    []string
	Principal principal.Principal
}

// Credential is a short-lived downstream credential. Secret contains sensitive
// material and must never be logged or serialized into ordinary configuration.
type Credential struct {
	Scheme    string
	Secret    string
	ExpiresAt time.Time
}

// Provider resolves credentials for a protected downstream service such as a
// customer MCP Gateway. Implementations may use static secrets, OAuth client
// credentials, token exchange, or an on-behalf-of flow.
type Provider interface {
	Resolve(ctx context.Context, request Request) (Credential, error)
}
