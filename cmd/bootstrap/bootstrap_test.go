package bootstrap

import (
	"io"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/internal/pkg/cli"
)

// These cases all return during up-front argument validation, before the
// command touches the licenser or the database, so a zero-value cli.App is
// sufficient to exercise them.
func TestBootstrapCommand_FlagValidation(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "unsupported format",
			args:    []string{"--format", "xml"},
			wantErr: "unsupported output format",
		},
		{
			name:    "missing email",
			args:    []string{"--format", "json"},
			wantErr: "email is required",
		},
		{
			name:    "api-key-expiration without with-api-key",
			args:    []string{"--email", "seed@example.com", "--api-key-expiration", "90"},
			wantErr: "--api-key-name and --api-key-expiration require --with-api-key",
		},
		{
			name:    "api-key-name without with-api-key",
			args:    []string{"--email", "seed@example.com", "--api-key-name", "seed"},
			wantErr: "--api-key-name and --api-key-expiration require --with-api-key",
		},
		{
			name:    "zero expiration is rejected when explicit",
			args:    []string{"--email", "seed@example.com", "--with-api-key", "--api-key-expiration", "0"},
			wantErr: "--api-key-expiration must be greater than 0 (days)",
		},
		{
			name:    "negative expiration is rejected",
			args:    []string{"--email", "seed@example.com", "--with-api-key", "--api-key-expiration", "-5"},
			wantErr: "--api-key-expiration must be greater than 0 (days)",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cmd := AddBootstrapCommand(&cli.App{})
			cmd.SetArgs(tc.args)
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
			cmd.SilenceUsage = true
			cmd.SilenceErrors = true

			err := cmd.Execute()
			require.Error(t, err)
			require.EqualError(t, err, tc.wantErr)
		})
	}
}
