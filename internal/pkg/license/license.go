package license

import (
	"sync/atomic"
)

var LICENSE atomic.Bool

type Licenser interface {
	Set(l bool)
	Check() bool
}

func NewLicense() Licenser {
	return NewLocalLicense()
}

type LocalLicense struct{}

func NewLocalLicense() LocalLicense {
	return LocalLicense{}
}

func (ll LocalLicense) Set(l bool) {
	LICENSE.Store(l)
}

func (ll LocalLicense) Check() bool {
	return LICENSE.Load()
}
