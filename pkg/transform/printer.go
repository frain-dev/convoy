package transform

import (
	"bytes"
	"fmt"
)

func NewBufferPrinter() *BuffPrinter {
	bufferLogger := &bytes.Buffer{}
	return &BuffPrinter{
		Buff: bufferLogger,
		BuffOutPrint: func(s string) {
			bufferLogger.WriteString(fmt.Sprintf("%s\n", s))
		},
		BuffErrPrint: func(s string) {
			bufferLogger.WriteString(fmt.Sprintf("%s\n", s))
		},
	}
}

// BuffPrinter implements the console.Printer interface
// that writes to a buffer.
type BuffPrinter struct {
	Buff         *bytes.Buffer
	BuffOutPrint func(s string)
	BuffErrPrint func(s string)
}

// Log writes s to a buffer.
func (b *BuffPrinter) Log(s string) {
	b.BuffOutPrint(s)
}

// Warn writes s to a buffer.
func (b *BuffPrinter) Warn(s string) {
	b.BuffErrPrint(s)
}

// Error writes s to a buffer.
func (b *BuffPrinter) Error(s string) {
	b.BuffErrPrint(s)
}

func (b *BuffPrinter) String() string {
	return string(b.Buff.Bytes())
}
