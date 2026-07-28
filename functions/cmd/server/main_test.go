package main

import "testing"

func TestFunctionHost(t *testing.T) {
	for _, test := range []struct {
		name      string
		localOnly string
		want      string
	}{
		{name: "local only", localOnly: "true", want: "127.0.0.1"},
		{name: "default all interfaces"},
		{name: "case sensitive", localOnly: "TRUE"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := functionHost(test.localOnly); got != test.want {
				t.Fatalf("functionHost(%q) = %q, want %q", test.localOnly, got, test.want)
			}
		})
	}
}
