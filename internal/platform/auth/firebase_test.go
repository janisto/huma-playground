package auth

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"maps"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	firebase "firebase.google.com/go/v4"
	fbauth "firebase.google.com/go/v4/auth"

	"github.com/janisto/huma-playground/internal/platform/portable"
	"github.com/janisto/huma-playground/internal/testutil"
)

const deterministicFirebaseProjectID = "portable-auth-test"

func TestExtractBearerTokenValid(t *testing.T) {
	tests := []struct {
		name   string
		header string
		want   string
	}{
		{
			name:   "lowercase bearer",
			header: "bearer token123",
			want:   "token123",
		},
		{
			name:   "uppercase Bearer",
			header: "Bearer token123",
			want:   "token123",
		},
		{
			name:   "mixed case BEARER",
			header: "BEARER token123",
			want:   "token123",
		},
		{
			name:   "token with dots (JWT-like)",
			header: "Bearer eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.sig",
			want:   "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.sig",
		},
		{
			name:   "multiple separator spaces",
			header: "Bearer   token123",
			want:   "token123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ExtractBearerToken(tt.header)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExtractBearerTokenEmpty(t *testing.T) {
	_, err := ExtractBearerToken("")
	if !errors.Is(err, ErrNoToken) {
		t.Fatalf("expected ErrNoToken, got %v", err)
	}
}

func TestExtractBearerTokenInvalid(t *testing.T) {
	tests := []struct {
		name   string
		header string
	}{
		{
			name:   "missing scheme",
			header: "token123",
		},
		{
			name:   "wrong scheme",
			header: "Basic token123",
		},
		{
			name:   "basic auth format",
			header: "Basic dXNlcjpwYXNz",
		},
		{
			name:   "bearer without token",
			header: "Bearer",
		},
		{
			name:   "bearer with trailing space and no token",
			header: "Bearer ",
		},
		{
			name:   "bearer with whitespace token only",
			header: "Bearer    ",
		},
		{
			name:   "bearer token with extra segment",
			header: "Bearer token123 extra",
		},
		{
			name:   "horizontal tab separator",
			header: "Bearer\ttoken123",
		},
		{
			name:   "comma combined credentials",
			header: "Bearer token123,Bearer token456",
		},
		{
			name:   "credential trailing space before protocol normalization",
			header: "Bearer token123 ",
		},
		{
			name:   "only spaces",
			header: "   ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ExtractBearerToken(tt.header)
			if !errors.Is(err, ErrInvalidToken) {
				t.Fatalf("expected ErrInvalidToken, got %v", err)
			}
		})
	}
}

func TestFirebaseUserFields(t *testing.T) {
	user := FirebaseUser{
		UID:           "user-123",
		Email:         "test@example.com",
		EmailVerified: true,
	}

	if user.UID != "user-123" {
		t.Fatalf("expected UID user-123, got %s", user.UID)
	}
	if user.Email != "test@example.com" {
		t.Fatalf("expected email test@example.com, got %s", user.Email)
	}
	if !user.EmailVerified {
		t.Fatal("expected EmailVerified to be true")
	}
}

func TestErrorTypes(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{"ErrNoToken", ErrNoToken, "missing authorization header"},
		{"ErrInvalidToken", ErrInvalidToken, "invalid token"},
		{"ErrTokenExpired", ErrTokenExpired, "token expired"},
		{"ErrTokenRevoked", ErrTokenRevoked, "token revoked"},
		{"ErrUserDisabled", ErrUserDisabled, "user disabled"},
		{"ErrCertificateFetch", ErrCertificateFetch, "failed to fetch certificates"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Error() != tt.want {
				t.Fatalf("got %q, want %q", tt.err.Error(), tt.want)
			}
		})
	}
}

func TestNewFirebaseVerifier(t *testing.T) {
	testutil.SkipIfEmulatorUnavailable(t)
	testutil.SetupEmulator(t)

	ctx := t.Context()
	config := &firebase.Config{ProjectID: testutil.ProjectID}
	fbApp, err := firebase.NewApp(ctx, config)
	if err != nil {
		t.Fatalf("failed to create Firebase app: %v", err)
	}

	ac, err := fbApp.Auth(ctx)
	if err != nil {
		t.Fatalf("failed to create auth client: %v", err)
	}

	verifier := NewFirebaseVerifier(ac)
	if verifier == nil {
		t.Fatal("expected non-nil verifier")
		return
	}
	if verifier.client != ac {
		t.Fatal("expected verifier.client to be the provided auth client")
	}
}

func TestFirebaseVerifierRejectsMissingClient(t *testing.T) {
	for _, verifier := range []*FirebaseVerifier{nil, NewFirebaseVerifier(nil)} {
		if _, err := verifier.Verify(t.Context(), "token"); !errors.Is(err, ErrAuthUnavailable) {
			t.Fatalf("expected ErrAuthUnavailable, got %v", err)
		}
	}
}

func TestFirebaseVerifierVerifyValidToken(t *testing.T) {
	testutil.SkipIfEmulatorUnavailable(t)
	testutil.SetupEmulator(t)

	ctx := t.Context()
	testutil.ClearAccounts(t)

	config := &firebase.Config{ProjectID: testutil.ProjectID}
	fbApp, err := firebase.NewApp(ctx, config)
	if err != nil {
		t.Fatalf("failed to create Firebase app: %v", err)
	}

	ac, err := fbApp.Auth(ctx)
	if err != nil {
		t.Fatalf("failed to create auth client: %v", err)
	}

	result := testutil.CreateTestUser(t, "verify@example.com", "password123")

	verifier := NewFirebaseVerifier(ac)
	user, err := verifier.Verify(ctx, result.IDToken)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if user.UID == "" {
		t.Fatal("expected non-empty UID")
	}
	if user.Email != "verify@example.com" {
		t.Fatalf("expected email verify@example.com, got %s", user.Email)
	}
}

func TestFirebaseVerifierVerifyInvalidToken(t *testing.T) {
	testutil.SkipIfEmulatorUnavailable(t)
	testutil.SetupEmulator(t)

	ctx := t.Context()

	config := &firebase.Config{ProjectID: testutil.ProjectID}
	fbApp, err := firebase.NewApp(ctx, config)
	if err != nil {
		t.Fatalf("failed to create Firebase app: %v", err)
	}

	ac, err := fbApp.Auth(ctx)
	if err != nil {
		t.Fatalf("failed to create auth client: %v", err)
	}

	verifier := NewFirebaseVerifier(ac)
	_, err = verifier.Verify(ctx, "invalid-token-string")
	if err == nil {
		t.Fatal("expected error for invalid token")
	}
	if !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("expected ErrInvalidToken, got %v", err)
	}
}

func TestFirebaseVerifierVerifyRevokedToken(t *testing.T) {
	testutil.SkipIfEmulatorUnavailable(t)
	testutil.SetupEmulator(t)
	testutil.ClearAccounts(t)

	ctx := t.Context()

	config := &firebase.Config{ProjectID: testutil.ProjectID}
	fbApp, err := firebase.NewApp(ctx, config)
	if err != nil {
		t.Fatalf("failed to create Firebase app: %v", err)
	}

	ac, err := fbApp.Auth(ctx)
	if err != nil {
		t.Fatalf("failed to create auth client: %v", err)
	}

	result := testutil.CreateTestUser(t, "revoke@example.com", "password123")

	if err := ac.RevokeRefreshTokens(ctx, result.LocalID); err != nil {
		t.Fatalf("failed to revoke tokens: %v", err)
	}

	verifier := NewFirebaseVerifier(ac)
	_, verifyErr := verifier.Verify(ctx, result.IDToken)
	if verifyErr == nil {
		t.Skip("emulator does not enforce token revocation checks")
	}
	if !errors.Is(verifyErr, ErrTokenRevoked) && !errors.Is(verifyErr, ErrInvalidToken) {
		t.Fatalf("expected ErrTokenRevoked or ErrInvalidToken, got %v", verifyErr)
	}
}

func TestFirebaseVerifierVerifyDisabledUser(t *testing.T) {
	testutil.SkipIfEmulatorUnavailable(t)
	testutil.SetupEmulator(t)
	testutil.ClearAccounts(t)

	ctx := t.Context()

	config := &firebase.Config{ProjectID: testutil.ProjectID}
	fbApp, err := firebase.NewApp(ctx, config)
	if err != nil {
		t.Fatalf("failed to create Firebase app: %v", err)
	}

	ac, err := fbApp.Auth(ctx)
	if err != nil {
		t.Fatalf("failed to create auth client: %v", err)
	}

	result := testutil.CreateTestUser(t, "disabled@example.com", "password123")

	disabled := false
	_, err = ac.UpdateUser(ctx, result.LocalID, (&fbauth.UserToUpdate{}).Disabled(true))
	if err != nil {
		t.Logf("emulator may not support DisableUser: %v", err)
	} else {
		disabled = true
	}

	if disabled {
		verifier := NewFirebaseVerifier(ac)
		_, verifyErr := verifier.Verify(ctx, result.IDToken)
		if verifyErr == nil {
			t.Skip("emulator does not enforce disable check on token verification")
		}
		if !errors.Is(verifyErr, ErrUserDisabled) && !errors.Is(verifyErr, ErrInvalidToken) {
			t.Fatalf("expected ErrUserDisabled or ErrInvalidToken, got %v", verifyErr)
		}
	}
}

func TestFirebaseVerifierRejectsDeletedUser(t *testing.T) {
	testutil.SkipIfEmulatorUnavailable(t)
	testutil.SetupEmulator(t)
	testutil.ClearAccounts(t)

	ctx := t.Context()
	fbApp, err := firebase.NewApp(ctx, &firebase.Config{ProjectID: testutil.ProjectID})
	if err != nil {
		t.Fatalf("create Firebase app: %v", err)
	}
	ac, err := fbApp.Auth(ctx)
	if err != nil {
		t.Fatalf("create Auth client: %v", err)
	}
	result := testutil.CreateTestUser(t, "deleted@example.com", "password123")
	if deleteErr := ac.DeleteUser(ctx, result.LocalID); deleteErr != nil {
		t.Fatalf("delete user: %v", deleteErr)
	}

	_, err = NewFirebaseVerifier(ac).Verify(ctx, result.IDToken)
	if !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("deleted user verification error = %v, want ErrInvalidToken", err)
	}
	if errors.Is(err, ErrAuthUnavailable) {
		t.Fatalf("deleted user verification error = %v, must not be dependency unavailable", err)
	}
}

type deterministicFirebaseFixture struct {
	mu                sync.Mutex
	server            *httptest.Server
	signingKey        *rsa.PrivateKey
	certificatePEM    string
	certificateStatus int
	certificateCalls  int
	identityDocument  string
	identityCalls     int
}

func newDeterministicFirebaseFixture(t *testing.T) *deterministicFirebaseFixture {
	t.Helper()
	now := time.Now()
	signingKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate Firebase signing key: %v", err)
	}
	rootTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "synthetic Firebase test root"},
		NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour), IsCA: true, BasicConstraintsValid: true,
		KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
	}
	rootDER, err := x509.CreateCertificate(rand.Reader, rootTemplate, rootTemplate, &signingKey.PublicKey, signingKey)
	if err != nil {
		t.Fatalf("create Firebase signing certificate: %v", err)
	}
	rootCertificate, err := x509.ParseCertificate(rootDER)
	if err != nil {
		t.Fatalf("parse Firebase signing certificate: %v", err)
	}
	serverKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate local Firebase TLS key: %v", err)
	}
	serverTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(2), Subject: pkix.Name{CommonName: "synthetic Firebase provider"},
		DNSNames:  []string{"www.googleapis.com", "oauth2.googleapis.com", "identitytoolkit.googleapis.com"},
		NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour),
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	serverDER, err := x509.CreateCertificate(
		rand.Reader, serverTemplate, rootCertificate, &serverKey.PublicKey, signingKey,
	)
	if err != nil {
		t.Fatalf("create local Firebase TLS certificate: %v", err)
	}
	fixture := &deterministicFirebaseFixture{
		signingKey:       signingKey,
		certificatePEM:   string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: rootDER})),
		identityDocument: `{"users":[{"localId":"principal-a","validSince":"0","disabled":false}]}`,
	}
	server := httptest.NewUnstartedServer(http.HandlerFunc(fixture.serveHTTP))
	server.EnableHTTP2 = false
	server.TLS = &tls.Config{
		MinVersion: tls.VersionTLS12,
		Certificates: []tls.Certificate{{
			Certificate: [][]byte{serverDER, rootDER},
			PrivateKey:  serverKey,
		}},
	}
	server.StartTLS()
	fixture.server = server

	roots := x509.NewCertPool()
	roots.AddCert(rootCertificate)
	dialer := &net.Dialer{Timeout: 2 * time.Second}
	transport := &http.Transport{
		Proxy: nil,
		DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
			return dialer.DialContext(ctx, network, server.Listener.Addr().String())
		},
		TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots},
	}
	originalTransport := http.DefaultTransport
	http.DefaultTransport = transport
	t.Cleanup(func() {
		transport.CloseIdleConnections()
		http.DefaultTransport = originalTransport
		server.Close()
	})
	return fixture
}

func (fixture *deterministicFirebaseFixture) serveHTTP(response http.ResponseWriter, request *http.Request) {
	fixture.mu.Lock()
	defer fixture.mu.Unlock()
	response.Header().Set("Content-Type", "application/json")
	switch {
	case strings.Contains(request.URL.Path, "/robot/v1/metadata/x509/"):
		fixture.certificateCalls++
		if fixture.certificateStatus != 0 {
			response.WriteHeader(fixture.certificateStatus)
			_, _ = response.Write([]byte(`{"error":"certificate service unavailable"}`))
			return
		}
		response.Header().Set("Cache-Control", "public, max-age=3600")
		if err := json.NewEncoder(response).Encode(map[string]string{"known-key": fixture.certificatePEM}); err != nil {
			response.WriteHeader(http.StatusInternalServerError)
		}
	case request.URL.Path == "/token":
		_, _ = response.Write(
			[]byte(`{"access_token":"synthetic-access-token","token_type":"Bearer","expires_in":3600}`),
		)
	case strings.HasSuffix(request.URL.Path, "/accounts:lookup"):
		fixture.identityCalls++
		_, _ = response.Write([]byte(fixture.identityDocument))
	default:
		http.NotFound(response, request)
	}
}

func (fixture *deterministicFirebaseFixture) setIdentityDocument(document string) {
	fixture.mu.Lock()
	defer fixture.mu.Unlock()
	fixture.identityDocument = document
}

func (fixture *deterministicFirebaseFixture) setCertificateStatus(status int) {
	fixture.mu.Lock()
	defer fixture.mu.Unlock()
	fixture.certificateStatus = status
}

func (fixture *deterministicFirebaseFixture) callCounts() (certificate, identity int) {
	fixture.mu.Lock()
	defer fixture.mu.Unlock()
	return fixture.certificateCalls, fixture.identityCalls
}

func (fixture *deterministicFirebaseFixture) credentialsPath(t *testing.T) string {
	t.Helper()
	privateKey, err := x509.MarshalPKCS8PrivateKey(fixture.signingKey)
	if err != nil {
		t.Fatalf("marshal synthetic service-account key: %v", err)
	}
	document, err := json.Marshal(
		map[string]string{ //nolint:gosec // Synthetic test-only credentials never leave t.TempDir.
			"type":           "service_account",
			"project_id":     deterministicFirebaseProjectID,
			"private_key_id": "synthetic-private-key",
			"private_key": string(
				pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateKey}),
			),
			"client_email":                "synthetic@portable-auth-test.iam.gserviceaccount.com",
			"client_id":                   "123456789",
			"auth_uri":                    "https://accounts.google.com/o/oauth2/auth",
			"token_uri":                   "https://oauth2.googleapis.com/token",
			"auth_provider_x509_cert_url": "https://www.googleapis.com/oauth2/v1/certs",
		},
	)
	if err != nil {
		t.Fatalf("marshal synthetic service-account document: %v", err)
	}
	path := filepath.Join(t.TempDir(), "synthetic-service-account.json")
	if err := os.WriteFile(path, document, 0o600); err != nil {
		t.Fatalf("write synthetic service-account document: %v", err)
	}
	return path
}

func newDeterministicFirebaseVerifier(t *testing.T) *FirebaseVerifier {
	t.Helper()
	app, err := firebase.NewApp(t.Context(), &firebase.Config{ProjectID: deterministicFirebaseProjectID})
	if err != nil {
		t.Fatalf("create deterministic Firebase app: %v", err)
	}
	client, err := app.Auth(t.Context())
	if err != nil {
		t.Fatalf("create deterministic Firebase Auth client: %v", err)
	}
	return NewFirebaseVerifier(client)
}

func deterministicFirebaseToken(
	t *testing.T,
	key *rsa.PrivateKey,
	algorithm string,
	keyID string,
	now int64,
	overrides map[string]any,
	corrupt bool,
) string {
	t.Helper()
	header := map[string]any{"alg": algorithm, "kid": keyID, "typ": "JWT"}
	payload := map[string]any{
		"aud": deterministicFirebaseProjectID,
		"iss": "https://securetoken.google.com/" + deterministicFirebaseProjectID,
		"sub": "principal-a", "iat": now - 60, "exp": now + 3600, "auth_time": now - 60,
	}
	maps.Copy(payload, overrides)
	encodedHeader := encodeFirebaseJWTPart(t, header)
	encodedPayload := encodeFirebaseJWTPart(t, payload)
	signingInput := encodedHeader + "." + encodedPayload
	var signature []byte
	if algorithm == "RS256" {
		digest := sha256.Sum256([]byte(signingInput))
		var err error
		signature, err = rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
		if err != nil {
			t.Fatalf("sign deterministic Firebase token: %v", err)
		}
		if corrupt {
			signature[0] ^= 0xff
		}
	} else if algorithm != "none" {
		signature = []byte("not-an-rs256-signature")
	}
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(signature)
}

func encodeFirebaseJWTPart(t *testing.T, value any) string {
	t.Helper()
	document, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal deterministic Firebase JWT: %v", err)
	}
	return base64.RawURLEncoding.EncodeToString(document)
}

func TestFirebaseVerifierProductionSecurityBoundaryDeterministically(t *testing.T) {
	fixture := newDeterministicFirebaseFixture(t)
	t.Setenv("FIREBASE_AUTH_EMULATOR_HOST", "")
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", fixture.credentialsPath(t))
	verifier := newDeterministicFirebaseVerifier(t)
	now := time.Now().Unix()
	tests := []struct {
		name, algorithm, keyID string
		overrides              map[string]any
		corrupt                bool
		want                   error
	}{
		{name: "none algorithm", algorithm: "none", keyID: "known-key", want: ErrInvalidToken},
		{name: "symmetric algorithm", algorithm: "HS256", keyID: "known-key", want: ErrInvalidToken},
		{name: "missing key ID", algorithm: "RS256", want: ErrInvalidToken},
		{
			name: "wrong audience", algorithm: "RS256", keyID: "known-key",
			overrides: map[string]any{"aud": "another-project"}, want: ErrInvalidToken,
		},
		{
			name: "wrong issuer", algorithm: "RS256", keyID: "known-key",
			overrides: map[string]any{"iss": "https://securetoken.google.com/another-project"}, want: ErrInvalidToken,
		},
		{
			name: "expired", algorithm: "RS256", keyID: "known-key",
			overrides: map[string]any{"iat": now - 7200, "exp": now - 600}, want: ErrTokenExpired,
		},
		{
			name: "empty subject", algorithm: "RS256", keyID: "known-key",
			overrides: map[string]any{"sub": ""}, want: ErrInvalidToken,
		},
		{
			name: "oversized subject", algorithm: "RS256", keyID: "known-key",
			overrides: map[string]any{"sub": strings.Repeat("x", 129)}, want: ErrInvalidToken,
		},
		{name: "unknown key", algorithm: "RS256", keyID: "unknown-key", want: ErrInvalidToken},
		{
			name: "corrupted known-key signature", algorithm: "RS256", keyID: "known-key",
			corrupt: true, want: ErrInvalidToken,
		},
	}
	portable.ConfigureHuma()
	router := setupTestAPI(verifier, true)
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			token := deterministicFirebaseToken(
				t, fixture.signingKey, test.algorithm, test.keyID, now, test.overrides, test.corrupt,
			)
			if _, err := verifier.Verify(t.Context(), token); !errors.Is(err, test.want) {
				t.Fatalf("verification error=%v want=%v", err, test.want)
			}
			request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", nil)
			request.Header.Set("Authorization", "Bearer "+token)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			var problem struct {
				Code string `json:"code"`
			}
			if unmarshalErr := json.Unmarshal(
				response.Body.Bytes(),
				&problem,
			); response.Code != http.StatusUnauthorized ||
				response.Header().Get("WWW-Authenticate") != "Bearer" || unmarshalErr != nil ||
				problem.Code != "unauthorized" {
				t.Fatalf(
					"status=%d challenge=%q problem=%#v err=%v body=%s",
					response.Code,
					response.Header().Get("WWW-Authenticate"),
					problem,
					unmarshalErr,
					response.Body.String(),
				)
			}
		})
	}
	certificateCalls, identityCalls := fixture.callCounts()
	if certificateCalls == 0 || identityCalls != 0 {
		t.Fatalf("certificate calls=%d identity calls=%d", certificateCalls, identityCalls)
	}

	validToken := deterministicFirebaseToken(t, fixture.signingKey, "RS256", "known-key", now, nil, false)
	fixture.setIdentityDocument(`{"users":[{"localId":"principal-a","validSince":"0","disabled":false}]}`)
	user, err := verifier.Verify(t.Context(), validToken)
	if err != nil || user == nil || user.UID != "principal-a" {
		t.Fatalf("valid user=%#v err=%v", user, err)
	}
	fixture.setIdentityDocument(`{"users":[{"localId":"principal-a","validSince":"0","disabled":true}]}`)
	if _, err := verifier.Verify(t.Context(), validToken); !errors.Is(err, ErrUserDisabled) {
		t.Fatalf("disabled user error=%v", err)
	}
	fixture.setIdentityDocument(fmt.Sprintf(
		`{"users":[{"localId":"principal-a","validSince":%q,"disabled":false}]}`,
		strconv.FormatInt(now, 10),
	))
	if _, err := verifier.Verify(t.Context(), validToken); !errors.Is(err, ErrTokenRevoked) {
		t.Fatalf("revoked token error=%v", err)
	}
	fixture.setIdentityDocument(`{"users":[]}`)
	if _, err := verifier.Verify(t.Context(), validToken); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("deleted user error=%v", err)
	}
	fixture.setIdentityDocument(`{"users":`)
	if _, err := verifier.Verify(t.Context(), validToken); !errors.Is(err, ErrAuthUnavailable) {
		t.Fatalf("revocation dependency error=%v", err)
	}

	_, identityCallsBeforeOutage := fixture.callCounts()
	fixture.setCertificateStatus(http.StatusServiceUnavailable)
	outageVerifier := newDeterministicFirebaseVerifier(t)
	if _, err := outageVerifier.Verify(t.Context(), validToken); !errors.Is(err, ErrCertificateFetch) ||
		!errors.Is(err, ErrAuthUnavailable) {
		t.Fatalf("certificate outage error=%v", err)
	}
	_, identityCallsAfterOutage := fixture.callCounts()
	if identityCallsAfterOutage != identityCallsBeforeOutage {
		t.Fatalf("certificate outage reached identity service: before=%d after=%d",
			identityCallsBeforeOutage, identityCallsAfterOutage)
	}
}
