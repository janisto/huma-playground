package profile

import (
	"context"
	"errors"
	"sync"
	"testing"
)

func TestMockCreate(t *testing.T) {
	svc := NewMockProfileService()
	ctx := context.Background()

	params := CreateParams{
		Firstname:   "John",
		Lastname:    "Doe",
		Email:       "john@example.com",
		PhoneNumber: "+358401234567",
		Marketing:   true,
		Terms:       true,
	}

	p, err := svc.Create(ctx, "user-123", params)
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
		t.Errorf("expected email john@example.com, got %s", p.Email)
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

func TestMockCreateDuplicate(t *testing.T) {
	svc := NewMockProfileService()
	ctx := context.Background()

	params := CreateParams{Firstname: "John", Lastname: "Doe", Email: "john@example.com", Terms: true}

	_, err := svc.Create(ctx, "user-123", params)
	if err != nil {
		t.Fatalf("first create failed: %v", err)
	}

	_, err = svc.Create(ctx, "user-123", params)
	if !errors.Is(err, ErrAlreadyExists) {
		t.Fatalf("expected ErrAlreadyExists, got %v", err)
	}
}

func TestMockGet(t *testing.T) {
	svc := NewMockProfileService()
	ctx := context.Background()

	params := CreateParams{Firstname: "Jane", Lastname: "Smith", Email: "jane@example.com", Terms: true}
	_, _ = svc.Create(ctx, "user-456", params)

	p, err := svc.Get(ctx, "user-456")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Firstname != "Jane" {
		t.Errorf("expected firstname Jane, got %s", p.Firstname)
	}
}

func TestMockGetNotFound(t *testing.T) {
	svc := NewMockProfileService()
	ctx := context.Background()

	_, err := svc.Get(ctx, "nonexistent")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestMockUpdatePartial(t *testing.T) {
	svc := NewMockProfileService()
	ctx := context.Background()

	params := CreateParams{
		Firstname:   "John",
		Lastname:    "Doe",
		Email:       "john@example.com",
		PhoneNumber: "+358401234567",
		Marketing:   false,
		Terms:       true,
	}
	_, _ = svc.Create(ctx, "user-789", params)

	newFirstname := "Johnny"
	newMarketing := true
	updated, err := svc.Update(ctx, "user-789", UpdateParams{
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
}

func TestMockUpdateAllFields(t *testing.T) {
	svc := NewMockProfileService()
	ctx := context.Background()

	params := CreateParams{
		Firstname:   "John",
		Lastname:    "Doe",
		Email:       "john@example.com",
		PhoneNumber: "+358401234567",
		Marketing:   false,
		Terms:       true,
	}
	created, _ := svc.Create(ctx, "user-all", params)

	newFirstname := "Jane"
	newLastname := "Smith"
	newEmail := "jane@example.com"
	newPhone := "+358409876543"
	newMarketing := true

	updated, err := svc.Update(ctx, "user-all", UpdateParams{
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
	if updated.UpdatedAt.Before(created.CreatedAt) {
		t.Error("expected UpdatedAt to be at or after creation time")
	}
}

func TestMockUpdateNotFound(t *testing.T) {
	svc := NewMockProfileService()
	ctx := context.Background()

	newName := "Test"
	_, err := svc.Update(ctx, "nonexistent", UpdateParams{Firstname: &newName})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestMockDelete(t *testing.T) {
	svc := NewMockProfileService()
	ctx := context.Background()

	params := CreateParams{Firstname: "Delete", Lastname: "Me", Email: "delete@example.com", Terms: true}
	_, _ = svc.Create(ctx, "user-delete", params)

	err := svc.Delete(ctx, "user-delete")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = svc.Get(ctx, "user-delete")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected profile to be deleted, got %v", err)
	}
}

func TestMockDeleteNotFound(t *testing.T) {
	svc := NewMockProfileService()
	ctx := context.Background()

	err := svc.Delete(ctx, "nonexistent")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestMockDeleteTwice(t *testing.T) {
	svc := NewMockProfileService()
	ctx := context.Background()

	params := CreateParams{Firstname: "Delete", Lastname: "Twice", Email: "twice@example.com", Terms: true}
	_, _ = svc.Create(ctx, "user-twice", params)

	err := svc.Delete(ctx, "user-twice")
	if err != nil {
		t.Fatalf("first delete failed: %v", err)
	}

	err = svc.Delete(ctx, "user-twice")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound on second delete, got %v", err)
	}
}

func TestMockClear(t *testing.T) {
	svc := NewMockProfileService()
	ctx := context.Background()

	_, _ = svc.Create(ctx, "user-1", CreateParams{Firstname: "One", Terms: true})
	_, _ = svc.Create(ctx, "user-2", CreateParams{Firstname: "Two", Terms: true})

	svc.Clear()

	_, err := svc.Get(ctx, "user-1")
	if !errors.Is(err, ErrNotFound) {
		t.Error("expected user-1 to be cleared")
	}
	_, err = svc.Get(ctx, "user-2")
	if !errors.Is(err, ErrNotFound) {
		t.Error("expected user-2 to be cleared")
	}
}

func TestMockConcurrentAccess(t *testing.T) {
	svc := NewMockProfileService()
	ctx := context.Background()

	const numGoroutines = 50
	var wg sync.WaitGroup

	for i := range numGoroutines {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			userID := "concurrent-user"

			switch id % 4 {
			case 0:
				_, _ = svc.Create(ctx, userID, CreateParams{Firstname: "Test", Terms: true})
			case 1:
				_, _ = svc.Get(ctx, userID)
			case 2:
				name := "Updated"
				_, _ = svc.Update(ctx, userID, UpdateParams{Firstname: &name})
			case 3:
				_ = svc.Delete(ctx, userID)
			}
		}(i)
	}

	wg.Wait()
}

func TestMockInterfaceCompliance(t *testing.T) {
	var _ Service = (*MockProfileService)(nil)
}

func TestMockCreateEmailNormalization(t *testing.T) {
	svc := NewMockProfileService()
	ctx := context.Background()

	p, err := svc.Create(ctx, "user-norm", CreateParams{
		Firstname: "Test",
		Email:     "  UPPER@EXAMPLE.COM  ",
		Terms:     true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Email != "upper@example.com" {
		t.Fatalf("expected normalized email, got %q", p.Email)
	}
}

func TestMockCreatePhoneNormalization(t *testing.T) {
	svc := NewMockProfileService()
	ctx := context.Background()

	p, err := svc.Create(ctx, "user-phone", CreateParams{
		Firstname:   "Test",
		PhoneNumber: "  +358401234567  ",
		Terms:       true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.PhoneNumber != "+358401234567" {
		t.Fatalf("expected trimmed phone, got %q", p.PhoneNumber)
	}
}

func TestMockUpdateEmailNormalization(t *testing.T) {
	svc := NewMockProfileService()
	ctx := context.Background()

	_, _ = svc.Create(ctx, "user-upd-email", CreateParams{
		Firstname: "Test",
		Email:     "test@example.com",
		Terms:     true,
	})

	newEmail := "  UPDATED@EXAMPLE.COM  "
	updated, err := svc.Update(ctx, "user-upd-email", UpdateParams{Email: &newEmail})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.Email != "updated@example.com" {
		t.Fatalf("expected normalized email, got %q", updated.Email)
	}
}

func TestMockUpdatePhoneNormalization(t *testing.T) {
	svc := NewMockProfileService()
	ctx := context.Background()

	_, _ = svc.Create(ctx, "user-upd-phone", CreateParams{
		Firstname:   "Test",
		PhoneNumber: "+358401234567",
		Terms:       true,
	})

	newPhone := "  +358409876543  "
	updated, err := svc.Update(ctx, "user-upd-phone", UpdateParams{PhoneNumber: &newPhone})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.PhoneNumber != "+358409876543" {
		t.Fatalf("expected trimmed phone, got %q", updated.PhoneNumber)
	}
}
