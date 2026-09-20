package oidc

// Config describes a standards-based OpenID Connect provider. Keycloak should
// normally use this generic provider rather than a Keycloak-specific code path.
type Config struct {
	IssuerURL    string
	ClientID     string
	ClientSecret string
	RedirectURL  string
	Scopes       []string
}
