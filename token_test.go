package hookcamp

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestToken_String(t *testing.T) {

	tt := []struct {
		s string
	}{
		{"xyz"},
		{"ejddd"},
		{"fkkfpekeejddd"},
	}

	for _, v := range tt {
		// Does not seem like a serious test but it checks to make sure
		// the actual values are as is and not modified like `ToUpper`
		// or something of that stuff
		require.Equal(t, v.s, Token(v.s).String())
	}
}

func TestToken_IsZero(t *testing.T) {

	tt := []struct {
		s      string
		isZero bool
	}{
		{"xyz", false},
		{"ejddd", false},
		{"fkkfpekeejddd", false},
		{"", true},
		{"  ", true},
	}

	for _, v := range tt {
		require.Equal(t, v.isZero, Token(v.s).IsZero())
	}
}

func TestToken_Equals(t *testing.T) {

	val, err := NewToken()
	require.NoError(t, err)

	require.True(t, val.Equals(val))

	val2, err2 := NewToken()

	require.NoError(t, err2)
	require.False(t, val.Equals(val2))
}

func TestToken_New(t *testing.T) {

	val, err := NewToken()
	require.NoError(t, err)

	require.False(t, val.IsZero())
}

func BenchmarkTokenCreation(b *testing.B) {
	for i := 0; i <= b.N; i++ {
		_, _ = NewToken()
	}
}
