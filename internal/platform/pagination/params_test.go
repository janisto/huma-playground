package pagination

import "testing"

func TestDefaultLimitZero(t *testing.T) {
	p := Params{Limit: 0}
	if got := p.DefaultLimit(); got != 20 {
		t.Fatalf("expected 20 for zero limit, got %d", got)
	}
}

func TestDefaultLimitNegative(t *testing.T) {
	p := Params{Limit: -5}
	if got := p.DefaultLimit(); got != 20 {
		t.Fatalf("expected 20 for negative limit, got %d", got)
	}
}

func TestDefaultLimitOne(t *testing.T) {
	p := Params{Limit: 1}
	if got := p.DefaultLimit(); got != 1 {
		t.Fatalf("expected 1, got %d", got)
	}
}

func TestDefaultLimitMax(t *testing.T) {
	p := Params{Limit: 100}
	if got := p.DefaultLimit(); got != 100 {
		t.Fatalf("expected 100, got %d", got)
	}
}

func TestDefaultLimitCustom(t *testing.T) {
	p := Params{Limit: 50}
	if got := p.DefaultLimit(); got != 50 {
		t.Fatalf("expected 50, got %d", got)
	}
}

func TestParamsDefaults(t *testing.T) {
	p := Params{}
	if p.Cursor != "" {
		t.Fatalf("expected empty cursor, got %q", p.Cursor)
	}
	if p.Limit != 0 {
		t.Fatalf("expected zero limit, got %d", p.Limit)
	}
}
