package auth

import "github.com/danielgtaylor/huma/v2"

const BearerAuthScheme = "bearerAuth"

// RequireAuth returns Huma operation security requirements for Firebase bearer tokens.
func RequireAuth() []map[string][]string {
	return []map[string][]string{{BearerAuthScheme: {}}}
}

// RegisterSecurityScheme adds the Firebase bearer-token security scheme to OpenAPI.
func RegisterSecurityScheme(api huma.API) {
	openAPI := api.OpenAPI()
	if openAPI.Components.SecuritySchemes == nil {
		openAPI.Components.SecuritySchemes = make(map[string]*huma.SecurityScheme)
	}
	openAPI.Components.SecuritySchemes[BearerAuthScheme] = &huma.SecurityScheme{
		Type:         "http",
		Scheme:       "bearer",
		BearerFormat: "JWT",
		Description:  "Firebase ID token",
	}
}
