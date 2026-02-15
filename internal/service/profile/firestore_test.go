package profile

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"cloud.google.com/go/firestore"

	"github.com/janisto/huma-playground/internal/testutil"
)

func setupFirestoreTest(t *testing.T) (*FirestoreStore, func()) {
	t.Helper()

	testutil.SkipIfFirestoreUnavailable(t)
	testutil.SetupEmulator(t)
	testutil.ClearEmulators(t)

	ctx := context.Background()
	client, err := firestore.NewClient(ctx, testutil.ProjectID)
	if err != nil {
		t.Fatalf("failed to create Firestore client: %v", err)
	}

	store := NewFirestoreStore(client)
	cleanup := func() {
		testutil.ClearFirestore(t)
		_ = client.Close()
	}

	return store, cleanup
}

func TestFirestoreCreate(t *testing.T) {
	store, cleanup := setupFirestoreTest(t)
	defer cleanup()

	ctx := context.Background()
	params := CreateParams{
		Firstname:   "John",
		Lastname:    "Doe",
		Email:       "JOHN@EXAMPLE.COM",
		PhoneNumber: "+358401234567",
		Marketing:   true,
		Terms:       true,
	}

	p, err := store.Create(ctx, "user-123", params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if p.ID != "user-123" {
		t.Errorf("expected ID user-123, got %s", p.ID)
	}
	if p.Firstname != "John" {
		t.Errorf("expected firstname John, got %s", p.Firstname)
	}
	if p.Lastname != "Doe" {
		t.Errorf("expected lastname Doe, got %s", p.Lastname)
	}
	if p.Email != "john@example.com" {
		t.Errorf("expected email to be lowercased, got %s", p.Email)
	}
	if p.PhoneNumber != "+358401234567" {
		t.Errorf("expected phone +358401234567, got %s", p.PhoneNumber)
	}
	if !p.Marketing {
		t.Error("expected marketing true")
	}
	if !p.Terms {
		t.Error("expected terms true")
	}
	if p.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}
	if p.UpdatedAt.IsZero() {
		t.Error("expected UpdatedAt to be set")
	}
}

func TestFirestoreCreateDuplicate(t *testing.T) {
	store, cleanup := setupFirestoreTest(t)
	defer cleanup()

	ctx := context.Background()
	params := CreateParams{
		Firstname: "John",
		Lastname:  "Doe",
		Email:     "john@example.com",
		Terms:     true,
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

	ctx := context.Background()
	params := CreateParams{
		Firstname:   "Jane",
		Lastname:    "Smith",
		Email:       "jane@example.com",
		PhoneNumber: "+358409876543",
		Marketing:   false,
		Terms:       true,
	}
	_, _ = store.Create(ctx, "user-get", params)

	p, err := store.Get(ctx, "user-get")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if p.ID != "user-get" {
		t.Errorf("expected ID user-get, got %s", p.ID)
	}
	if p.Firstname != "Jane" {
		t.Errorf("expected firstname Jane, got %s", p.Firstname)
	}
	if p.Lastname != "Smith" {
		t.Errorf("expected lastname Smith, got %s", p.Lastname)
	}
}

func TestFirestoreGetNotFound(t *testing.T) {
	store, cleanup := setupFirestoreTest(t)
	defer cleanup()

	ctx := context.Background()

	_, err := store.Get(ctx, "nonexistent")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestFirestoreUpdatePartial(t *testing.T) {
	store, cleanup := setupFirestoreTest(t)
	defer cleanup()

	ctx := context.Background()
	params := CreateParams{
		Firstname:   "John",
		Lastname:    "Doe",
		Email:       "john@example.com",
		PhoneNumber: "+358401234567",
		Marketing:   false,
		Terms:       true,
	}
	created, _ := store.Create(ctx, "user-update", params)

	time.Sleep(10 * time.Millisecond)

	newFirstname := "Johnny"
	newMarketing := true
	updated, err := store.Update(ctx, "user-update", UpdateParams{
		Firstname: &newFirstname,
		Marketing: &newMarketing,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if updated.Firstname != "Johnny" {
		t.Errorf("expected firstname Johnny, got %s", updated.Firstname)
	}
	if updated.Lastname != "Doe" {
		t.Errorf("expected lastname Doe (unchanged), got %s", updated.Lastname)
	}
	if updated.Email != "john@example.com" {
		t.Errorf("expected email unchanged, got %s", updated.Email)
	}
	if !updated.Marketing {
		t.Error("expected marketing to be updated to true")
	}
	if !updated.UpdatedAt.After(created.CreatedAt) {
		t.Error("expected UpdatedAt to be after CreatedAt")
	}
}

func TestFirestoreUpdateEmailNormalization(t *testing.T) {
	store, cleanup := setupFirestoreTest(t)
	defer cleanup()

	ctx := context.Background()
	params := CreateParams{
		Firstname: "Test",
		Lastname:  "User",
		Email:     "test@example.com",
		Terms:     true,
	}
	_, _ = store.Create(ctx, "user-email", params)

	newEmail := "  UPDATED@EXAMPLE.COM  "
	updated, err := store.Update(ctx, "user-email", UpdateParams{
		Email: &newEmail,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if updated.Email != "updated@example.com" {
		t.Errorf("expected email to be normalized, got %s", updated.Email)
	}
}

func TestFirestoreUpdateAllFields(t *testing.T) {
	store, cleanup := setupFirestoreTest(t)
	defer cleanup()

	ctx := context.Background()
	params := CreateParams{
		Firstname:   "John",
		Lastname:    "Doe",
		Email:       "john@example.com",
		PhoneNumber: "+358401234567",
		Marketing:   false,
		Terms:       true,
	}
	_, _ = store.Create(ctx, "user-all", params)

	newFirstname := "Jane"
	newLastname := "Smith"
	newEmail := "jane@example.com"
	newPhone := "+358409876543"
	newMarketing := true

	updated, err := store.Update(ctx, "user-all", UpdateParams{
		Firstname:   &newFirstname,
		Lastname:    &newLastname,
		Email:       &newEmail,
		PhoneNumber: &newPhone,
		Marketing:   &newMarketing,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if updated.Firstname != "Jane" {
		t.Errorf("expected firstname Jane, got %s", updated.Firstname)
	}
	if updated.Lastname != "Smith" {
		t.Errorf("expected lastname Smith, got %s", updated.Lastname)
	}
	if updated.Email != "jane@example.com" {
		t.Errorf("expected email jane@example.com, got %s", updated.Email)
	}
	if updated.PhoneNumber != "+358409876543" {
		t.Errorf("expected phone +358409876543, got %s", updated.PhoneNumber)
	}
	if !updated.Marketing {
		t.Error("expected marketing true")
	}
	if !updated.Terms {
		t.Error("expected terms to remain true")
	}
}

func TestFirestoreUpdateNotFound(t *testing.T) {
	store, cleanup := setupFirestoreTest(t)
	defer cleanup()

	ctx := context.Background()

	newName := "Test"
	_, err := store.Update(ctx, "nonexistent", UpdateParams{Firstname: &newName})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestFirestoreDelete(t *testing.T) {
	store, cleanup := setupFirestoreTest(t)
	defer cleanup()

	ctx := context.Background()
	params := CreateParams{
		Firstname: "Delete",
		Lastname:  "Me",
		Email:     "delete@example.com",
		Terms:     true,
	}
	_, _ = store.Create(ctx, "user-delete", params)

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

	ctx := context.Background()

	err := store.Delete(ctx, "nonexistent")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestFirestoreDeleteTwice(t *testing.T) {
	store, cleanup := setupFirestoreTest(t)
	defer cleanup()

	ctx := context.Background()
	params := CreateParams{
		Firstname: "Delete",
		Lastname:  "Twice",
		Email:     "twice@example.com",
		Terms:     true,
	}
	_, _ = store.Create(ctx, "user-twice", params)

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

	ctx := context.Background()
	const numGoroutines = 10
	results := make(chan error, numGoroutines)

	var wg sync.WaitGroup
	for range numGoroutines {
		wg.Go(func() {
			_, err := store.Create(ctx, "concurrent-user", CreateParams{
				Firstname: "Test",
				Lastname:  "User",
				Email:     "test@example.com",
				Terms:     true,
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

	ctx := context.Background()
	_, _ = store.Create(ctx, "delete-concurrent", CreateParams{
		Firstname: "Delete",
		Lastname:  "Concurrent",
		Email:     "concurrent@example.com",
		Terms:     true,
	})

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
	var _ Service = (*FirestoreStore)(nil)
}

func TestCategorizeError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{"already exists", ErrAlreadyExists, "already_exists"},
		{"not found", ErrNotFound, "not_found"},
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

func TestNewFirestoreStore(t *testing.T) {
	testutil.SkipIfFirestoreUnavailable(t)
	testutil.SetupEmulator(t)

	ctx := context.Background()
	client, err := firestore.NewClient(ctx, testutil.ProjectID)
	if err != nil {
		t.Fatalf("failed to create Firestore client: %v", err)
	}
	defer func() { _ = client.Close() }()

	store := NewFirestoreStore(client)
	if store == nil {
		t.Fatal("expected non-nil store")
	}
	if store.client != client {
		t.Fatal("expected store.client to be the provided client")
	}
}

func TestFirestoreGetCancelledContext(t *testing.T) {
	store, cleanup := setupFirestoreTest(t)
	defer cleanup()

	ctx, cancel := context.WithCancel(context.Background())
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

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := store.Create(ctx, "user-canceled", CreateParams{
		Firstname: "Test",
		Lastname:  "User",
		Email:     "test@example.com",
		Terms:     true,
	})
	if err == nil {
		t.Fatal("expected error with canceled context")
	}
}

func TestFirestoreUpdateCancelledContext(t *testing.T) {
	store, cleanup := setupFirestoreTest(t)
	defer cleanup()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	newName := "Test"
	_, err := store.Update(ctx, "user-canceled", UpdateParams{Firstname: &newName})
	if err == nil {
		t.Fatal("expected error with canceled context")
	}
}

func TestFirestoreDeleteCancelledContext(t *testing.T) {
	store, cleanup := setupFirestoreTest(t)
	defer cleanup()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := store.Delete(ctx, "user-canceled")
	if err == nil {
		t.Fatal("expected error with canceled context")
	}
}
