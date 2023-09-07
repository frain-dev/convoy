package transform

import (
	"errors"
	"fmt"
	"github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/console"
	"github.com/dop251/goja_nodejs/require"
	"io"
	"net/http"
	"time"
)

var ErrFunctionNotFound = errors.New("transform function not found, please define it or rename the existing function")
var ErrMaxExecutionTimeElapsed = errors.New("script execution time elapsed 10 seconds")

type Transformer struct {
	rt *goja.Runtime
}

func NewTransformer(r *goja.Runtime) *Transformer {
	r.SetFieldNameMapper(goja.TagFieldNameMapper("json", true))
	return &Transformer{rt: r}
}

const url = "https://underscorejs.org/underscore-min.js"
const deadline = time.Second * 10

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

	time.AfterFunc(deadline, func() {
		t.rt.Interrupt(ErrMaxExecutionTimeElapsed)
	})

	_, err = t.rt.RunString(string(data))
	if err != nil {
		return nil, err
	}

	return t.Transform(function, payload)
}

func (t *Transformer) RunStringUnsafe(function string, payload interface{}) (interface{}, error) {
	err := t.rt.Set("payload", payload)
	if err != nil {
		return nil, err
	}

	time.AfterFunc(deadline, func() {
		t.rt.Interrupt(ErrMaxExecutionTimeElapsed)
	})

	value, err := t.rt.RunString(function)
	if err != nil {
		return nil, err
	}

	return value, nil
}

// Transform mutates the payload by the passed function
// The output of Transform should be idempotent
func (t *Transformer) Transform(function string, payload interface{}) (interface{}, error) {
	new(require.Registry).Enable(t.rt)
	console.Enable(t.rt)

	time.AfterFunc(deadline, func() {
		t.rt.Interrupt(ErrMaxExecutionTimeElapsed)
	})

	_, err := t.rt.RunString(function)
	if err != nil {
		return nil, err
	}

	f := t.rt.Get("transform")
	if f == nil {
		return nil, ErrFunctionNotFound
	}

	var transform func(interface{}) (interface{}, error)
	err = t.rt.ExportTo(f, &transform)
	if err != nil {
		return nil, err
	}

	time.AfterFunc(deadline, func() {
		t.rt.Interrupt(ErrMaxExecutionTimeElapsed)
	})

	value, err := transform(payload)
	if err != nil {
		return nil, err
	}

	return value, err
}
