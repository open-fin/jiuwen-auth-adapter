# Architecture

## Trust boundaries

The browser, portal-provided headers, URL parameters, and external tokens are untrusted until verified. The adapter verifies the upstream identity and creates a normalized principal. JiuwenSwarm Gateway independently validates the resulting internal token before using that principal.

```text
Untrusted                       Authentication boundary              Resource boundary

Browser / customer portal  ->  Jiuwen Auth Adapter  -> internal JWT -> JiuwenSwarm Gateway
                                  external issuer                     internal issuer,
                                  and claim checks                    audience and ACL checks
```

## Direct login flow

1. The browser requests `/auth/login`.
2. The adapter creates state, nonce, and a PKCE verifier and redirects to the configured OIDC provider.
3. The provider returns an authorization code to `/auth/callback`.
4. The adapter validates state and exchanges the code using the PKCE verifier.
5. The adapter validates the returned identity and maps it to a normalized principal.
6. The adapter creates a server-side session or short-lived internal token.
7. Gateway validates that token on every protected request.

## Enterprise portal flow

1. The portal authenticates the user with its existing identity system.
2. The portal redirects with a short-lived, single-use ticket. A raw access token must not be placed in the URL.
3. The adapter redeems the ticket through a back channel or validates an access token submitted by POST.
4. The adapter maps the verified identity and creates the same session/token used by direct login.

If the portal token has `aud=customer-portal`, it must not be accepted directly by JiuwenSwarm. The portal or adapter must obtain/exchange a token intended for `aud=jiuwenswarm`.

## Internal token

The internal token should be short-lived and asymmetrically signed. Suggested claims are:

```json
{
  "iss": "https://auth.example.com",
  "aud": "jiuwenswarm",
  "sub": "customer-a:user-123",
  "user_id": "user-123",
  "tenant_id": "customer-a",
  "groups": ["finance"],
  "roles": ["chat-user"],
  "iat": 1790000000,
  "exp": 1790000300,
  "jti": "unique-token-id"
}
```

Signing keys must support rotation. Private keys remain in the adapter; Gateway consumes only the public JWKS.

## Multi-provider design

Provider-specific code terminates at the `IdentityProvider` interface. Claim mapping is configuration-driven per tenant wherever possible. Generic OIDC is the first provider; Keycloak is an OIDC configuration rather than a hard-coded dependency.

SAML can be added later by terminating the SAML assertion at the adapter and issuing the same internal token. JiuwenSwarm therefore never needs to understand SAML.

## One repository, two security directions

This repository owns two modules with different trust directions:

```text
Inbound authentication
User / customer portal -> Auth Server -> Jiuwen principal and internal token

Outbound credentials
AgentServer -> Credential Broker -> customer OAuth server -> customer MCP Gateway
```

The internal Jiuwen token normally has `aud=jiuwenswarm` and must not be forwarded to a customer MCP Gateway. The credential broker obtains a separate downstream token whose audience and scopes match that MCP Gateway.

Static shared MCP credentials do not require the broker. Dynamic client-credentials, tenant-level, user-level, token-exchange, refresh, or revocation requirements do.
