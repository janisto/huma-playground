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
	profile, err := store.Create(ctx, userID, params)
	if err != nil {
		t.Fatalf("create profile %q: %v", userID, err)
	}
	return profile
}

func TestFirestoreCreate(t *testing.T) {
	store, cleanup := setupFirestoreTest(t)
	defer cleanup()

	ctx := t.Context()
	params := CreateParams{
		FirstName:    "John",
		LastName:     "Doe",
		ContactEmail: "JOHN@EXAMPLE.COM",
		PhoneNumber:  "+358401234567",
		Marketing:    true,
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
	if p.ContactEmail != "JOHN@EXAMPLE.COM" {
		t.Errorf("expected contactEmail to be preserved, got %s", p.ContactEmail)
	}
	if p.PhoneNumber != "+358401234567" {
		t.Errorf("expected phone +358401234567, got %s", p.PhoneNumber)
	}
	if !p.Marketing {
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

func TestFirestoreCreateDuplicate(t *testing.T) {
	store, cleanup := setupFirestoreTest(t)
	defer cleanup()

	ctx := t.Context()
	params := CreateParams{
		FirstName:    "John",
		LastName:     "Doe",
		ContactEmail: "john@example.com",
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
		FirstName:    "Jane",
		LastName:     "Smith",
		ContactEmail: "jane@example.com",
		PhoneNumber:  "+358409876543",
		Marketing:    false,
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
		FirstName:    "John",
		LastName:     "Doe",
		ContactEmail: "john@example.com",
		PhoneNumber:  "+358401234567",
		Marketing:    false,
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
	newMarketing := true
	updated, err := store.Update(ctx, "user-update", UpdateParams{
		FirstName: &newFirstName,
		Marketing: &newMarketing,
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
	if !updated.Marketing {
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

func TestFirestoreUpdatePreservesContactEmail(t *testing.T) {
	store, cleanup := setupFirestoreTest(t)
	defer cleanup()

	ctx := t.Context()
	params := CreateParams{
		FirstName:    "Test",
		LastName:     "User",
		ContactEmail: "test@example.com",
	}
	createTestProfile(t, store, ctx, "user-contactEmail", params)

	newEmail := "  UPDATED@EXAMPLE.COM  "
	updated, err := store.Update(ctx, "user-contactEmail", UpdateParams{
		ContactEmail: &newEmail,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if updated.ContactEmail != "  UPDATED@EXAMPLE.COM  " {
		t.Errorf("expected contactEmail to be preserved, got %s", updated.ContactEmail)
	}
}

func TestFirestoreUpdateAllFields(t *testing.T) {
	store, cleanup := setupFirestoreTest(t)
	defer cleanup()

	ctx := t.Context()
	params := CreateParams{
		FirstName:    "John",
		LastName:     "Doe",
		ContactEmail: "john@example.com",
		PhoneNumber:  "+358401234567",
		Marketing:    false,
	}
	createTestProfile(t, store, ctx, "user-all", params)

	newFirstName := "Jane"
	newLastName := "Smith"
	newEmail := "jane@example.com"
	newPhone := "+358409876543"
	newMarketing := true

	updated, err := store.Update(ctx, "user-all", UpdateParams{
		FirstName:    &newFirstName,
		LastName:     &newLastName,
		ContactEmail: &newEmail,
		PhoneNumber:  &newPhone,
		Marketing:    &newMarketing,
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
	if !updated.Marketing {
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
			_, err := store.Create(ctx, "concurrent-user", CreateParams{
				FirstName:    "Test",
				LastName:     "User",
				ContactEmail: "test@example.com",
			})
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
	_, err := store.Create(ctx, "delete-concurrent", CreateParams{
		FirstName:    "Delete",
		LastName:     "Concurrent",
		ContactEmail: "concurrent@example.com",
	})
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

	_, err := store.Create(ctx, "user-canceled", CreateParams{
		FirstName:    "Test",
		LastName:     "User",
		ContactEmail: "test@example.com",
	})
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
