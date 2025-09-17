package api

import "testing"

func TestNewSuccessEnvelopeCopiesData(t *testing.T) {
	trace := "trace-123"
	input := struct{ Value string }{Value: "ok"}
	env := NewSuccessEnvelope(&trace, input)

	if env.Data == nil {
		t.Fatalf("expected Data pointer to be non-nil")
	}
	if got := env.Data.Value; got != "ok" {
		t.Fatalf("unexpected data value: %q", got)
	}
	if env.Meta.TraceID == nil || *env.Meta.TraceID != trace {
		t.Fatalf("expected traceId %q, got %+v", trace, env.Meta.TraceID)
	}

	input.Value = "mutated"
	if env.Data.Value != "ok" {
		t.Fatalf("data should not change after original input mutation, got %q", env.Data.Value)
	}
}

func TestNewErrorEnvelopeClonesDetails(t *testing.T) {
	trace := "trace-456"
	details := []FieldIssue{{Field: "field", Issue: "bad"}}
	env := NewErrorEnvelope[struct{}](&trace, "CODE", "message", details)

	if env.Data != nil {
		t.Fatalf("expected Data to be nil, got %+v", env.Data)
	}
	if env.Error == nil {
		t.Fatalf("expected Error to be non-nil")
	}
	if env.Meta.TraceID == nil || *env.Meta.TraceID != trace {
		t.Fatalf("expected traceId %q, got %+v", trace, env.Meta.TraceID)
	}
	if env.Error.TraceID == nil || *env.Error.TraceID != trace {
		t.Fatalf("expected error traceId %q, got %+v", trace, env.Error.TraceID)
	}
	if len(env.Error.Details) != 1 || env.Error.Details[0].Field != "field" || env.Error.Details[0].Issue != "bad" {
		t.Fatalf("unexpected details: %+v", env.Error.Details)
	}

	details[0].Issue = "mutated"
	if env.Error.Details[0].Issue != "bad" {
		t.Fatalf("details should be copied, got %q", env.Error.Details[0].Issue)
	}
}
