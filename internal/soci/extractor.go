//go:build linux

package soci

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/amartani/oci-extract/internal/pathutil"
	"github.com/awslabs/soci-snapshotter/ztoc"
)

// Extractor handles file extraction from SOCI-indexed layers
type Extractor struct {
	reader io.ReaderAt
	size   int64
	ztoc   *ztoc.Ztoc
}

// NewExtractor creates a new SOCI extractor
func NewExtractor(reader io.ReaderAt, size int64, ztocBlob []byte) (*Extractor, error) {
	// Parse the zTOC blob - convert []byte to io.Reader
	ztocReader := bytes.NewReader(ztocBlob)
	z, err := ztoc.Unmarshal(ztocReader)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal ztoc: %w", err)
	}

	return &Extractor{
		reader: reader,
		size:   size,
		ztoc:   z,
	}, nil
}

// ExtractFile extracts a specific file using the zTOC information
func (e *Extractor) ExtractFile(ctx context.Context, targetPath string, outputPath string) error {
	// Convert ReaderAt to SectionReader for Ztoc.ExtractFile
	sr := io.NewSectionReader(e.reader, 0, e.size)

	// Use the built-in Ztoc ExtractFile method
	data, err := e.ztoc.ExtractFile(sr, targetPath)
	if err != nil {
		return fmt.Errorf("failed to extract file %s: %w", targetPath, err)
	}

	// Create output directory if needed
	outputDir := filepath.Dir(outputPath)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Write to output file
	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write output file: %w", err)
	}

	return nil
}

// ListFiles lists all files in the zTOC
func (e *Extractor) ListFiles() []string {
	var files []string
	for _, entry := range e.ztoc.FileMetadata {
		// Only include regular files
		if entry.Type == "reg" {
			// Normalize path for consistent display (ensure leading slash)
			files = append(files, pathutil.NormalizeForDisplay(entry.Name))
		}
	}
	return files
}
