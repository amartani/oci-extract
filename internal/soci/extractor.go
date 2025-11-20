package soci

import (
	"compress/gzip"
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
	ztoc   *ztoc.Ztoc
}

// NewExtractor creates a new SOCI extractor
func NewExtractor(reader io.ReaderAt, ztocBlob []byte) (*Extractor, error) {
	// Parse the zTOC blob
	z, err := ztoc.Unmarshal(ztocBlob)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal ztoc: %w", err)
	}

	return &Extractor{
		reader: reader,
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

	// This is a simplified version - actual implementation would iterate
	// through the zTOC's file entries
	for _, entry := range e.ztoc.TOC {
		if entry.Name == targetPath {
			return &FileSpan{
				StartOffset: entry.Offset,
				EndOffset:   entry.Offset + entry.CompressedSize,
				UncompressedSize: entry.UncompressedSize,
			}, nil
		}
	}

	return nil, fmt.Errorf("file %s not found in zTOC", targetPath)
}

// ExtractFile extracts a specific file using the zTOC information
func (e *Extractor) ExtractFile(ctx context.Context, targetPath string, outputPath string) error {
	// Find the file in the zTOC
	span, err := e.FindFile(targetPath)
	if err != nil {
		return err
	}

	// Read the compressed chunk from the layer
	compressedData := make([]byte, span.EndOffset-span.StartOffset)
	_, err = e.reader.ReadAt(compressedData, span.StartOffset)
	if err != nil {
		return fmt.Errorf("failed to read compressed data: %w", err)
	}

	// Decompress the data
	// Note: SOCI may require initializing the gzip reader with checkpoint state
	// for files deep in the stream. This is a simplified version.
	decompressed, err := decompressChunk(compressedData)
	if err != nil {
		return fmt.Errorf("failed to decompress chunk: %w", err)
	}

	// Create output directory if needed
	outputDir := filepath.Dir(outputPath)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Write to output file
	if err := os.WriteFile(outputPath, decompressed, 0644); err != nil {
		return fmt.Errorf("failed to write output file: %w", err)
	}

	return nil
}

// decompressChunk decompresses a gzip chunk
func decompressChunk(data []byte) ([]byte, error) {
	reader, err := gzip.NewReader(io.NopCloser(io.Reader(nil)))
	if err != nil {
		return nil, fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer reader.Close()

	// Read the decompressed data
	decompressed, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to decompress: %w", err)
	}

	return decompressed, nil
}

// ListFiles lists all files in the zTOC
func (e *Extractor) ListFiles() []string {
	var files []string
	for _, entry := range e.ztoc.TOC {
		if entry.Type == "reg" { // Regular file
			files = append(files, entry.Name)
		}
	}
	return files
}
