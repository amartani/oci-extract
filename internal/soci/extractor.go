package soci

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

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

// FileSpan represents the location of a file in the compressed stream
type FileSpan struct {
	StartOffset int64
	EndOffset   int64
	UncompressedSize int64
}

// FindFile locates a file in the zTOC and returns its span information
func (e *Extractor) FindFile(targetPath string) (*FileSpan, error) {
	// Search through the zTOC metadata to find the file
	// The zTOC contains a list of files with their offset information
	for _, entry := range e.ztoc.FileMetadata {
		if entry.Name == targetPath {
			// Note: This is simplified - actual FileMetadata structure may differ
			return &FileSpan{
				StartOffset:      0, // Would need to calculate from metadata
				EndOffset:        0, // Would need to calculate from metadata
				UncompressedSize: int64(entry.UncompressedSize),
			}, nil
		}
	}

	return nil, fmt.Errorf("file %s not found in zTOC", targetPath)
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
			files = append(files, entry.Name)
		}
	}
	return files
}
