package detector

import (
	"context"
	"fmt"

	v1 "github.com/google/go-containerregistry/pkg/v1"
)

// Format represents the detected image layer format
type Format int

const (
	// FormatUnknown indicates the format could not be determined
	FormatUnknown Format = iota

	// FormatStandard indicates a standard OCI/Docker layer
	FormatStandard

	// FormatEStargz indicates an eStargz layer
	FormatEStargz

	// FormatSOCI indicates a SOCI-indexed layer
	FormatSOCI

	// FormatZstd indicates a zstd-compressed layer
	FormatZstd

	// FormatZstdChunked indicates a zstd:chunked (seekable) layer
	FormatZstdChunked
)

// String returns the string representation of the format
func (f Format) String() string {
	switch f {
	case FormatStandard:
		return "standard"
	case FormatEStargz:
		return "estargz"
	case FormatSOCI:
		return "soci"
	case FormatZstd:
		return "zstd"
	case FormatZstdChunked:
		return "zstd:chunked"
	default:
		return "unknown"
	}
}

// DetectFormat determines the format of an OCI layer
func DetectFormat(ctx context.Context, layer v1.Layer) (Format, error) {
	// Check media type first
	mediaType, err := layer.MediaType()
	if err != nil {
		return FormatUnknown, fmt.Errorf("failed to get media type: %w", err)
	}

	mt := string(mediaType)

	// Check for zstd compression based on media type
	if mt == "application/vnd.oci.image.layer.v1.tar+zstd" ||
		mt == "application/vnd.docker.image.rootfs.diff.tar.zstd" {
		// Could be either standard zstd or zstd:chunked
		// Try to detect if it has a chunked footer (similar to eStargz)
		// For now, return FormatZstd and let the orchestrator try chunked first
		return FormatZstd, nil
	}

	// Check for eStargz footer
	// eStargz layers have a magic footer at the end
	hasEStargzFooter, err := checkEStargzFooter(layer)
	if err == nil && hasEStargzFooter {
		return FormatEStargz, nil
	}

	// Check annotations for SOCI
	// SOCI layers are typically standard layers but with associated indices
	// We'd need to check if a SOCI index exists for this layer
	// For now, we'll check media type hints
	if mt == "application/vnd.oci.image.layer.v1.tar+gzip" ||
		mt == "application/vnd.docker.image.rootfs.diff.tar.gzip" {
		// Could be either eStargz or standard
		// Default to standard if no eStargz footer
		return FormatStandard, nil
	}

	return FormatUnknown, nil
}

// checkEStargzFooter checks if a layer has the eStargz magic footer
func checkEStargzFooter(layer v1.Layer) (bool, error) {
	// Get compressed reader
	rc, err := layer.Compressed()
	if err != nil {
		return false, err
	}
	defer func() { _ = rc.Close() }()

	// The eStargz footer is in the last 47 bytes
	// We'd need to seek to the end, but rc is just an io.ReadCloser
	// In a real implementation, we'd use a ReaderAt or convert to one

	// For now, let's check the size and attempt to read
	size, err := layer.Size()
	if err != nil {
		return false, err
	}

	// If layer is too small, it can't have an eStargz footer
	if size < 47 {
		return false, nil
	}

	// We'd need to read the last 47 bytes and check for the magic number
	// This is a simplified check - real implementation would need proper seeking
	return false, nil
}
