package transform

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
)

type Name struct {
	FirstName string `json:"first_name,omitempty"`
	LastName  string `json:"last_name,omitempty"`
}

type Payload struct {
	Name     string `json:"name"`
	FullName *Name  `json:"full_name"`
}

func TestTransform(t *testing.T) {
	p := Payload{
		Name: "A B C",
		FullName: &Name{
			FirstName: "A",
			LastName:  "B",
		},
	}

	want := []string{"A", "B-C"}
	function := `function transform(payload){
		const [first_name, ...rest] = payload?.name?.trim().replace(/([ ,])+/g, ' ').split(' ');
		return [first_name, rest.join('-')];
	}`

	transformer := NewTransformer()
	result, _, err := transformer.Transform(function, p)
	require.NoError(t, err)

	for i := 0; i < len(want); i++ {
		assert.Equal(t, result.([]interface{})[i], want[i])
	}
}

func TestTransformJsSyntaxError(t *testing.T) {
	p := Payload{
		Name: "A B C",
		FullName: &Name{
			FirstName: "A",
			LastName:  "B",
		},
	}

	function := `function transform(){var i = 0;for (;;) {i++;} return 0;`
	transformer := NewTransformer()
	_, _, err := transformer.Transform(function, p)
	require.True(t, strings.Contains(err.Error(), "SyntaxError"))
}

func TestTransformMissingFunction(t *testing.T) {
	p := Payload{
		Name: "A B C",
		FullName: &Name{
			FirstName: "A",
			LastName:  "B",
		},
	}

	function := `var i = 0; i++`
	transformer := NewTransformer()
	_, _, err := transformer.Transform(function, p)
	require.ErrorIs(t, ErrFunctionNotFound, err)
}

func TestTransformFunctionNotFound(t *testing.T) {
	p := Payload{
		Name: "A B C",
		FullName: &Name{
			FirstName: "A",
			LastName:  "B",
		},
	}

	function := `function run(){var i = 0;for (;;) {i++;} return 0;}`

	transformer := NewTransformer()
	_, _, err := transformer.Transform(function, p)
	require.ErrorIs(t, ErrFunctionNotFound, err)
}

func TestTransformScriptTimeout(t *testing.T) {
	p := Payload{
		Name: "A B C",
		FullName: &Name{
			FirstName: "A",
			LastName:  "B",
		},
	}

	function := `function transform(){var i = 0;for (;;) {i++;} return 0;}`

	transformer := NewTransformer()
	_, _, err := transformer.Transform(function, p)
	require.True(t, strings.Contains(err.Error(), ErrMaxExecutionTimeElapsed.Error()))

}

func TestTransformScriptTimeoutMalformedScript(t *testing.T) {
	p := Payload{
		Name: "A B C",
		FullName: &Name{
			FirstName: "A",
			LastName:  "B",
		},
	}

	function := `var i = 0;for (;;) {i++;}`
	transformer := NewTransformer()
	_, _, err := transformer.Transform(function, p)
	require.True(t, strings.Contains(err.Error(), ErrMaxExecutionTimeElapsed.Error()))
}

func BenchmarkRunStringRaw(b *testing.B) {
	function := `
    (() => {
		const [first_name, ...rest] = payload?.name?.trim().replace(/([ ,])+/g, ' ').split(' ');
		return [first_name, payload.full_name.first_name, rest.join(' '), payload.full_name.last_name];
	})()`

	p := Payload{
		Name: "A B C",
		FullName: &Name{
			FirstName: "A",
			LastName:  "B",
		},
	}

	b.ResetTimer()
	b.ReportAllocs()

	transformer := NewTransformer()

	for i := 0; i < b.N; i++ {
		_, _, err := transformer.RunStringUnsafe(function, p)
		require.NoError(b, err)
	}
}

func BenchmarkRunStringRaw_NewVM(b *testing.B) {
	function := `
    (() => {
		const [first_name, ...rest] = payload?.name?.trim().replace(/([ ,])+/g, ' ').split(' ');
		return [first_name, payload.full_name.first_name, rest.join(' '), payload.full_name.last_name];
	})()`

	p := Payload{
		Name: "A B C",
		FullName: &Name{
			FirstName: "A",
			LastName:  "B",
		},
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		transformer := NewTransformer()
		_, _, err := transformer.RunStringUnsafe(function, p)
		require.NoError(b, err)
	}
}

func BenchmarkTransform(b *testing.B) {
	function := `function transform(payload){
		const [first_name, ...rest] = payload?.name?.trim().replace(/([ ,])+/g, ' ').split(' ');
		return [first_name, rest.join('-')];
	}`

	p := Payload{
		Name: "A B C",
		FullName: &Name{
			FirstName: "A",
			LastName:  "B",
		},
	}

	transformer := NewTransformer()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, _, err := transformer.Transform(function, p)
		require.NoError(b, err)
	}
}

func BenchmarkTransform_NewVM(b *testing.B) {
	function := `function transform(payload){
		const [first_name, ...rest] = payload?.name?.trim().replace(/([ ,])+/g, ' ').split(' ');
		return [first_name, rest.join('-')];
	}`

	p := Payload{
		Name: "A B C",
		FullName: &Name{
			FirstName: "A",
			LastName:  "B",
		},
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		transformer := NewTransformer()
		_, _, err := transformer.Transform(function, p)
		require.NoError(b, err)
	}
}

func Benchmark_TransformUsingUnderscoreJs(b *testing.B) {
	function := `function transform(payload){
		const [first_name, ...rest] = payload?.name?.trim().replace(/([ ,])+/g, ' ').split(' ');
		return [first_name, rest.join('-')];
	}`

	p := Payload{
		Name: "A B C",
		FullName: &Name{
			FirstName: "A",
			LastName:  "B",
		},
	}

	transformer := NewTransformer()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, _, err := transformer.TransformUsingUnderscoreJs(function, p)
		require.NoError(b, err)
	}
}

func Benchmark_TransformUsingUnderscoreJs_NewVM(b *testing.B) {
	function := `function transform(payload){
		const [first_name, ...rest] = payload?.name?.trim().replace(/([ ,])+/g, ' ').split(' ');
		return [first_name, rest.join('-')];
	}`

	p := Payload{
		Name: "A B C",
		FullName: &Name{
			FirstName: "A",
			LastName:  "B",
		},
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		transformer := NewTransformer()
		_, _, err := transformer.TransformUsingUnderscoreJs(function, p)
		require.NoError(b, err)
	}
}
