//go:build !linux

package soci

import (
	"context"
	"io"
)

// Extractor handles file extraction from SOCI-indexed layers
type Extractor struct {
	reader io.ReaderAt
	size   int64
}

// NewExtractor returns an error on non-Linux platforms
func NewExtractor(reader io.ReaderAt, size int64, ztocBlob []byte) (*Extractor, error) {
	return nil, errSOCINotSupported
}

// ExtractFile returns an error on non-Linux platforms
func (e *Extractor) ExtractFile(ctx context.Context, targetPath string, outputPath string) error {
	return errSOCINotSupported
}

// ListFiles returns an empty list on non-Linux platforms
func (e *Extractor) ListFiles() []string {
	return nil
}
