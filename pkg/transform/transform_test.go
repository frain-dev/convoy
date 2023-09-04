package transform

import (
	"github.com/dop251/goja"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestTransform(t *testing.T) {
	type Name struct {
		FirstName string `json:"first_name,omitempty"`
		LastName  string `json:"last_name,omitempty"`
	}
	type Payload struct {
		Name     string `json:"name"`
		FullName *Name  `json:"full_name"`
	}

	transformer := NewTransformer(goja.New())

	p := Payload{
		Name: "A B C",
		FullName: &Name{
			FirstName: "A",
			LastName:  "B",
		},
	}
	want := []string{"A", "B-C"}

	result, err := transformer.Transform(`
	function transform(payload){
		const [first_name, ...rest] = payload?.name?.trim().replace(/([ ,])+/g, ' ').split(' ');
		return [first_name, rest.join('-')];
	}`, p)
	require.NoError(t, err)

	for i := 0; i < len(want); i++ {
		assert.Equal(t, result.([]interface{})[i], want[i])
	}
}
