---
name: security-review
description: Evidence-backed security audit of this Huma/Chi service, Firebase and GitHub boundaries, separate function, CI, and container.
---

# Task: REST API Security Review

Perform an evidence-backed security review of this Huma/Chi REST API using the
OWASP API Security risks as a lens. This is a read-only audit; do not modify
code, configuration, workflows, or dependencies unless explicitly requested.

## Required File Reads

Before analysis, read these files:

1. `cmd/server/config.go`, `cmd/server/application.go`, and `cmd/server/main.go` - configuration, composition, middleware, and lifecycle
2. `internal/platform/middleware/cors.go`, `forwarded.go`, and `security.go` - trust boundary, CORS, cache, and browser headers
3. `internal/platform/middleware/accesslog.go`, `internal/platform/respond/respond.go`, and `internal/platform/audit/audit.go` - logs, errors, and recovery
4. `internal/platform/auth/` and `internal/platform/firebase/` - identity verification and SDK initialization
5. `internal/service/profile/` and `internal/service/github/` - ownership, persistence, and upstream boundaries
6. All files in `internal/http/v1/` - endpoint contracts and authorization
7. `functions/function.go` and `functions/cmd/server/main.go` - independent HTTP function boundary
8. `go.mod`, `functions/go.mod`, `Justfile`, workflows, `.dockerignore`, and `Dockerfile` - dependency, CI, secret, and artifact controls

## Security Review Checklist

### 1. Authentication & Authorization
- [ ] All protected endpoints require authentication
- [ ] Authorization checks verify user permissions before resource access
- [ ] Token validation handles all error cases (expired, revoked, invalid)
- [ ] `WWW-Authenticate: Bearer` header included in 401 responses
- [ ] No sensitive operations allowed without verified identity
- [ ] Offline Firebase mode fails protected routes closed with 503
- [ ] Production rejects demo projects and emulator hosts

### 2. Input Validation & Data Sanitization
- [ ] All inputs validated via Huma struct tags with strict types
- [ ] Path parameters have proper type constraints
- [ ] Query parameters validated with Huma annotations
- [ ] Request body limits enforced
- [ ] Cursor length, type, and referenced item are validated before pagination
- [ ] GitHub owner/repository inputs, including dot-segment normalization cases, and upstream response sizes are bounded
- [ ] Firestore operations preserve the authenticated user's ownership boundary
- [ ] Firestore conditional mutations use atomic preconditions instead of lock-heavy check-then-write transactions

### 3. Security Headers (OWASP Recommended)
Verify these policies are applied at the correct boundary. Public contracts and
safe public GET responses use `no-cache`; private, mutating, and fallback paths
use `no-store`:

```http
Cache-Control: no-store or no-cache
Content-Security-Policy: default-src 'none'; frame-ancestors 'none'
X-Content-Type-Options: nosniff
X-Frame-Options: DENY
Referrer-Policy: no-referrer
```

### 4. Error Handling & Information Leakage
- [ ] Error responses use RFC 9457 Problem Details format
- [ ] Error responses use generic messages (e.g., "Unauthorized" not internal details)
- [ ] Stack traces never exposed in production
- [ ] Internal exception details logged but not returned to clients
- [ ] Dependency failures are logged once with request correlation and a safe operation name
- [ ] 404 vs 403 responses don't leak resource existence
- [ ] Validation errors don't expose internal field names or structure

### 5. Logging & Monitoring
- [ ] Authentication failures logged with context (IP, endpoint, timestamp)
- [ ] Sensitive data (tokens, passwords, PII) never logged
- [ ] Security events use appropriate log levels (WARN/ERROR)
- [ ] Request correlation IDs present for traceability (X-Request-ID)
- [ ] Audit/security log callers either run under installed observability request context or use an explicit process logger
- [ ] Suspicious patterns (brute force, scanning) would be detectable

### 6. Secrets & Configuration
- [ ] No hardcoded secrets, API keys, or credentials in code
- [ ] Secrets loaded via environment variables
- [ ] Service account files excluded from version control
- [ ] Offline/emulator Firebase modes and wildcard CORS are rejected outside development
- [ ] Emulator endpoints reject malformed authorities, URL schemes, whitespace, and partial configuration
- [ ] Log-level configuration cannot select terminating Zap levels

### 7. CORS & Origin Policy
- [ ] CORS origins explicitly defined (no wildcards in production)
- [ ] Credentials allowed only from trusted origins
- [ ] Preflight requests handled correctly
- [ ] Methods and headers properly restricted
- [ ] Vary header includes Origin for proper caching
- [ ] HSTS is configured at the trusted TLS edge, not emitted on local HTTP responses

### 8. Rate Limiting & DoS Protection
- [ ] Absence of an application rate limiter is evaluated against the actual deployment boundary; do not recommend a misleading per-instance limiter without a concrete quota model
- [ ] Request body size limits enforced
- [ ] Timeouts configured for external service calls
- [ ] Pagination limits on list endpoints (cursor-based pagination)
- [ ] GitHub upstream errors and bodies are bounded and do not leak credentials

### 9. Insecure Direct Object References (IDOR)
- [ ] Users can only access resources they own
- [ ] Resource ownership verified before read/update/delete
- [ ] UUIDs or non-sequential IDs used where appropriate
- [ ] Bulk operations validate all resource access

### 10. Panic Recovery
- [ ] Panic recovery middleware is in place
- [ ] Panics are logged with stack traces (server-side only)
- [ ] Panics return proper Problem Details response to client
- [ ] No sensitive information leaked in panic responses

### 11. Content Negotiation Security
- [ ] Unsupported content types return 415 Unsupported Media Type
- [ ] Accept fallback and JSON/CBOR negotiation behavior are intentional and tested
- [ ] Response content type matches negotiated type
- [ ] CBOR/JSON handling is secure
- [ ] Chi 404/405/recovery and Huma validation errors share the same negotiation and schema-link behavior

### 12. Dependency Security
- [ ] Both modules are updated through `just update`
- [ ] Both modules pass `just vuln`
- [ ] Required CI covers both modules, emulator integration, dependency review, and the final container artifact
- [ ] Minimal dependency footprint

### 13. Function, Container & Supply Chain
- [ ] The independent function enforces its method, media type, body-size, single-object JSON, and `Allow` contracts
- [ ] Deployment guidance uses Cloud Run source functions rather than unsupported Firebase CLI Go deployment
- [ ] The final image is non-root, contains only the compiled server, and reports honest version and OCI metadata
- [ ] Docker build inputs exclude credentials, local environments, coverage output, and repository-only artifacts
- [ ] Workflow permissions are least privilege, checkout credentials are not persisted, and actions use exact release tags
- [ ] Stable `ci` and `lint` aggregate jobs fail unless every required internal job succeeds

## Output Format

Provide findings in this structure:

### Critical Issues
Issues requiring immediate attention (authentication bypass, data exposure, injection).

### High Priority
Significant security gaps (missing authorization, weak validation).

### Medium Priority
Best practice violations (logging gaps, incomplete headers).

### Low Priority / Recommendations
Enhancements for defense in depth.

### Security Strengths
Patterns implemented correctly that should be maintained.

For each finding include:

- **Location**: File path and line number
- **Issue**: Clear description of the vulnerability
- **Risk**: Potential impact if exploited
- **Recommendation**: Specific remediation steps

Separate verified vulnerabilities from hardening opportunities. Do not report
the intentional lack of in-process rate limiting, local HTTP without HSTS, or
ignored forwarded headers as vulnerabilities unless the documented deployment
model makes them exploitable. Record checks that could not run and why.

## Huma/Chi-Specific Considerations

- Check Chi middleware ordering (security-critical middleware should run early)
- Verify Huma validation tags are comprehensive
- Ensure Problem Details responses don't leak internal state
- Check that all routes use appropriate Huma error helpers
- Verify CORS middleware configuration in Chi stack
- Check request ID propagation for audit trails
- Ensure structured logging redacts sensitive fields
- Verify untrusted forwarding headers are removed before observability and Huma construct client or schema metadata
- Verify the mounted Huma router owns operation access logs while Chi-only routes and fallback handlers are logged exactly once
