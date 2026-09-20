# Development and Verification Guide

Authentication code must be implemented and verified incrementally. A successful login is not sufficient evidence of security: invalid tokens, forged identity headers, replay, cross-tenant access, and Gateway bypasses must all be rejected explicitly.

This guide defines the recommended implementation order, tests, and acceptance gates for Jiuwen Auth Adapter and its JiuwenSwarm integration.

## Verification layers

```text
Unit tests
    ↓
Adapter integration tests with fake OIDC/JWKS services
    ↓
Integration tests with a real Keycloak container
    ↓
JiuwenSwarm Gateway integration tests
    ↓
Browser end-to-end tests
    ↓
Security, failure, concurrency, and key-rotation tests
```

Normal CI must not depend on a customer's live Keycloak installation. Use deterministic in-process test servers for most tests and a pinned Keycloak container for protocol compatibility tests. Customer environments are reserved for final acceptance.

## General development rule

Each phase must include:

- implementation code;
- positive tests;
- negative and bypass tests;
- documented acceptance criteria;
- an independently reviewable commit;
- no unresolved security-critical test failures before starting the next phase.

## Phase 1: normalized identity contract

Define and test the provider-neutral principal before implementing any login protocol.

```json
{
  "sub": "customer-a:user-001",
  "user_id": "user-001",
  "tenant_id": "customer-a",
  "username": "alice",
  "groups": ["finance"],
  "roles": ["chat-user"]
}
```

Required tests:

- reject missing or empty `sub`, `user_id`, and `tenant_id`;
- allow empty groups and roles;
- map configured external claims correctly;
- reject ambiguous or conflicting claim mappings;
- ensure headers and request bodies cannot override the verified user ID;
- ensure equal upstream user IDs in different tenants produce distinct subjects;
- avoid exposing raw upstream tokens or unneeded claims in the principal.

Acceptance gate: all downstream code receives identity through the normalized principal rather than provider-specific claim maps.

## Phase 2: external JWT and JWKS validation

Implement generic OIDC access-token validation against a local fake issuer and JWKS endpoint. Do not require a real Keycloak instance for these tests.

| Case | Expected result |
|---|---|
| Correctly signed access token | Accepted |
| Invalid signature | `401` |
| Expired token | `401` |
| Future `nbf` outside clock skew | `401` |
| Unexpected issuer | `401` |
| Wrong audience | `401` |
| Unknown `kid` | Refresh JWKS once, then reject if still unknown |
| `alg=none` | Reject |
| Algorithm outside allowlist | Reject |
| Malformed or oversized JWT | Reject safely |
| JWKS timeout or invalid response | Fail closed |

Additional requirements:

- pin expected issuer and audience;
- allow only configured asymmetric algorithms;
- use bounded HTTP timeouts and response sizes;
- cache JWKS for a bounded period;
- avoid unbounded retries for unknown `kid` values;
- never log the raw token.

Acceptance gate: no code path can create a principal from an unverified token.

## Phase 3: internal token issuer and JWKS

Implement short-lived tokens intended specifically for JiuwenSwarm and publish public keys at:

```text
GET /.well-known/jwks.json
```

Required tests:

- sign with the private key and verify with the published JWKS;
- emit configured issuer and `aud=jiuwenswarm`;
- enforce a short, bounded lifetime;
- emit a unique `jti`;
- reject external-provider tokens where an internal token is required;
- never return or log private keys;
- verify new and old keys during a rotation overlap;
- reject an old key after the overlap closes;
- verify concurrent issuance does not duplicate token IDs.

Acceptance gate: JiuwenSwarm can verify internal tokens using only configured trust metadata and public JWKS.

## Phase 4: direct OIDC login with PKCE

Implement:

```text
GET /auth/login
GET /auth/callback
```

Begin with an in-process fake OIDC server, then repeat the contract tests against a Keycloak container.

Required tests:

- `/auth/login` creates unpredictable state, nonce, and PKCE verifier;
- callback validates state before code exchange;
- an authorization code can be used only once;
- mismatched PKCE verifier and nonce fail;
- missing or malformed callback parameters fail safely;
- redirect targets are allowlisted and cannot create an open redirect;
- a successful callback maps the expected principal;
- failed callbacks leave no partial authenticated session;
- secrets and authorization codes do not enter logs or errors.

Acceptance gate: login succeeds with both fake issuer and Keycloak, while replay and callback manipulation fail closed.

## Phase 5: enterprise portal exchange

Implement:

```text
POST /auth/portal/exchange
```

Prefer a short-lived, single-use portal ticket redeemed through a back channel. Never put an access token in a redirect URL.

Required tests:

- a valid ticket is accepted exactly once;
- expired tickets are rejected;
- concurrent redemption produces exactly one success;
- tickets are bound to customer, environment, redirect target, and intended service;
- a portal token with only `aud=customer-portal` is rejected;
- token exchange produces `aud=jiuwenswarm`;
- redemption network failure does not authenticate an invalid ticket;
- tickets and tokens do not enter logs.

Single-use behavior must use an atomic Redis operation or database transaction. An in-process map is insufficient for multiple replicas.

Acceptance gate: direct and portal login produce the same principal and internal session/token format.

## Phase 6: browser session, refresh, and logout

Prefer an opaque server-side session identifier:

```http
Set-Cookie: jiuwen_session=...; Path=/; HttpOnly; Secure; SameSite=Lax
```

Required tests:

- JavaScript cannot read the HttpOnly cookie;
- production cookies require `Secure`;
- cookie scope excludes unrelated applications;
- modified or unknown cookies fail authentication;
- logout invalidates local session even if upstream logout fails;
- rotated refresh tokens cannot be replayed;
- refresh tokens are not returned to browser JavaScript;
- state-changing cookie-authenticated endpoints enforce CSRF protection;
- multiple replicas observe the same sessions;
- session expiry is enforced server-side.

Acceptance gate: browser secrets are not stored in localStorage, and logout/expiry consistently affects HTTP and WebSocket access.

## Phase 7: JiuwenSwarm HTTP and SSE integration

Add Gateway validation before protected requests reach business dispatch. Exercise at least:

```text
GET  /api/v1/sessions
POST /api/v1/chat/completions
GET  /api/v1/... using SSE
File and share routes
```

Required tests:

- missing or invalid authentication returns `401`;
- insufficient permission returns `403`;
- valid identity can execute an authorized chat request;
- authentication middleware does not buffer or break SSE;
- health and readiness remain usable by Kubernetes probes;
- client-supplied identity headers are removed or overwritten;
- identity fields in a body cannot override the principal;
- tenant A cannot access tenant B sessions or files;
- errors contain no token or sensitive claims.

This request must remain Alice regardless of forged headers:

```http
Authorization: Bearer <alice-token>
X-User-Id: admin
X-Group-Id: another-tenant
```

Acceptance gate: all protected HTTP and SSE operations derive authoritative identity from the verified principal.

## Phase 8: JiuwenSwarm WebSocket integration

Protect both `/ws` and `/ws/git`.

Required tests:

- handshake without session/token is rejected;
- invalid or expired token is rejected;
- valid session completes the handshake;
- forged `user_id` query parameter has no effect;
- forged identity headers have no effect;
- HTTP and WebSocket derive the same principal;
- token-expiry policy for existing connections is enforced;
- logout prevents new connections and, if policy requires, terminates old ones;
- WebSocket URLs and logs contain no tokens.

Acceptance gate: WebSocket routing identity is attached from authentication state, never from browser-controlled query parameters.

## Phase 9: tenant and role authorization

Authentication proves identity; authorization decides allowed actions. Create at least:

| User | Tenant | Roles |
|---|---|---|
| Alice | tenant-a | `chat-user` |
| Bob | tenant-b | `chat-user` |
| Admin | tenant-a | `admin` |

Required tests:

- Alice accesses only allowed tenant-a resources;
- Bob cannot access tenant-a resources even with known IDs;
- tenant-a Admin is not automatically tenant-b Admin;
- selecting an Agent or group does not grant entitlement;
- role/group mapping is deterministic and documented;
- `403` does not reveal whether a foreign resource exists.

Acceptance gate: horizontal and vertical privilege-escalation tests pass over HTTP, SSE, and WebSocket.

## Phase 10: real Keycloak compatibility

Run a pinned Keycloak container with an imported test realm:

```text
Realm:  test
Client: jiuwenswarm
Users:
  alice -> tenant-a / chat-user
  bob   -> tenant-b / chat-user
  admin -> tenant-a / admin
```

Verify direct login and portal exchange. Cover discovery, audience configuration, realm/client role claims, groups, refresh rotation, logout, JWKS rotation, and issuer behavior behind a reverse proxy.

Acceptance gate: the same contract suite passes against the fake issuer and pinned Keycloak deployment.

## Failure, concurrency, and attack testing

Test at least:

- unavailable OIDC provider;
- JWKS timeout and malformed response;
- unavailable Redis/session database;
- adapter and Gateway restarts;
- signing-key rotation;
- bounded clock skew;
- simultaneous refresh requests;
- authorization-code and portal-ticket replay;
- oversized tokens and malicious `kid` values;
- invalid-`kid` floods intended to force JWKS refreshes;
- log scanning for JWTs, cookies, client secrets, codes, and tickets.

Authentication failures must fail closed. Availability failures must never become anonymous or privileged access.

## CI pipeline

Run on every pull request:

```bash
go test -race ./...
go vet ./...
go test ./... -coverprofile=coverage.out
```

Pull requests run unit, fake OIDC/JWKS, race, mapping, and secret-leak tests. Before merge, run pinned Keycloak, Gateway integration, protocol identity-consistency, and tenant-isolation tests. Before release, add browser E2E, rotation, failure injection, load/concurrency, dependency/container scans, and a final log-leak scan.

## Recommended implementation order

```text
1. Principal validation and claim mapping
2. External JWT/JWKS validation
3. Internal token issuance and JWKS publication
