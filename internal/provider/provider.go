package provider

import (
	"context"

	"github.com/open-fin/jiuwen-auth-adapter/internal/principal"
)

// IdentityProvider hides Keycloak, Entra ID, Okta, Auth0 and other upstream
// identity systems behind one contract.
type IdentityProvider interface {
	AuthorizationURL(state, nonce, codeChallenge string) (string, error)
	ExchangeCode(ctx context.Context, code, codeVerifier string) (principal.Principal, error)
	ValidateAccessToken(ctx context.Context, rawToken string) (principal.Principal, error)
}
