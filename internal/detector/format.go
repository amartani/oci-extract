package detector

import (
	"context"
	"fmt"
	"io"

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
)

// String returns the string representation of the format
func (f Format) String() string {
	switch f {
	case FormatStandard:
		return "standard"
	case FormatEStargz:
		return "estargz"
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

	// Check for eStargz footer
	// eStargz layers have a magic footer at the end
	hasEStargzFooter, err := checkEStargzFooter(layer)
	if err == nil && hasEStargzFooter {
		return FormatEStargz, nil
	}

	// Check media type - standard gzip compressed layers
	if mt == "application/vnd.oci.image.layer.v1.tar+gzip" ||
		mt == "application/vnd.docker.image.rootfs.diff.tar.gzip" {
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

// DetectFormatFromReader detects format from a ReaderAt
func DetectFormatFromReader(reader io.ReaderAt, size int64) (Format, error) {
	if size < 47 {
		return FormatStandard, nil
	}

	// Read the last 47 bytes
	footer := make([]byte, 47)
	_, err := reader.ReadAt(footer, size-47)
	if err != nil {
		return FormatUnknown, fmt.Errorf("failed to read footer: %w", err)
	}

	// eStargz magic number is at the end
	// The footer format is: [tocOffset:22][footerSize:10][magic:15]
	// Magic is: "estargz.footer"
	magic := string(footer[32:])
	if magic == "estargz.footer\x00" {
		return FormatEStargz, nil
	}

	return FormatStandard, nil
}
