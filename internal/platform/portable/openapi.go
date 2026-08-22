package portable

import (
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/danielgtaylor/huma/v2"
)

const (
	boundedNameSchemaPattern = `^(?![\u0009-\u000D\u0020\u0085\u00A0\u1680\u2000-\u200A\u2028\u2029\u202F\u205F\u3000])(?![\s\S]*[\u0009-\u000D\u0020\u0085\u00A0\u1680\u2000-\u200A\u2028\u2029\u202F\u205F\u3000]$)(?![\s\S]*[\u0000-\u001F\u007F-\u009F])[\s\S]+$`
	emailCoreSchemaPattern   = `(?!\.)(?![^@]*\.\.)(?![^@]*\.@)[A-Za-z0-9!#$%&'*+/=?^_{|}~.-]{1,64}@[A-Za-z0-9](?:[A-Za-z0-9-]{0,61}[A-Za-z0-9])?(?:\.[A-Za-z0-9](?:[A-Za-z0-9-]{0,61}[A-Za-z0-9])?)+`
	canonicalEmailPattern    = `^(?!\.)(?![^@]*\.\.)(?![^@]*\.@)[A-Za-z0-9!#$%&'*+/=?^_{|}~.-]{1,64}@[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)+$`
	requestEmailPattern      = `^[\u0009-\u000D\u0020]*(?=[^\u0009-\u000D\u0020]{1,254}[\u0009-\u000D\u0020]*$)` + emailCoreSchemaPattern + `[\u0009-\u000D\u0020]*$`
	requestPhonePattern      = `^[\u0009-\u000D\u0020]*\+[1-9][0-9]{6,14}[\u0009-\u000D\u0020]*$`
	timestampSchemaPattern   = `^[0-9]{4}-(0[1-9]|1[0-2])-(0[1-9]|[12][0-9]|3[01])T([01][0-9]|2[0-3]):[0-5][0-9]:[0-5][0-9]\.[0-9]{3}Z$`
)

// InstallOpenAPIHook projects the shared GCP representation and response
// headers from the same operations that Huma registers at runtime.
func InstallOpenAPIHook(api huma.API) {
	api.OpenAPI().OnAddOperation = append(
		api.OpenAPI().OnAddOperation,
		func(_ *huma.OpenAPI, operation *huma.Operation) {
			if operation.RequestBody != nil {
				if jsonMedia := operation.RequestBody.Content[MediaTypeJSON]; jsonMedia != nil {
					operation.RequestBody.Content = map[string]*huma.MediaType{
						MediaTypeJSON: jsonMedia,
						MediaTypeCBOR: jsonMedia,
					}
				}
			}
			for status, response := range operation.Responses {
				if response == nil {
					continue
				}
				if response.Content != nil {
					if problemMedia := response.Content[MediaTypeProblemJSON]; problemMedia != nil {
						response.Content = map[string]*huma.MediaType{
							MediaTypeProblemJSON: problemMedia,
							MediaTypeCBOR:        problemMedia,
						}
					} else if jsonMedia := response.Content[MediaTypeJSON]; jsonMedia != nil {
						response.Content = map[string]*huma.MediaType{
							MediaTypeJSON: jsonMedia,
							MediaTypeCBOR: jsonMedia,
						}
					}
				}
				addPortableResponseHeaders(response)
				if status == strconv.Itoa(http.StatusUnauthorized) && protected(operation.Security) {
					addStringHeader(response, "WWW-Authenticate", "Bearer authentication challenge")
				}
				if status == strconv.Itoa(http.StatusTooManyRequests) {
					addSafeIntegerHeader(response, "Retry-After", "Delay in seconds before retrying", true)
					addSafeIntegerHeader(response, "X-RateLimit-Reset", "GitHub quota reset Unix time", false)
				}
			}
			requestHeader := &huma.Param{
				Name:        "X-Request-ID",
				In:          "header",
				Required:    false,
				Description: "Optional 1-128 character request ID. Missing, invalid, repeated, or comma-combined values are replaced.",
				Schema: &huma.Schema{
					Type:      huma.TypeString,
					MinLength: new(1),
					MaxLength: new(128),
					Pattern:   `^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`,
				},
			}
			if !slices.ContainsFunc(operation.Parameters, func(parameter *huma.Param) bool {
				return parameter.In == "header" && parameter.Name == "X-Request-ID"
			}) {
				operation.Parameters = append(operation.Parameters, requestHeader)
			}
		},
	)
}

// FinalizeOpenAPI applies semantic constraints which Huma cannot infer from
// Go field types alone. It mutates only the native runtime-generated document.
func FinalizeOpenAPI(api huma.API) {
	document := api.OpenAPI()
	if document.Components == nil || document.Components.Schemas == nil {
		return
	}
	schemas := document.Components.Schemas.Map()
	finalizePortableSchemas(schemas)
	installProblemSchemas(schemas)
	for _, path := range document.Paths {
		for _, operation := range pathOperations(path) {
			finalizeOperationResponses(operation)
		}
	}
}

func finalizePortableSchemas(schemas map[string]*huma.Schema) {
	setMinProperties(schemas["ProfileUpdateBody"], 1)
	if source := schemas["Source"]; source != nil {
		one := 1
		source.MinProperties = &one
		source.MaxProperties = &one
		for _, property := range source.Properties {
			property.MinLength = &one
		}
	}
	if issue := schemas["Issue"]; issue != nil {
		if source := issue.Properties["source"]; source != nil {
			source.Nullable = false
		}
	}
	for name, schema := range schemas {
		if schema == nil {
			continue
		}
		for propertyName, property := range schema.Properties {
			if property == nil {
				continue
			}
			switch propertyName {
			case "createdAt", "updatedAt", "pushedAt", "timestamp":
				if property.Format == "date-time" {
					property.Pattern = timestampSchemaPattern
				}
			case "firstName", "lastName", "name":
				if boundedNameProperty(name, propertyName) {
					property.Pattern = boundedNameSchemaPattern
				}
			}
		}
	}
	setRequestContactSchemas(schemas["ProfileCreateInputBody"])
	setRequestContactSchemas(schemas["ProfileUpdateBody"])
	setResponseContactSchemas(schemas["Profile"])
	if itemPage := schemas["ListData"]; itemPage != nil {
		if items := itemPage.Properties["items"]; items != nil {
			items.MaxItems = new(100)
		}
		if total := itemPage.Properties["total"]; total != nil {
			total.Minimum = new(float64(0))
			total.Maximum = new(float64(9_007_199_254_740_991))
		}
	}
	if repository := schemas["Repository"]; repository != nil {
		if license := repository.Properties["license"]; license != nil {
			license.MinLength = new(1)
		}
	}
}

func boundedNameProperty(schemaName, propertyName string) bool {
	switch schemaName {
	case "HelloCreateInputBody":
		return propertyName == "name"
	case "Item":
		return propertyName == "name"
	case "Profile", "ProfileCreateInputBody", "ProfileUpdateBody":
		return propertyName == "firstName" || propertyName == "lastName"
	default:
		return false
	}
}

func setRequestContactSchemas(schema *huma.Schema) {
	if schema == nil {
		return
	}
	if email := schema.Properties["contactEmail"]; email != nil {
		email.MinLength = nil
		email.MaxLength = nil
		email.Pattern = requestEmailPattern
	}
	if phone := schema.Properties["phoneNumber"]; phone != nil {
		phone.Pattern = requestPhonePattern
	}
}

func setResponseContactSchemas(schema *huma.Schema) {
	if schema == nil {
		return
	}
	if email := schema.Properties["contactEmail"]; email != nil {
		email.Pattern = canonicalEmailPattern
	}
	if phone := schema.Properties["phoneNumber"]; phone != nil {
		phone.Pattern = `^\+[1-9][0-9]{6,14}$`
	}
}

func setMinProperties(schema *huma.Schema, value int) {
	if schema != nil {
		schema.MinProperties = &value
	}
}

func installProblemSchemas(schemas map[string]*huma.Schema) {
	for code, definition := range problemDefinitions {
		name := problemSchemaName(code)
		schemas[name] = &huma.Schema{
			Type:                 huma.TypeObject,
			AdditionalProperties: false,
			Properties: map[string]*huma.Schema{
				"type":   {Type: huma.TypeString, Enum: []any{"about:blank"}},
				"title":  {Type: huma.TypeString, Enum: []any{definition.Title}},
				"status": {Type: huma.TypeInteger, Enum: []any{definition.Status}},
				"detail": {Type: huma.TypeString, Enum: []any{definition.Detail}},
				"code":   {Type: huma.TypeString, Enum: []any{string(code)}},
				"errors": {
					Type: huma.TypeArray, Items: &huma.Schema{Ref: "#/components/schemas/Issue"},
					MinItems: new(1), MaxItems: new(32),
				},
			},
			Required: []string{"title", "status", "detail", "code"},
		}
	}
}

func finalizeOperationResponses(operation *huma.Operation) {
	if operation == nil {
		return
	}
	for rawStatus, response := range operation.Responses {
		status, err := strconv.Atoi(rawStatus)
		if err != nil || status < 400 || response == nil {
			continue
		}
		code := responseCode(operation.OperationID, status)
		definition, ok := problemDefinitions[code]
		if !ok {
			continue
		}
		response.Description = definition.Title
		for mediaType, media := range response.Content {
			if (mediaType == MediaTypeProblemJSON || mediaType == MediaTypeCBOR) && media != nil {
				media.Schema = &huma.Schema{Ref: "#/components/schemas/" + problemSchemaName(code)}
			}
		}
		finalizeResponseHeaders(response)
	}
	for _, response := range operation.Responses {
		finalizeResponseHeaders(response)
	}
}

func responseCode(operationID string, status int) Code {
	switch status {
	case http.StatusBadRequest:
		return CodeInvalidRequest
	case http.StatusUnauthorized:
		return CodeUnauthorized
	case http.StatusNotFound:
		if strings.HasPrefix(operationID, "getGitHub") || strings.HasPrefix(operationID, "listGitHub") {
			return CodeGitHubNotFound
		}
		return CodeProfileNotFound
	case http.StatusNotAcceptable:
		return CodeNotAcceptable
	case http.StatusConflict:
		return CodeProfileExists
	case http.StatusRequestEntityTooLarge:
		return CodePayloadTooLarge
	case http.StatusUnsupportedMediaType:
		return CodeUnsupportedMediaType
	case http.StatusUnprocessableEntity:
		return CodeValidationFailed
	case http.StatusTooManyRequests:
		return CodeGitHubRateLimit
	case http.StatusInternalServerError:
		return CodeInternalError
	case http.StatusBadGateway:
		return CodeGitHubUpstream
	case http.StatusServiceUnavailable:
		return CodeDependencyUnavailable
	case http.StatusGatewayTimeout:
		return CodeGitHubTimeout
	default:
		return ""
	}
}

func finalizeResponseHeaders(response *huma.Response) {
	if response == nil {
		return
	}
	headerEnums := map[string]string{
		"Cache-Control": "no-store", "X-Content-Type-Options": "nosniff",
		"X-Frame-Options": "DENY", "Referrer-Policy": "strict-origin-when-cross-origin",
		"WWW-Authenticate": "Bearer", "Location": "/v1/profile",
	}
	for name, value := range headerEnums {
		if header := response.Headers[name]; header != nil && header.Schema != nil {
			header.Schema.Enum = []any{value}
		}
	}
	if requestID := response.Headers["X-Request-ID"]; requestID != nil && requestID.Schema != nil {
		requestID.Schema.MinLength = new(1)
		requestID.Schema.MaxLength = new(128)
		requestID.Schema.Pattern = `^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`
	}
}

func problemSchemaName(code Code) string {
	parts := strings.Split(string(code), "_")
	for index := range parts {
		parts[index] = strings.ToUpper(parts[index][:1]) + parts[index][1:]
	}
	return "Problem" + strings.Join(parts, "")
}

func pathOperations(path *huma.PathItem) []*huma.Operation {
	if path == nil {
		return nil
	}
	return []*huma.Operation{
		path.Get,
		path.Put,
		path.Post,
		path.Delete,
		path.Options,
		path.Head,
		path.Patch,
		path.Trace,
	}
}

func protected(security []map[string][]string) bool {
	return len(security) > 0
}

func addPortableResponseHeaders(response *huma.Response) {
	addStringHeader(response, "X-Request-ID", "Selected request correlation identifier")
	addStringHeader(response, "Cache-Control", "Response cache policy")
	addStringHeader(response, "X-Content-Type-Options", "MIME sniffing policy")
	addStringHeader(response, "X-Frame-Options", "Framing policy")
	addStringHeader(response, "Referrer-Policy", "Referrer policy")
	addStringHeader(response, "Vary", "Fields used for response selection")
}

func addStringHeader(response *huma.Response, name, description string) {
	if response.Headers == nil {
		response.Headers = make(map[string]*huma.Param)
	}
	if _, exists := response.Headers[name]; exists {
		return
	}
	response.Headers[name] = &huma.Param{
		Description: description,
		Schema:      &huma.Schema{Type: huma.TypeString},
	}
}

func addSafeIntegerHeader(response *huma.Response, name, description string, required bool) {
	if response.Headers == nil {
		response.Headers = make(map[string]*huma.Param)
	}
	response.Headers[name] = &huma.Param{
		Description: description,
		Required:    required,
		Schema: &huma.Schema{
			Type:    huma.TypeInteger,
			Minimum: new(float64(0)),
			Maximum: new(float64(9_007_199_254_740_991)),
		},
	}
}
