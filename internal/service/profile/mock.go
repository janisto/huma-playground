package profile

import (
	"context"
	"strings"
	"sync"
	"time"
)

// MockProfileService implements Service for unit tests.
type MockProfileService struct {
	mu       sync.RWMutex
	profiles map[string]*Profile
}

// NewMockProfileService creates a new mock service.
func NewMockProfileService() *MockProfileService {
	return &MockProfileService{
		profiles: make(map[string]*Profile),
	}
}

func (m *MockProfileService) Create(ctx context.Context, userID string, params CreateParams) (*Profile, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.profiles[userID]; exists {
		return nil, ErrAlreadyExists
	}

	now := time.Now().UTC()
	p := &Profile{
		ID:          userID,
		Firstname:   params.Firstname,
		Lastname:    params.Lastname,
		Email:       strings.ToLower(strings.TrimSpace(params.Email)),
		PhoneNumber: strings.TrimSpace(params.PhoneNumber),
		Marketing:   params.Marketing,
		Terms:       params.Terms,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	m.profiles[userID] = p
	return p, nil
}

func (m *MockProfileService) Get(ctx context.Context, userID string) (*Profile, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	p, exists := m.profiles[userID]
	if !exists {
		return nil, ErrNotFound
	}
	return p, nil
}

func (m *MockProfileService) Update(ctx context.Context, userID string, params UpdateParams) (*Profile, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	p, exists := m.profiles[userID]
	if !exists {
		return nil, ErrNotFound
	}

	if params.Firstname != nil {
		p.Firstname = *params.Firstname
	}
	if params.Lastname != nil {
		p.Lastname = *params.Lastname
	}
	if params.Email != nil {
		p.Email = strings.ToLower(strings.TrimSpace(*params.Email))
	}
	if params.PhoneNumber != nil {
		p.PhoneNumber = strings.TrimSpace(*params.PhoneNumber)
	}
	if params.Marketing != nil {
		p.Marketing = *params.Marketing
	}
	p.UpdatedAt = time.Now().UTC()
	return p, nil
}

func (m *MockProfileService) Delete(ctx context.Context, userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.profiles[userID]; !exists {
		return ErrNotFound
	}
	delete(m.profiles, userID)
	return nil
}

// Clear removes all profiles (useful for test cleanup).
func (m *MockProfileService) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.profiles = make(map[string]*Profile)
}

// Compile-time interface check
var _ Service = (*MockProfileService)(nil)
