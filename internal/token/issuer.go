package token

import (
	"context"
	"time"

	"github.com/open-fin/jiuwen-auth-adapter/internal/principal"
)

// Issuer creates short-lived tokens whose audience is JiuwenSwarm. Gateway
// validates these tokens and never trusts browser-supplied identity headers.
type Issuer interface {
	Issue(ctx context.Context, identity principal.Principal, ttl time.Duration) (string, error)
}
