package zstd

import (
	"archive/tar"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/klauspost/compress/zstd"
)

// Extractor handles file extraction from standard zstd-compressed OCI layers
type Extractor struct {
	layer v1.Layer
}

// NewExtractor creates a new standard zstd layer extractor
func NewExtractor(layer v1.Layer) *Extractor {
	return &Extractor{
		layer: layer,
	}
}

// ExtractFile extracts a specific file from a zstd-compressed OCI layer
// This downloads and decompresses the entire layer using zstd
func (e *Extractor) ExtractFile(ctx context.Context, targetPath string, outputPath string) error {
	// Get the compressed layer data
	rc, err := e.layer.Compressed()
	if err != nil {
		return fmt.Errorf("failed to get compressed layer: %w", err)
	}
	defer func() { _ = rc.Close() }()

	// Create zstd reader
	zstdReader, err := zstd.NewReader(rc)
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

// ListFiles lists all files in a zstd-compressed OCI layer
func (e *Extractor) ListFiles(ctx context.Context) ([]string, error) {
	// Get the compressed layer data
	rc, err := e.layer.Compressed()
	if err != nil {
		return nil, fmt.Errorf("failed to get compressed layer: %w", err)
	}
	defer func() { _ = rc.Close() }()

	// Create zstd reader
	zstdReader, err := zstd.NewReader(rc)
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
			files = append(files, header.Name)
		}
	}

	return files, nil
}
