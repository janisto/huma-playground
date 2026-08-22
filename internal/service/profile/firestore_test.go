package profile

import (
	"context"
	"errors"
	"sync"
	"testing"

	"cloud.google.com/go/firestore"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/janisto/huma-playground/internal/testutil"
)

func setupFirestoreTest(t *testing.T) (*FirestoreStore, func()) {
	t.Helper()

	testutil.SkipIfEmulatorUnavailable(t)
	testutil.SetupEmulator(t)
	testutil.ClearFirestore(t)

	ctx := t.Context()
	client, err := firestore.NewClient(ctx, testutil.ProjectID)
	if err != nil {
		t.Fatalf("failed to create Firestore client: %v", err)
	}

	store := NewFirestoreStore(client)
	cleanup := func() {
		testutil.ClearFirestore(t)
		if err := client.Close(); err != nil {
			t.Errorf("close Firestore client: %v", err)
		}
	}

	return store, cleanup
}

func createTestProfile(
	t *testing.T,
	store *FirestoreStore,
	ctx context.Context,
	userID string,
	params CreateParams,
) *Profile {
	t.Helper()
	profile, err := store.Create(ctx, userID, completeCreateParams(params))
	if err != nil {
		t.Fatalf("create profile %q: %v", userID, err)
	}
	return profile
}

func completeCreateParams(params CreateParams) CreateParams {
	if params.PhoneNumber == "" {
		params.PhoneNumber = "+358401234567"
	}
	params.TermsAccepted = true
	return params
}

func TestFirestoreCreate(t *testing.T) {
	store, cleanup := setupFirestoreTest(t)
	defer cleanup()

	ctx := t.Context()
	params := CreateParams{
		FirstName:      "John",
		LastName:       "Doe",
		ContactEmail:   "JOHN@example.com",
		PhoneNumber:    "+358401234567",
		MarketingOptIn: true,
		TermsAccepted:  true,
	}

	p, err := store.Create(ctx, "user-123", params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if p.ID != "user-123" {
		t.Errorf("expected ID user-123, got %s", p.ID)
	}
	if p.FirstName != "John" {
		t.Errorf("expected firstName John, got %s", p.FirstName)
	}
	if p.LastName != "Doe" {
		t.Errorf("expected lastName Doe, got %s", p.LastName)
	}
	if p.ContactEmail != "JOHN@example.com" {
		t.Errorf("expected contactEmail to be preserved, got %s", p.ContactEmail)
	}
	if p.PhoneNumber != "+358401234567" {
		t.Errorf("expected phone +358401234567, got %s", p.PhoneNumber)
	}
	if !p.MarketingOptIn {
		t.Error("expected marketing true")
	}
	if p.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}
	if p.UpdatedAt.IsZero() {
		t.Error("expected UpdatedAt to be set")
	}
	document, err := store.client.Collection(profilesCollection).Doc("user-123").Get(ctx)
	if err != nil {
		t.Fatalf("read raw document: %v", err)
	}
	data := document.Data()
	for _, key := range []string{
		"first_name",
		"last_name",
		"contact_email",
		"phone_number",
		"marketing_opt_in",
		"terms_accepted",
		"created_at",
		"updated_at",
	} {
		if _, exists := data[key]; !exists {
			t.Errorf("expected Firestore field %q in %#v", key, data)
		}
	}
	for _, legacy := range []string{"firstname", "lastname", "email", "terms"} {
		if _, exists := data[legacy]; exists {
			t.Errorf("unexpected legacy Firestore field %q", legacy)
		}
	}
}

func TestProfileDocumentPathSeparatesEncodedAndLegacyIDs(t *testing.T) {
	tests := []struct {
		userID string
		want   string
	}{
		{userID: "user-123", want: "profiles/user-123"},
		{userID: "uid~Lw", want: "profiles/uid~Lw"},
		{userID: "user/name", want: "profiles/_encoded/by-uid/dXNlci9uYW1l"},
		{userID: "/", want: "profiles/_encoded/by-uid/Lw"},
		{userID: ".", want: "profiles/_encoded/by-uid/Lg"},
		{userID: "..", want: "profiles/_encoded/by-uid/Li4"},
		{userID: "__reserved__", want: "profiles/_encoded/by-uid/X19yZXNlcnZlZF9f"},
	}
	seen := make(map[string]string)
	for _, test := range tests {
		got := profileDocumentPath(test.userID)
		if got != test.want {
			t.Fatalf("profileDocumentPath(%q) = %q, want %q", test.userID, got, test.want)
		}
		if previous, exists := seen[got]; exists {
			t.Fatalf("document path collision for %q and %q: %q", previous, test.userID, got)
		}
		seen[got] = test.userID
	}
}

func TestFirestoreEncodedUIDCannotAccessLegacyProfile(t *testing.T) {
	store, cleanup := setupFirestoreTest(t)
	defer cleanup()

	ctx := t.Context()
	const legacyUserID = "uid~Lw"
	now := store.timestamp()
	_, err := store.client.Collection(profilesCollection).Doc(legacyUserID).Create(ctx, firestoreProfile{
		FirstName:     "Legacy",
		LastName:      "Owner",
		ContactEmail:  "legacy@example.com",
		PhoneNumber:   "+358401234567",
		TermsAccepted: true,
		CreatedAt:     now,
		UpdatedAt:     now,
	})
	if err != nil {
		t.Fatalf("create legacy profile: %v", err)
	}

	_, err = store.Get(ctx, "/")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("encoded UID read legacy profile: %v", err)
	}

	created := createTestProfile(t, store, ctx, "/", CreateParams{
		FirstName:    "Encoded",
		LastName:     "Owner",
		ContactEmail: "encoded@example.com",
	})
	if created.ContactEmail != "encoded@example.com" {
		t.Fatalf("encoded profile email = %q, want encoded@example.com", created.ContactEmail)
	}

	legacy, err := store.Get(ctx, legacyUserID)
	if err != nil {
		t.Fatalf("get legacy profile: %v", err)
	}
	if legacy.ContactEmail != "legacy@example.com" {
		t.Fatalf("legacy profile email = %q, want legacy@example.com", legacy.ContactEmail)
	}
}

func TestFirestoreSupportsFirebaseUIDOutsideDocumentIDGrammar(t *testing.T) {
	store, cleanup := setupFirestoreTest(t)
	defer cleanup()

	const userID = "firebase/user"
	params := completeCreateParams(CreateParams{FirstName: "Path", LastName: "Safe", ContactEmail: "path@example.com"})
	created := createTestProfile(t, store, t.Context(), userID, params)
	if created.ID != userID {
		t.Fatalf("created profile ID = %q, want %q", created.ID, userID)
	}
	got, err := store.Get(t.Context(), userID)
	if err != nil {
		t.Fatalf("get profile: %v", err)
	}
	if got.ID != userID {
		t.Fatalf("profile ID = %q, want %q", got.ID, userID)
	}
	if _, err := store.client.Doc(profileDocumentPath(userID)).Get(t.Context()); err != nil {
		t.Fatalf("read encoded profile document: %v", err)
	}
}

func TestFirestoreCreateDuplicate(t *testing.T) {
	store, cleanup := setupFirestoreTest(t)
	defer cleanup()

	ctx := t.Context()
	params := CreateParams{
		FirstName:     "John",
		LastName:      "Doe",
		ContactEmail:  "john@example.com",
		PhoneNumber:   "+358401234567",
		TermsAccepted: true,
	}

	_, err := store.Create(ctx, "user-dup", params)
	if err != nil {
		t.Fatalf("first create failed: %v", err)
	}

	_, err = store.Create(ctx, "user-dup", params)
	if !errors.Is(err, ErrAlreadyExists) {
		t.Fatalf("expected ErrAlreadyExists, got %v", err)
	}
}

func TestFirestoreGet(t *testing.T) {
	store, cleanup := setupFirestoreTest(t)
	defer cleanup()

	ctx := t.Context()
	params := CreateParams{
		FirstName:      "Jane",
		LastName:       "Smith",
		ContactEmail:   "jane@example.com",
		PhoneNumber:    "+358409876543",
		MarketingOptIn: false,
		TermsAccepted:  true,
	}
	createTestProfile(t, store, ctx, "user-get", params)

	p, err := store.Get(ctx, "user-get")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if p.ID != "user-get" {
		t.Errorf("expected ID user-get, got %s", p.ID)
	}
	if p.FirstName != "Jane" {
		t.Errorf("expected firstName Jane, got %s", p.FirstName)
	}
	if p.LastName != "Smith" {
		t.Errorf("expected lastName Smith, got %s", p.LastName)
	}
}

func TestFirestoreGetNotFound(t *testing.T) {
	store, cleanup := setupFirestoreTest(t)
	defer cleanup()

	ctx := t.Context()

	_, err := store.Get(ctx, "nonexistent")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestFirestoreUpdatePartial(t *testing.T) {
	store, cleanup := setupFirestoreTest(t)
	defer cleanup()

	ctx := t.Context()
	params := CreateParams{
		FirstName:      "John",
		LastName:       "Doe",
		ContactEmail:   "john@example.com",
		PhoneNumber:    "+358401234567",
		MarketingOptIn: false,
		TermsAccepted:  true,
	}
	created := createTestProfile(t, store, ctx, "user-update", params)
	if _, err := store.client.Collection(profilesCollection).Doc("user-update").Set(
		ctx,
		map[string]any{"future_field": "preserved"},
		firestore.MergeAll,
	); err != nil {
		t.Fatalf("add future field: %v", err)
	}

	newFirstName := "Johnny"
	newMarketingOptIn := true
	updated, err := store.Update(ctx, "user-update", UpdateParams{
		FirstName:      &newFirstName,
		MarketingOptIn: &newMarketingOptIn,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if updated.FirstName != "Johnny" {
		t.Errorf("expected firstName Johnny, got %s", updated.FirstName)
	}
	if updated.LastName != "Doe" {
		t.Errorf("expected lastName Doe (unchanged), got %s", updated.LastName)
	}
	if updated.ContactEmail != "john@example.com" {
		t.Errorf("expected contactEmail unchanged, got %s", updated.ContactEmail)
	}
	if !updated.MarketingOptIn {
		t.Error("expected marketing to be updated to true")
	}
	if !updated.UpdatedAt.After(created.CreatedAt) {
		t.Error("expected UpdatedAt to be after CreatedAt")
	}
	document, err := store.client.Collection(profilesCollection).Doc("user-update").Get(ctx)
	if err != nil {
		t.Fatalf("read updated document: %v", err)
	}
	if got := document.Data()["future_field"]; got != "preserved" {
		t.Fatalf("partial update removed future field: %v", got)
	}
}

func TestFirestoreUpdatePreservesCanonicalContactEmail(t *testing.T) {
	store, cleanup := setupFirestoreTest(t)
	defer cleanup()

	ctx := t.Context()
	params := CreateParams{
		FirstName:    "Test",
		LastName:     "User",
		ContactEmail: "test@example.com",
	}
	createTestProfile(t, store, ctx, "user-contactEmail", params)

	newEmail := "UPDATED@example.com"
	updated, err := store.Update(ctx, "user-contactEmail", UpdateParams{
		ContactEmail: &newEmail,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if updated.ContactEmail != "UPDATED@example.com" {
		t.Errorf("expected contactEmail to be preserved, got %s", updated.ContactEmail)
	}
}

func TestFirestoreUpdateRejectsNonCanonicalContactEmailBeforeFirestore(t *testing.T) {
	store := NewFirestoreStore(nil)
	nonCanonical := "  UPDATED@EXAMPLE.COM  "
	if _, err := store.Update(t.Context(), "user-contactEmail", UpdateParams{
		ContactEmail: &nonCanonical,
	}); !errors.Is(err, ErrInvalidStored) {
		t.Fatalf("error=%v want=%v", err, ErrInvalidStored)
	}
}

func TestFirestoreUpdateAllFields(t *testing.T) {
	store, cleanup := setupFirestoreTest(t)
	defer cleanup()

	ctx := t.Context()
	params := CreateParams{
		FirstName:      "John",
		LastName:       "Doe",
		ContactEmail:   "john@example.com",
		PhoneNumber:    "+358401234567",
		MarketingOptIn: false,
	}
	createTestProfile(t, store, ctx, "user-all", params)

	newFirstName := "Jane"
	newLastName := "Smith"
	newEmail := "jane@example.com"
	newPhone := "+358409876543"
	newMarketingOptIn := true

	updated, err := store.Update(ctx, "user-all", UpdateParams{
		FirstName:      &newFirstName,
		LastName:       &newLastName,
		ContactEmail:   &newEmail,
		PhoneNumber:    &newPhone,
		MarketingOptIn: &newMarketingOptIn,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if updated.FirstName != "Jane" {
		t.Errorf("expected firstName Jane, got %s", updated.FirstName)
	}
	if updated.LastName != "Smith" {
		t.Errorf("expected lastName Smith, got %s", updated.LastName)
	}
	if updated.ContactEmail != "jane@example.com" {
		t.Errorf("expected contactEmail jane@example.com, got %s", updated.ContactEmail)
	}
	if updated.PhoneNumber != "+358409876543" {
		t.Errorf("expected phone +358409876543, got %s", updated.PhoneNumber)
	}
	if !updated.MarketingOptIn {
		t.Error("expected marketing true")
	}
}

func TestFirestoreUpdateNotFound(t *testing.T) {
	store, cleanup := setupFirestoreTest(t)
	defer cleanup()

	ctx := t.Context()

	newName := "Test"
	_, err := store.Update(ctx, "nonexistent", UpdateParams{FirstName: &newName})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestFirestoreDelete(t *testing.T) {
	store, cleanup := setupFirestoreTest(t)
	defer cleanup()

	ctx := t.Context()
	params := CreateParams{
		FirstName:    "Delete",
		LastName:     "Me",
		ContactEmail: "delete@example.com",
	}
	createTestProfile(t, store, ctx, "user-delete", params)

	err := store.Delete(ctx, "user-delete")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = store.Get(ctx, "user-delete")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected profile to be deleted, got %v", err)
	}
}

func TestFirestoreDeleteNotFound(t *testing.T) {
	store, cleanup := setupFirestoreTest(t)
	defer cleanup()

	ctx := t.Context()

	err := store.Delete(ctx, "nonexistent")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestFirestoreDeleteTwice(t *testing.T) {
	store, cleanup := setupFirestoreTest(t)
	defer cleanup()

	ctx := t.Context()
	params := CreateParams{
		FirstName:    "Delete",
		LastName:     "Twice",
		ContactEmail: "twice@example.com",
	}
	createTestProfile(t, store, ctx, "user-twice", params)

	err := store.Delete(ctx, "user-twice")
	if err != nil {
		t.Fatalf("first delete failed: %v", err)
	}

	err = store.Delete(ctx, "user-twice")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound on second delete, got %v", err)
	}
}

func TestFirestoreConcurrentCreate(t *testing.T) {
	store, cleanup := setupFirestoreTest(t)
	defer cleanup()

	ctx := t.Context()
	const numGoroutines = 10
	results := make(chan error, numGoroutines)

	var wg sync.WaitGroup
	for range numGoroutines {
		wg.Go(func() {
			_, err := store.Create(ctx, "concurrent-user", completeCreateParams(CreateParams{
				FirstName:    "Test",
				LastName:     "User",
				ContactEmail: "test@example.com",
			}))
			results <- err
		})
	}
	wg.Wait()
	close(results)

	var success, alreadyExists int
	for err := range results {
		switch {
		case err == nil:
			success++
		case errors.Is(err, ErrAlreadyExists):
			alreadyExists++
		default:
			t.Errorf("unexpected error: %v", err)
		}
	}

	if success != 1 {
		t.Errorf("expected exactly 1 success, got %d", success)
	}
	if alreadyExists != numGoroutines-1 {
		t.Errorf("expected %d already exists, got %d", numGoroutines-1, alreadyExists)
	}
}

func TestFirestoreConcurrentDelete(t *testing.T) {
	store, cleanup := setupFirestoreTest(t)
	defer cleanup()

	ctx := t.Context()
	_, err := store.Create(ctx, "delete-concurrent", completeCreateParams(CreateParams{
		FirstName:    "Delete",
		LastName:     "Concurrent",
		ContactEmail: "concurrent@example.com",
	}))
	if err != nil {
		t.Fatalf("create profile: %v", err)
	}

	const numGoroutines = 10
	results := make(chan error, numGoroutines)

	var wg sync.WaitGroup
	for range numGoroutines {
		wg.Go(func() {
			results <- store.Delete(ctx, "delete-concurrent")
		})
	}
	wg.Wait()
	close(results)

	var success, notFound int
	for err := range results {
		switch {
		case err == nil:
			success++
		case errors.Is(err, ErrNotFound):
			notFound++
		default:
			t.Errorf("unexpected error: %v", err)
		}
	}

	if success != 1 {
		t.Errorf("expected exactly 1 success, got %d", success)
	}
	if notFound != numGoroutines-1 {
		t.Errorf("expected %d not found, got %d", numGoroutines-1, notFound)
	}
}

func TestFirestoreInterfaceCompliance(t *testing.T) {
	var _ Store = (*FirestoreStore)(nil)
}

func TestCategorizeError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{"already exists", ErrAlreadyExists, "already_exists"},
		{"not found", ErrNotFound, "not_found"},
		{"unavailable", ErrUnavailable, "unavailable"},
		{"internal error", errors.New("unexpected"), "internal_error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := categorizeError(tt.err)
			if got != tt.want {
				t.Fatalf("categorizeError(%v) = %q, want %q", tt.err, got, tt.want)
			}
		})
	}
}

func TestClassifyDependencyError(t *testing.T) {
	for _, code := range []codes.Code{
		codes.Aborted,
		codes.Canceled,
		codes.DeadlineExceeded,
		codes.ResourceExhausted,
		codes.Unavailable,
	} {
		t.Run(code.String(), func(t *testing.T) {
			err := classifyDependencyError(status.Error(code, "temporary"))
			if !errors.Is(err, ErrUnavailable) {
				t.Fatalf("expected ErrUnavailable, got %v", err)
			}
		})
	}
	if err := classifyDependencyError(context.DeadlineExceeded); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("expected deadline to be unavailable, got %v", err)
	}
	if err := classifyDependencyError(
		context.Canceled,
	); !errors.Is(err, context.Canceled) ||
		errors.Is(err, ErrUnavailable) {
		t.Fatalf("expected cancellation to remain cancellation, got %v", err)
	}
	if err := classifyDependencyError(status.Error(codes.InvalidArgument, "bad data")); errors.Is(err, ErrUnavailable) {
		t.Fatalf("did not expect invalid argument to be unavailable: %v", err)
	}
}

func TestNewFirestoreStore(t *testing.T) {
	testutil.SkipIfEmulatorUnavailable(t)
	testutil.SetupEmulator(t)

	ctx := t.Context()
	client, err := firestore.NewClient(ctx, testutil.ProjectID)
	if err != nil {
		t.Fatalf("failed to create Firestore client: %v", err)
	}
	defer func() {
		if err := client.Close(); err != nil {
			t.Errorf("close Firestore client: %v", err)
		}
	}()

	store := NewFirestoreStore(client)
	if store == nil {
		t.Fatal("expected non-nil store")
		return
	}
	if store.client != client {
		t.Fatal("expected store.client to be the provided client")
	}
}

func TestFirestoreGetCancelledContext(t *testing.T) {
	store, cleanup := setupFirestoreTest(t)
	defer cleanup()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	_, err := store.Get(ctx, "user-canceled")
	if err == nil {
		t.Fatal("expected error with canceled context")
	}
	if errors.Is(err, ErrNotFound) {
		t.Fatal("expected non-NotFound error, got ErrNotFound")
	}
}

func TestFirestoreCreateCancelledContext(t *testing.T) {
	store, cleanup := setupFirestoreTest(t)
	defer cleanup()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	_, err := store.Create(ctx, "user-canceled", completeCreateParams(CreateParams{
		FirstName:    "Test",
		LastName:     "User",
		ContactEmail: "test@example.com",
	}))
	if err == nil {
		t.Fatal("expected error with canceled context")
	}
}

func TestFirestoreUpdateCancelledContext(t *testing.T) {
	store, cleanup := setupFirestoreTest(t)
	defer cleanup()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	newName := "Test"
	_, err := store.Update(ctx, "user-canceled", UpdateParams{FirstName: &newName})
	if err == nil {
		t.Fatal("expected error with canceled context")
	}
}

func TestFirestoreDeleteCancelledContext(t *testing.T) {
	store, cleanup := setupFirestoreTest(t)
	defer cleanup()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	err := store.Delete(ctx, "user-canceled")
	if err == nil {
		t.Fatal("expected error with canceled context")
	}
}
