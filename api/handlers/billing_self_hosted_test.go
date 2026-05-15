package handlers

import "testing"

func TestMaskEmailForLog(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		email string
		want  string
	}{
		{name: "empty", email: "", want: ""},
		{name: "normal", email: "smart@example.com", want: "s***@example.com"},
		{name: "single char local", email: "a@example.com", want: "a***@example.com"},
		{name: "missing local", email: "@example.com", want: "***@example.com"},
		{name: "missing domain", email: "smart@", want: "***"},
		{name: "invalid no at", email: "smartexample.com", want: "***"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := maskEmailForLog(tt.email); got != tt.want {
				t.Fatalf("maskEmailForLog(%q) = %q, want %q", tt.email, got, tt.want)
			}
		})
	}
}
