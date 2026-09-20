# Jiuwen Auth Adapter

Jiuwen Auth Adapter is the identity compatibility layer for JiuwenSwarm. It connects customer identity systems such as Keycloak and other OpenID Connect providers to one stable identity contract understood by JiuwenSwarm.

The adapter is intentionally separate from JiuwenSwarm's business Gateway. It handles login protocols and provider-specific claim mapping; the Gateway remains responsible for enforcing authentication and authorization on every protected HTTP, SSE, and WebSocket request.

> Status: architecture and runnable service scaffold. Authentication endpoints and cryptographic token issuance are not implemented yet.

## Why this service exists

JiuwenSwarm `dev-stable` can carry an access token from its Web UI, but its Gateway does not currently provide a complete, provider-neutral authentication boundary. In particular, passing a Bearer token is not the same as validating its signature, issuer, audience, expiry, user identity, tenant, and roles.

Customers may also enter JiuwenSwarm in different ways:

1. Their enterprise portal has already authenticated the user and hands JiuwenSwarm a credential or one-time ticket.
2. The user opens JiuwenSwarm directly and JiuwenSwarm initiates an OIDC Authorization Code + PKCE login.
3. A future customer uses another OIDC provider or SAML rather than Keycloak.

This adapter lets all entry paths converge on the same normalized identity and short-lived JiuwenSwarm token.

## Target architecture

```text
 Customer portal token/ticket       Direct JiuwenSwarm login
              |                               |
              +---------------+---------------+
                              v
                    Jiuwen Auth Adapter
                 OIDC/SAML and claim mapping
                              |
                    short-lived internal JWT
                     aud = "jiuwenswarm"
                              |
                              v
                    JiuwenSwarm Gateway
             HTTP + SSE + WebSocket enforcement
                              |
                              v
                    Runtime / AgentServer
```

The adapter should support generic OIDC discovery first. Keycloak, Auth0, Okta, and Microsoft Entra ID should normally be configuration variants of the same OIDC provider rather than separate implementations.

## Responsibilities

The adapter is responsible for:

- OIDC discovery, Authorization Code + PKCE login, callback, refresh, and logout;
- validating an external access token or exchanging a short-lived portal ticket;
- checking signature, issuer, audience, expiry, nonce, state, and PKCE binding;
- mapping provider-specific claims to a normalized `Principal`;
- issuing short-lived internal tokens with `aud=jiuwenswarm`;
- publishing a JWKS endpoint for JiuwenSwarm Gateway verification;
- optionally maintaining server-side sessions and secure HttpOnly cookies;
- producing audit events without logging tokens, authorization codes, or secrets.

The adapter is not responsible for:

- routing chat requests to agents;
- owning JiuwenSwarm sessions or conversation history;
- trusting identity headers supplied directly by a browser;
- replacing authorization checks inside JiuwenSwarm Gateway.

## Normalized principal

All providers map to one internal identity contract:

```json
{
  "sub": "customer-a:user-123",
  "user_id": "user-123",
  "tenant_id": "customer-a",
  "username": "zhangsan",
  "email": "zhangsan@example.com",
  "groups": ["finance"],
  "roles": ["chat-user"]
}
```

`sub`, `user_id`, and `tenant_id` are authoritative values derived from a verified upstream identity. They must never be copied from untrusted request headers.

## Planned HTTP API

```text
GET  /healthz                         Liveness
GET  /readyz                          Readiness
GET  /auth/login                      Start OIDC + PKCE login
GET  /auth/callback                   Process the OIDC callback
POST /auth/portal/exchange            Exchange a portal ticket/token
POST /auth/refresh                    Refresh or rotate a session
POST /auth/logout                     Revoke the local session
GET  /auth/me                         Return the normalized principal
GET  /.well-known/jwks.json           Publish internal signing keys
```

Compatibility routes matching the current JiuwenSwarm frontend may be exposed during migration:

```text
GET  /idp/v1/auth/me
POST /idp/v1/auth/refresh
POST /idp/v1/auth/logout
```

## Repository layout

```text
.
├── cmd/server/                    Process entry point
├── internal/config/               Environment and tenant configuration
├── internal/httpapi/              Login, callback, exchange and identity HTTP API
├── internal/principal/            Provider-neutral identity model
├── internal/provider/             Upstream identity provider interfaces
│   └── oidc/                      Generic OIDC implementation
├── internal/session/              Server-side session abstraction
├── internal/token/                Internal JWT issuer and JWKS publication
├── docs/
│   ├── architecture.md            Security boundaries and request flows
│   ├── development-guide.md       Incremental implementation and test gates
│   └── jiuwenswarm-integration.md Required JiuwenSwarm changes
├── Dockerfile
├── Makefile
└── go.mod
```

## Run the scaffold

```bash
make test
make run
curl http://localhost:8080/healthz
```

Only health endpoints are implemented in the initial scaffold. Security-sensitive authentication behavior will be added with tests before being enabled.

## Security principles

- Never put access tokens in URL query parameters.
- Prefer one-time portal tickets or a back-channel token exchange.
- Prefer Secure, HttpOnly, SameSite cookies for browser sessions.
- Accept access tokens for APIs, not ID tokens.
- Pin the expected issuer, audience, and allowed signing algorithms.
- Cache JWKS with bounded refresh and support key rotation.
- Fail closed when validation or identity mapping fails.
- Keep Gateway unreachable from untrusted networks except through an approved ingress path.
- Gateway must still validate the internal token; network isolation alone is not authentication.

See the following documents:

- [Architecture](docs/architecture.md)
- [Development and verification guide](docs/development-guide.md)
- [JiuwenSwarm integration](docs/jiuwenswarm-integration.md)

## License

Apache License 2.0. See [LICENSE](LICENSE).
