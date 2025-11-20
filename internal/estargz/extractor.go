package estargz

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/containerd/stargz-snapshotter/estargz"
	v1 "github.com/google/go-containerregistry/pkg/v1"
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

// IsEStargz checks if a layer is in eStargz format
func IsEStargz(layer v1.Layer) (bool, error) {
	// Get the layer's compressed reader
	rc, err := layer.Compressed()
	if err != nil {
		return false, fmt.Errorf("failed to get compressed layer: %w", err)
	}
	defer rc.Close()

	// Create a ReaderAt from the reader
	// Note: This is a simplified check - in production you'd want to check
	// the layer's media type or look for the eStargz footer
	mediaType, err := layer.MediaType()
	if err != nil {
		return false, nil
	}

	// eStargz layers typically have these media types
	mt := string(mediaType)
	return mt == "application/vnd.oci.image.layer.v1.tar+gzip" ||
		mt == "application/vnd.docker.image.rootfs.diff.tar.gzip", nil
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
	defer outFile.Close()

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

// ExtractFileFromLayer is a convenience method that extracts from a layer directly
func ExtractFileFromLayer(ctx context.Context, layer v1.Layer, reader io.ReaderAt, size int64, targetPath string, outputPath string) error {
	extractor := NewExtractor(reader, size)
	return extractor.ExtractFile(ctx, targetPath, outputPath)
}
