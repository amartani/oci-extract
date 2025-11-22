//go:build !linux

package soci

import (
	"context"
	"errors"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
)

const (
	// SOCIIndexMediaType is the media type for SOCI index artifacts
	SOCIIndexMediaType = "application/vnd.aws.soci.index.v1+json"

	// SOCIIndexAnnotation is the annotation key for SOCI indices
	SOCIIndexAnnotation = "com.amazon.aws.soci.index"
)

var errSOCINotSupported = errors.New("SOCI support is only available on Linux")

// IndexInfo contains information about a SOCI index
type IndexInfo struct {
	Descriptor v1.Descriptor
	Reference  name.Reference
}

// DiscoverSOCIIndex returns an error on non-Linux platforms
func DiscoverSOCIIndex(ctx context.Context, imageRef string) (*IndexInfo, error) {
	return nil, errSOCINotSupported
}

// GetSOCIIndex returns an error on non-Linux platforms
func GetSOCIIndex(ctx context.Context, info *IndexInfo) (*v1.IndexManifest, error) {
	return nil, errSOCINotSupported
}

// GetZtocForLayer returns an error on non-Linux platforms
func GetZtocForLayer(ctx context.Context, info *IndexInfo, layerDigest v1.Hash) ([]byte, error) {
	return nil, errSOCINotSupported
}
