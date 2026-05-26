package util

import (
	"strings"
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

func TestGenerateRandomString(t *testing.T) {
	got, err := GenerateRandomString(64)
	require.NoError(t, err)
	require.Len(t, got, 64)

	for _, ch := range got {
		require.True(t, strings.ContainsRune(string(letterBytes), ch))
	}
}

func TestGenerateRandomStringZeroLength(t *testing.T) {
	got, err := GenerateRandomString(0)
	require.NoError(t, err)
	require.Empty(t, got)
}

func BenchmarkGenerateRandomString100(b *testing.B) {
	for n := 0; n < b.N; n++ {
		_, err := GenerateRandomString(20)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGenerateRandomString1000(b *testing.B) {
	for n := 0; n < b.N; n++ {
		_, err := GenerateRandomString(20)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGenerateRandomString10000(b *testing.B) {
	for n := 0; n < b.N; n++ {
		_, err := GenerateRandomString(20)
		if err != nil {
			b.Fatal(err)
		}
	}
}
