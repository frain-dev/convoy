package io

import (
	"fmt"
	"io"
)

// countingReadCloser wraps an io.ReadCloser and counts the total bytes read.
type countingReadCloser struct {
	io.ReadCloser
	count   int
	onClose func(int)
}

// NewCountingReadCloser creates a new countingReadCloser that counts bytes as they're read.
func NewCountingReadCloser(rc io.ReadCloser, onClose func(int)) io.ReadCloser {
	return &countingReadCloser{
		ReadCloser: rc,
		onClose:    onClose,
	}
}

func (crc *countingReadCloser) Read(p []byte) (n int, err error) {
	n, err = crc.ReadCloser.Read(p)
	crc.count += n
	if err != nil && err != io.EOF {
		return n, fmt.Errorf("counting read: %w", err) // for other errors aside from EOF, return the error
	}
	return n, err // preserves io.EOF
}

func (crc *countingReadCloser) Close() error {
	err := crc.ReadCloser.Close()
	if crc.onClose != nil {
		crc.onClose(crc.count)
	}
	if err != nil {
		return fmt.Errorf("close counting reader: %w", err)
	}
	return nil
}
