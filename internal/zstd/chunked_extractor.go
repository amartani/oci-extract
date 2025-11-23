package zstd

import (
	"archive/tar"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/amartani/oci-extract/internal/pathutil"
	"github.com/containerd/stargz-snapshotter/estargz"
	"github.com/klauspost/compress/zstd"
)

// ChunkedExtractor handles file extraction from zstd:chunked (stargz-zstd) layers
// zstd:chunked is a seekable format similar to eStargz but using zstd compression
type ChunkedExtractor struct {
	reader io.ReaderAt
	size   int64
}

// NewChunkedExtractor creates a new zstd:chunked extractor
func NewChunkedExtractor(reader io.ReaderAt, size int64) *ChunkedExtractor {
	return &ChunkedExtractor{
		reader: reader,
		size:   size,
	}
}

// ExtractFile extracts a specific file from a zstd:chunked layer
func (e *ChunkedExtractor) ExtractFile(ctx context.Context, targetPath string, outputPath string) error {
	// Convert ReaderAt to SectionReader
	sr := io.NewSectionReader(e.reader, 0, e.size)

	// Try to open as estargz first (it may support zstd:chunked)
	r, err := estargz.Open(sr)
	if err == nil {
		// Successfully opened as stargz format, try to extract
		_, ok := r.Lookup(targetPath)
		if ok {
			fileReader, err := r.OpenFile(targetPath)
			if err == nil {
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
		}
	}

	// Fall back to standard zstd tar extraction
	// Reset reader to start
	sr = io.NewSectionReader(e.reader, 0, e.size)

	// Create zstd reader
	zstdReader, err := zstd.NewReader(sr)
	if err != nil {
		return fmt.Errorf("failed to create zstd reader: %w", err)
	}
	defer zstdReader.Close()

	// Create tar reader
	tarReader := tar.NewReader(zstdReader)

	// Normalize target path (remove leading slash)
	normalizedTarget := strings.TrimPrefix(targetPath, "/")

	// Iterate through tar archive
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break // End of archive
		}
		if err != nil {
			return fmt.Errorf("failed to read tar entry: %w", err)
		}

		// Normalize the entry name
		normalizedEntry := strings.TrimPrefix(header.Name, "./")
		normalizedEntry = strings.TrimPrefix(normalizedEntry, "/")

		// Check if this is our target file
		if normalizedEntry == normalizedTarget {
			// Found the file!
			// Handle regular files and symlinks
			if header.Typeflag != tar.TypeReg && header.Typeflag != tar.TypeSymlink && header.Typeflag != tar.TypeLink {
				return fmt.Errorf("target path %s is not a regular file or symlink (type: %d)", targetPath, header.Typeflag)
			}

			// If it's a symlink, return an error with the link target
			if header.Typeflag == tar.TypeSymlink || header.Typeflag == tar.TypeLink {
				return fmt.Errorf("target path %s is a symlink to %s, please extract the target instead", targetPath, header.Linkname)
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
			_, err = io.Copy(outFile, tarReader)
			if err != nil {
				return fmt.Errorf("failed to copy file contents: %w", err)
			}

			return nil
		}
	}

	return fmt.Errorf("file %s not found in layer", targetPath)
}

// ListFiles lists all files in a zstd:chunked layer
func (e *ChunkedExtractor) ListFiles(ctx context.Context) ([]string, error) {
	// zstd:chunked is backward-compatible with tar.zstd, so we can read it as a standard tar archive
	// This is less efficient than using the TOC but works correctly

	// Convert ReaderAt to SectionReader
	sr := io.NewSectionReader(e.reader, 0, e.size)

	// Create zstd reader
	zstdReader, err := zstd.NewReader(sr)
	if err != nil {
		return nil, fmt.Errorf("failed to create zstd reader: %w", err)
	}
	defer zstdReader.Close()

	// Create tar reader
	tarReader := tar.NewReader(zstdReader)

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
