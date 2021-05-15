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
