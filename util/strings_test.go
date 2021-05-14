package util

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsStringEmpty(t *testing.T) {
	tt := []struct {
		s     string
		empty bool
	}{
		{"", true},
		{" ", true},
		{".", false},
		{". ff ", false},
	}

	for _, v := range tt {
		require.Equal(t, v.empty, IsStringEmpty(v.s))
	}
}
