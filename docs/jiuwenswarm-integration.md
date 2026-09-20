# JiuwenSwarm integration

This document describes the changes required in JiuwenSwarm `dev-stable`. The adapter alone is not sufficient because the current Gateway can be reached with browser-controlled identity headers.

## Current gap

The current Web frontend stores `openjiuwen_access_token` and adds it as a Bearer token to HTTP requests. It also mirrors the access token to a browser cookie for WebSocket handshakes. However, the Gateway HTTP and WebSocket paths do not provide a complete JWT validation boundary before resolving `user_id`, `group_id`, and `bot_id`.

In enterprise mode, `GATEWAY_WEB_HTTP_TRUST_CLIENT_HEADERS=true` allows client-supplied `X-User-Id`, `X-Group-Id`, and `X-Bot-Id` values to influence routing. Those values must not be treated as authenticated identity.

## Required changes

### 1. Add a shared authentication principal

Add a provider-neutral principal model under `jiuwenswarm/common/auth/`. It should contain at least:

- issuer and subject;
- user ID and tenant ID;
- groups and roles;
- token ID and expiry for audit context.

Request handling should carry this principal explicitly instead of reconstructing identity from arbitrary headers.

### 2. Add internal JWT verification

Gateway should validate adapter-issued tokens using the adapter JWKS. Validation must include:

- signature and allowed asymmetric algorithm;
- exact issuer;
- audience containing `jiuwenswarm`;
- expiry, not-before and bounded clock skew;
- required subject, user and tenant claims;
- bounded JWKS caching and safe key rotation behavior.

Configuration should be provider-neutral, for example:

```text
JIUWEN_AUTH_ENABLED=true
JIUWEN_AUTH_ISSUER=https://auth.example.com
JIUWEN_AUTH_AUDIENCE=jiuwenswarm
JIUWEN_AUTH_JWKS_URL=https://auth.example.com/.well-known/jwks.json
JIUWEN_AUTH_ALLOWED_ALGORITHMS=RS256
```

### 3. Protect HTTP and SSE

Integrate authentication into `jiuwenswarm/gateway/channel_manager/web/web_http_app.py` before protected routes dispatch business operations.

The health/readiness endpoints may remain anonymous. Explicitly decide whether API documentation is public. Chat, sessions, history, configuration, skills, file and share routes must have an intentional policy rather than accidental anonymous access.

After verification:

- derive `user_id` and tenant scope from the principal;
- remove or overwrite untrusted identity headers;
- pass the principal through the dispatch context;
- return `401` for missing/invalid authentication and `403` for insufficient permission.

### 4. Protect WebSocket handshakes

Integrate the same verifier into `jiuwenswarm/gateway/channel_manager/web/web_ws_transport.py::_process_request` for both `/ws` and `/ws/git`.

Browser WebSocket clients cannot reliably set a custom Authorization header. Use a Secure, HttpOnly session cookie or an ingress-authenticated mechanism. Do not put tokens in the WebSocket query string because URLs are commonly logged.

Attach the verified principal to the connection and build the routing identity from it. Do not resolve the authoritative user from `user_id` query parameters or `X-User-Id` supplied by the browser.

### 5. Change HTTP dispatch identity resolution

Update `jiuwenswarm/gateway/channel_manager/web/web_http_dispatch.py` so `_trust_client_tenant_headers` is not an authentication decision.

When auth is enabled:

```text
principal.user_id   -> routing user_id
principal.tenant_id -> tenant isolation
mapped group policy -> group_id, if applicable
```

User-selected Agent or group context must be checked against entitlements; selection is not proof of authorization.

### 6. Keep frontend compatibility during migration

The adapter can expose the current frontend compatibility contract:

```text
GET  /idp/v1/auth/me
POST /idp/v1/auth/refresh
POST /idp/v1/auth/logout
```

Configure `USER_WEB_IDP_TARGET` to point to the adapter and keep `LOGIN_AUTH_SIMULATE=false`. The existing Manager context API can initially remain behind `USER_WEB_MANAGER_TARGET`, but it must authorize results using the verified principal.

Longer term, replace localStorage token persistence with an HttpOnly session cookie managed by the adapter/BFF. Until then, treat the localStorage flow as transitional because an XSS issue can read both access and refresh tokens.

### 7. Lock down deployment paths

- Expose Gateway only behind the approved ingress/User Web path.
- Apply Kubernetes NetworkPolicy so users cannot bypass the adapter/ingress.
- Strip incoming identity headers at the edge.
- Add trusted identity headers only after successful authentication, if headers remain part of the internal transport.
- Never log Authorization, cookies, authorization codes, portal tickets or raw JWT claims.

## Suggested implementation order

1. Principal model and JWT/JWKS verifier.
2. HTTP/SSE middleware with negative security tests.
3. WebSocket handshake authentication.
4. Removal of browser-controlled routing identity.
5. Adapter OIDC login and compatibility endpoints.
6. Portal ticket/token exchange.
7. Role and tenant authorization policies.
8. Key rotation, logout/revocation and end-to-end tests.

## Acceptance criteria

- Missing, expired, wrongly signed, wrong-issuer and wrong-audience tokens receive `401`.
- A valid token cannot access another tenant by changing headers, query parameters or request bodies.
- HTTP, SSE, `/ws`, and `/ws/git` derive the same identity.
- Key rotation succeeds without accepting unknown keys indefinitely.
- Health endpoints remain usable by Kubernetes probes.
- Authentication secrets and raw tokens never appear in normal logs or error responses.
