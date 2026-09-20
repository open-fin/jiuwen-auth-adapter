# 客户 MCP Gateway 对接决策指南

本文档回答一个具体问题：

> 如果客户的 MCP Gateway 是某种协议、认证和凭据模式，JiuwenSwarm 是只改配置、启用本仓库的 Credential Broker，还是必须修改代码？

## 两个不同的认证方向

~~~text
入站用户认证：
用户 -> Keycloak/客户门户 -> Auth Server -> JiuwenSwarm

出站 MCP 凭据：
JiuwenSwarm AgentServer -> Credential Broker -> 客户 OAuth Server -> 客户 MCP Gateway
~~~

Auth Server 确认用户是谁；Credential Broker 决定 JiuwenSwarm 调用客户 MCP 时使用什么凭据。二者位于同一个仓库，共用 OIDC、JWT、Principal、Token Exchange、审计和配置能力，但保持独立模块与权限边界。

## dev-stable 当前能力

JiuwenSwarm dev-stable 已经支持：

- MCP streamable-http（推荐）；
- MCP SSE；
- http / streamable_http 别名归一为 streamable-http；
- 固定 HTTP Header；
- 固定 query parameter；
- 固定 Bearer Token 或 API Key；
- 超时配置、工具发现和工具调用；
- 企业版 MCP 模板下发。

当前不完整支持：

- OAuth client_credentials 自动获取和刷新 token；
- 按租户、用户、MCP Server 和 scope 隔离的动态凭据；
- Token Exchange / On-Behalf-Of；
- MCP OAuth 用户授权流程；
- MCP 级 mTLS 客户端证书配置；
- 通用 credential_ref 和凭据服务。

## 客户条件决策表

| 如果客户是这种情况 | 需要什么 | 是否修改 JiuwenSwarm | Adapter/Broker 是否参与 |
|---|---|---:|---:|
| 标准 Streamable HTTP，无认证 | 配置 URL、transport、timeout，打通 AgentServer 网络 | 否 | 否 |
| 标准 SSE，无认证 | 配置 SSE URL 和 timeout | 否 | 否 |
| 全局共享的固定 Bearer Token | 用 Secret 注入 Authorization Header | 否 | 否 |
| 全局共享的固定 API Key | 用 Secret 注入客户指定 Header/query | 否 | 否 |
| Token 很少变更，可人工轮换 | 更新 Secret，触发 MCP reload/实例更新 | 通常否 | 可选 |
| OAuth2 client_credentials | Broker 保管 client secret，自动取 token、缓存和刷新 | 小幅接入，或使用透明代理 | 是 |
| 每个租户不同 Token | Broker 按 tenant_id + mcp_server_id + scope 隔离凭据 | 是 | 是 |
| 每个用户不同 Token | Auth Server 确认 Principal，Broker 执行 Token Exchange/OBO | 是 | 是 |
| 客户给的 Keycloak token 本来就是 aud=customer-mcp | 验证 issuer/audience/scope，按用户安全传递 | 是，不能写成全局静态 Header | 是 |
| 客户 token 只有 aud=customer-portal | 换成 aud=customer-mcp 的 token | 是 | 是 |
| MCP 要求 OAuth 用户授权/PKCE | Broker 管理授权会话、callback、refresh/revoke | 是 | 是 |
| MCP 要求 mTLS | 配置 CA/client cert/key，或使用内部代理 | 当前需要，除非使用代理 | 可选 |
| 客户使用私有 CA | 将 CA 注入 AgentServer 系统信任库；需要按 MCP 隔离时扩展 TLS 配置 | 取决于部署 | 通常否 |
| 客户要求自定义 HMAC/签名 | Broker Provider 在每次请求前签名 | 需要接入或透明代理 | 是 |
| 不同 Agent 使用不同凭据 | 将 agent_id 纳入策略和缓存维度 | 是 | 是 |
| 不同租户使用不同 MCP URL | 租户模板或 Broker 策略返回 endpoint + credential | 通常是 | 是 |

## 场景 A：只改配置

客户条件：

~~~text
- 标准 streamable-http 或 SSE
- 固定共享 Token/API Key，或者无认证
- 不需要按用户区分权限
- 不需要自动 OAuth 刷新
~~~

只需要配置 JiuwenSwarm：

~~~yaml
mcp:
  servers:
    - name: customer-mcp
      enabled: true
      transport: streamable-http
      url: https://mcp.customer.example/mcp
      headers:
        Authorization: Bearer $CUSTOMER_MCP_TOKEN
      timeout_s: 60
~~~

要求：

- CUSTOMER_MCP_TOKEN 由 Kubernetes Secret 注入实际运行 MCP Client 的 AgentServer；
- AgentServer Pod 能解析并访问客户域名；
- HTTPS 证书链受信任；
- 真实验证 initialize、tools/list、tools/call；
- Ingress、防火墙和代理不会中断长连接。

Auth Adapter 不参与这个场景。

## 场景 B：OAuth client credentials

客户条件：

~~~text
- 客户提供 token endpoint
- Jiuwen 使用 client_id/client_secret
- access token 短时有效
- MCP Gateway 要求特定 audience/scope
~~~

需要 Credential Broker：

~~~text
AgentServer -> Broker -> OAuth token endpoint
                         |
                         +-> aud=customer-mcp token

AgentServer -> MCP Gateway with token
~~~

Broker 需要：

- client secret 只保存在 Secret Manager/Kubernetes Secret；
- 按 tenant + mcp_server + audience + scopes 缓存；
- 在过期前提前刷新；
- 合并并发取票请求，避免 token endpoint 风暴；
- token 获取失败时 fail closed；
- 不记录 client secret 或 access token。

JiuwenSwarm 有两种接法：

1. **Credential Resolver**：AgentServer 连接或调用 MCP 前向 Broker 取短期 Header。需要修改 MCP auth callback。
2. **透明 MCP Auth Proxy**：AgentServer 连接 Broker 代理，由代理取 token 并转发到客户 MCP。对 JiuwenSwarm 改动更小。

## 场景 C：每用户 Token / Token Exchange

客户条件：

~~~text
- 客户 MCP 需要识别张三、李四
- 用户权限由 Keycloak/OIDC scope 决定
- Jiuwen 需要代表最终用户调用 MCP
~~~

必须同时使用 Auth Server 和 Credential Broker：

~~~text
用户 -> Auth Server -> verified Principal
                            |
                            v
                    Credential Broker
                    Token Exchange / OBO
                            |
                            v
                   aud=customer-mcp token
                            |
                            v
                    Customer MCP Gateway
~~~

必须修改 JiuwenSwarm：

- Gateway 验证 Jiuwen 内部 token，产生可信 Principal；
- Principal 安全传到 AgentServer/MCP 调用上下文；
- MCP 凭据解析器向 Broker 提交 tenant_id/user_id/mcp_server_id/scopes；
- 不允许请求 Header、body 或 query 覆盖 Principal；
- 连接池和 token 缓存按用户隔离；
- 会话结束或用户退出时清理用户级 MCP 连接。

缓存键至少包含：

~~~text
tenant_id + user_id + mcp_server_id + audience + scopes
~~~

## 场景 D：mTLS 或自定义签名

客户条件：

~~~text
- MCP Gateway 要求 client certificate
- 或每次请求使用 HMAC/客户私有签名
~~~

优先使用 Broker 的透明代理模式：

- client key 不进入 AgentServer 普通配置；
- Broker 完成 TLS 握手或每请求签名；
- JiuwenSwarm 仅连接内部受控 endpoint；
- Broker 只允许预配置的 MCP 目标，不能成为通用开放代理。

如果要做成 JiuwenSwarm 通用原生能力，则需要扩展 MCP transport 的 TLS 和 request-signing 配置。

## 两种 Token 不能默认混用

Auth Adapter 签发给 JiuwenSwarm 的 token 通常是：

~~~json
{
  "iss": "jiuwen-auth-adapter",
  "aud": "jiuwenswarm",
  "sub": "customer-a:user-123"
}
~~~

客户 MCP 需要的 token 通常是：

~~~json
{
  "iss": "customer-keycloak",
  "aud": "customer-mcp",
  "sub": "user-123",
  "scope": "mcp.tools.execute"
}
~~~

两者 issuer、audience 和 scope 不同。除非客户 MCP 明确信任 Jiuwen issuer，否则不能把 Jiuwen 内部 token 原样转发。

## JiuwenSwarm 需要修改的代码位置

使用动态 MCP 凭据时，预计修改：

~~~text
jiuwenswarm/common/auth/
    加载并传播已验证 Principal

jiuwenswarm/common/mcp_config.py
    支持 credential_ref / credential provider
    不再要求在 auth_headers 中保存长期明文 token

jiuwenswarm/gateway/channel_manager/web/
    验证入站用户身份，建立 Principal

AgentServer MCP 调用上下文
    传递 tenant/user/agent/service scope
    在建连或调用前向 Broker 解析凭据
~~~

建议配置只保存引用：

~~~yaml
mcp:
  servers:
    - name: customer-mcp
      transport: streamable-http
      url: https://mcp.customer.example/mcp
      credential_ref: customer-a/customer-mcp
      audience: customer-mcp
      scopes: [mcp.tools.execute]
~~~

## 客户需要提供的信息

对接前必须收集：

1. MCP transport：streamable-http 还是 SSE；
2. MCP endpoint 和网络边界；
3. 认证方式：无认证、API Key、Bearer、OAuth、mTLS 或自定义签名；
4. 凭据粒度：全局、租户、用户还是 Agent；
5. token endpoint、grant type、audience、scopes、TTL 和刷新方式；
6. 是否要求传递最终用户身份；
7. CA、client certificate、proxy 和 IP 白名单要求；
8. initialize、tools/list、tools/call 的真实样例；
9. 退出、撤销和审计要求；
10. 限流、超时、重试和并发限制。

## 最终判断规则

~~~text
固定共享凭据 + 标准 MCP
    -> 只改 JiuwenSwarm 配置

动态服务级 OAuth token
    -> Credential Broker + Jiuwen 小幅接入，或透明代理

租户级/用户级 token
    -> Auth Server + Credential Broker + Jiuwen Principal/credential resolver 改造

mTLS/自定义签名
    -> 优先 Broker 透明代理；产品化时再增加 Jiuwen 原生扩展
~~~
