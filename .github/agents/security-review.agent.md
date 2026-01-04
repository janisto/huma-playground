---
name: security-review
description: Comprehensive Huma/Chi REST API security audit based on OWASP best practices
---

# Task: REST API Security Review

Perform a comprehensive security review of this Huma/Chi REST API following OWASP API Security guidelines. **This is a read-only audit**; do not modify code unless explicitly requested.

## Required File Reads

Before analysis, read these files:
1. `cmd/server/main.go` - Application setup, middleware, and CORS configuration
2. `internal/platform/middleware/cors.go` - CORS configuration
3. `internal/platform/middleware/security.go` - Security headers middleware
4. `internal/platform/middleware/requestid.go` - Request ID middleware
5. `internal/platform/logging/middleware.go` - Request logging middleware
6. `internal/platform/respond/respond.go` - Error handling and panic recovery
7. All files in `internal/http/v1/` - Endpoint definitions
8. `internal/platform/logging/logger.go` - Logger configuration

## Security Review Checklist

### 1. Authentication & Authorization
- [ ] All protected endpoints require authentication
- [ ] Authorization checks verify user permissions before resource access
- [ ] Token validation handles all error cases (expired, revoked, invalid)
- [ ] `WWW-Authenticate: Bearer` header included in 401 responses
- [ ] No sensitive operations allowed without verified identity

### 2. Input Validation & Data Sanitization
- [ ] All inputs validated via Huma struct tags with strict types
- [ ] Path parameters have proper type constraints
- [ ] Query parameters validated with Huma annotations
- [ ] Request body limits enforced
- [ ] No raw string interpolation in database queries (prevent injection)

### 3. Security Headers (OWASP Recommended)
Verify these headers are set in security middleware:
```http
Cache-Control: no-store
Content-Security-Policy: default-src 'none'
Content-Type: application/json
Strict-Transport-Security: max-age=31536000; includeSubDomains
X-Content-Type-Options: nosniff
X-Frame-Options: DENY
Referrer-Policy: no-referrer
```

### 4. Error Handling & Information Leakage
- [ ] Error responses use RFC 9457 Problem Details format
- [ ] Error responses use generic messages (e.g., "Unauthorized" not internal details)
- [ ] Stack traces never exposed in production
- [ ] Internal exception details logged but not returned to clients
- [ ] 404 vs 403 responses don't leak resource existence
- [ ] Validation errors don't expose internal field names or structure

### 5. Logging & Monitoring
- [ ] Authentication failures logged with context (IP, endpoint, timestamp)
- [ ] Sensitive data (tokens, passwords, PII) never logged
- [ ] Security events use appropriate log levels (WARN/ERROR)
- [ ] Request correlation IDs present for traceability (X-Request-ID)
- [ ] Suspicious patterns (brute force, scanning) would be detectable

### 6. Secrets & Configuration
- [ ] No hardcoded secrets, API keys, or credentials in code
- [ ] Secrets loaded via environment variables
- [ ] Service account files excluded from version control
- [ ] Debug mode disabled in production configuration

### 7. CORS & Origin Policy
- [ ] CORS origins explicitly defined (no wildcards in production)
- [ ] Credentials allowed only from trusted origins
- [ ] Preflight requests handled correctly
- [ ] Methods and headers properly restricted
- [ ] Vary header includes Origin for proper caching

### 8. Rate Limiting & DoS Protection
- [ ] Rate limiting configured (if applicable)
- [ ] Request body size limits enforced
- [ ] Timeouts configured for external service calls
- [ ] Pagination limits on list endpoints (cursor-based pagination)

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
- [ ] Accept header is validated
- [ ] Response content type matches negotiated type
- [ ] CBOR/JSON handling is secure

### 12. Dependency Security
- [ ] Dependencies up to date (`go get -u ./...`)
- [ ] No known vulnerabilities in dependencies (`govulncheck ./...`)
- [ ] Minimal dependency footprint

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

## Huma/Chi-Specific Considerations

- Check Chi middleware ordering (security-critical middleware should run early)
- Verify Huma validation tags are comprehensive
- Ensure Problem Details responses don't leak internal state
- Check that all routes use appropriate Huma error helpers
- Verify CORS middleware configuration in Chi stack
- Check request ID propagation for audit trails
- Ensure structured logging redacts sensitive fields
