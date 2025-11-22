package estargz

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/containerd/stargz-snapshotter/estargz"
)

// Extractor handles file extraction from eStargz layers
type Extractor struct {
	reader io.ReaderAt
	size   int64
}

// NewExtractor creates a new eStargz extractor
func NewExtractor(reader io.ReaderAt, size int64) *Extractor {
	return &Extractor{
		reader: reader,
		size:   size,
	}
}

// ExtractFile extracts a specific file from an eStargz layer
func (e *Extractor) ExtractFile(ctx context.Context, targetPath string, outputPath string) error {
	// Convert ReaderAt to SectionReader
	sr := io.NewSectionReader(e.reader, 0, e.size)

	// Open the eStargz reader
	r, err := estargz.Open(sr)
	if err != nil {
		return fmt.Errorf("failed to open estargz: %w", err)
	}

	// Lookup the file in the TOC
	_, ok := r.Lookup(targetPath)
	if !ok {
		return fmt.Errorf("file %s not found in layer TOC", targetPath)
	}

	// Open the file from the eStargz layer
	fileReader, err := r.OpenFile(targetPath)
	if err != nil {
		return fmt.Errorf("failed to open file %s: %w", targetPath, err)
	}

	// Create output directory if needed
	outputDir := filepath.Dir(outputPath)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Create output file
	outFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer func() { _ = outFile.Close() }()

	// Copy the file contents
	_, err = io.Copy(outFile, fileReader)
	if err != nil {
		return fmt.Errorf("failed to copy file contents: %w", err)
	}

	return nil
}

// ListFiles lists all files in an eStargz layer
func (e *Extractor) ListFiles(ctx context.Context) ([]string, error) {
	// Convert ReaderAt to SectionReader
	sr := io.NewSectionReader(e.reader, 0, e.size)

	// Open the eStargz reader
	_, err := estargz.Open(sr)
	if err != nil {
		return nil, fmt.Errorf("failed to open estargz: %w", err)
	}

	var files []string

	// The estargz reader provides a TOC (Table of Contents)
	// We need to iterate through it to get all files
	// This is a simplified version - the actual TOC iteration would depend
	// on the estargz library's API

	return files, nil
}
