package transform

import (
	"fmt"
	"strings"
)

const newLine = " \r\n"

func NewBufferPrinter() *BuffPrinter {
	bufferLogger := &strings.Builder{}
	return &BuffPrinter{
		Buff: bufferLogger,
		BuffOutPrint: func(s string) {
			bufferLogger.WriteString(fmt.Sprintf("%s%s", s, newLine))
		},
		BuffErrPrint: func(s string) {
			bufferLogger.WriteString(fmt.Sprintf("%s%s", s, newLine))
		},
	}
}

// BuffPrinter implements the console.Printer interface
// that writes to a buffer.
type BuffPrinter struct {
	Buff         *strings.Builder
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

func (b *BuffPrinter) Format() []string {
	return strings.Split(b.Buff.String(), newLine)
}
