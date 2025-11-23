package estargz

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/amartani/oci-extract/internal/pathutil"
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
	// eStargz TOC doesn't expose a public API to iterate all entries
	// (the children field is unexported). Since eStargz is backward-compatible
	// with tar.gz, we fall back to reading it as a standard tar archive.
	// This is less efficient than using the TOC but works correctly.

	// Convert ReaderAt to SectionReader
	sr := io.NewSectionReader(e.reader, 0, e.size)

	// Create gzip reader
	gzipReader, err := gzip.NewReader(sr)
	if err != nil {
		return nil, fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer func() { _ = gzipReader.Close() }()

	// Create tar reader
	tarReader := tar.NewReader(gzipReader)

	var files []string

	// Iterate through tar archive
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read tar entry: %w", err)
		}

		// Only include regular files
		if header.Typeflag == tar.TypeReg {
			// Normalize path for consistent display (ensure leading slash)
			files = append(files, pathutil.NormalizeForDisplay(header.Name))
		}
	}

	return files, nil
}
