package io

import (
	"fmt"
	"io"
)

// countingReadCloser wraps an io.ReadCloser and counts the total bytes read.
type countingReadCloser struct {
	io.ReadCloser
	count *int
}

// NewCountingReadCloser creates a new countingReadCloser that counts bytes as they're read.
// The count pointer is updated in real-time as bytes are read, not just on close.
func NewCountingReadCloser(rc io.ReadCloser, count *int) io.ReadCloser {
	return &countingReadCloser{
		ReadCloser: rc,
		count:      count,
	}
}

func (crc *countingReadCloser) Read(p []byte) (n int, err error) {
	n, err = crc.ReadCloser.Read(p)
	if crc.count != nil {
		*crc.count += n
	}
	if err != nil && err != io.EOF {
		return n, fmt.Errorf("counting read: %w", err)
	}
	return n, err // preserves io.EOF
}

func (crc *countingReadCloser) Close() error {
	err := crc.ReadCloser.Close()
	if err != nil {
		return fmt.Errorf("close counting reader: %w", err)
	}
	return nil
}
