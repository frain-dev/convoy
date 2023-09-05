package transform

import (
	"fmt"
	"github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/console"
	"github.com/dop251/goja_nodejs/require"
	"io"
	"net/http"
)

type Transformer struct {
	vm *goja.Runtime
}

func NewTransformer(runtime *goja.Runtime) *Transformer {
	runtime.SetFieldNameMapper(goja.TagFieldNameMapper("json", true))
	return &Transformer{vm: runtime}
}

const url = "https://underscorejs.org/underscore-min.js"

var transform func(interface{}) interface{}

func closeWithError(closer io.Closer) {
	err := closer.Close()
	if err != nil {
		fmt.Printf("%v, an error occurred while closing the client", err)
	}
}

// TransformUsingUnderscoreJs downloads the underscore js library and then mutates the payload by the passed function.
// The output of TransformUsingUnderscoreJs should be idempotent
func (t *Transformer) TransformUsingUnderscoreJs(function string, payload interface{}) (interface{}, error) {
	res, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer closeWithError(res.Body)

	data, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	_, err = t.vm.RunString(string(data))
	if err != nil {
		return nil, err
	}

	return t.Transform(function, payload)
}

func (t *Transformer) RunStringUnsafe(function string, payload interface{}) (interface{}, error) {
	err := t.vm.Set("payload", payload)
	if err != nil {
		return nil, err
	}

	value, err := t.vm.RunString(function)
	if err != nil {
		return nil, err
	}

	return value, nil
}

// Transform mutates the payload by the passed function
// The output of Transform should be idempotent
func (t *Transformer) Transform(function string, payload interface{}) (interface{}, error) {
	new(require.Registry).Enable(t.vm)
	console.Enable(t.vm)

	_, err := t.vm.RunString(function)
	if err != nil {
		return nil, err
	}

	err = t.vm.ExportTo(t.vm.Get("transform"), &transform)
	if err != nil {
		return nil, err
	}

	return transform(payload), err
}
