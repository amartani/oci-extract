package soci

import (
	"context"
	"fmt"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

const (
	// SOCIIndexMediaType is the media type for SOCI index artifacts
	SOCIIndexMediaType = "application/vnd.aws.soci.index.v1+json"

	// SOCIIndexAnnotation is the annotation key for SOCI indices
	SOCIIndexAnnotation = "com.amazon.aws.soci.index"
)

// IndexInfo contains information about a SOCI index
type IndexInfo struct {
	Descriptor v1.Descriptor
	Reference  name.Reference
}

// DiscoverSOCIIndex finds the SOCI index for an image
func DiscoverSOCIIndex(ctx context.Context, imageRef string) (*IndexInfo, error) {
	ref, err := name.ParseReference(imageRef)
	if err != nil {
		return nil, fmt.Errorf("failed to parse reference: %w", err)
	}

	// Get the image to find its digest
	img, err := remote.Image(ref, remote.WithAuthFromKeychain(authn.DefaultKeychain))
	if err != nil {
		return nil, fmt.Errorf("failed to fetch image: %w", err)
	}

	digest, err := img.Digest()
	if err != nil {
		return nil, fmt.Errorf("failed to get image digest: %w", err)
	}

	// Try using the Referrers API (OCI 1.1)
	indexInfo, err := findViaReferrersAPI(ctx, ref, digest)
	if err == nil {
		return indexInfo, nil
	}

	// Fallback: Try the tag-based approach
	return findViaTagReference(ctx, ref, digest)
}

// findViaReferrersAPI uses the OCI Referrers API to find SOCI indices
func findViaReferrersAPI(ctx context.Context, ref name.Reference, digest v1.Hash) (*IndexInfo, error) {
	// Query the referrers API
	index, err := remote.Referrers(digest, remote.WithAuthFromKeychain(authn.DefaultKeychain))
	if err != nil {
		return nil, fmt.Errorf("failed to query referrers: %w", err)
	}

	manifest, err := index.IndexManifest()
	if err != nil {
		return nil, fmt.Errorf("failed to get index manifest: %w", err)
	}

	// Look for SOCI index artifact
	for _, desc := range manifest.Manifests {
		if desc.ArtifactType == SOCIIndexMediaType {
			return &IndexInfo{
				Descriptor: desc,
				Reference:  ref,
			}, nil
		}

		// Also check media type as fallback
		if desc.MediaType == SOCIIndexMediaType {
			return &IndexInfo{
				Descriptor: desc,
				Reference:  ref,
			}, nil
		}
	}

	return nil, fmt.Errorf("no SOCI index found in referrers")
}

// findViaTagReference tries to find SOCI index using tag-based naming
func findViaTagReference(ctx context.Context, ref name.Reference, digest v1.Hash) (*IndexInfo, error) {
	// SOCI indices are often tagged as sha256-<digest>.soci
	sociTag := fmt.Sprintf("sha256-%s.soci", digest.Hex)

	// Construct the SOCI index reference
	repo := ref.Context()
	sociRef, err := name.NewTag(fmt.Sprintf("%s:%s", repo.String(), sociTag))
	if err != nil {
		return nil, fmt.Errorf("failed to construct SOCI tag: %w", err)
	}

	// Try to fetch the SOCI index
	desc, err := remote.Get(sociRef, remote.WithAuthFromKeychain(authn.DefaultKeychain))
	if err != nil {
		return nil, fmt.Errorf("failed to fetch SOCI index via tag: %w", err)
	}

	return &IndexInfo{
		Descriptor: desc.Descriptor,
		Reference:  sociRef,
	}, nil
}

// GetSOCIIndex fetches and returns the SOCI index manifest
func GetSOCIIndex(ctx context.Context, info *IndexInfo) (*v1.IndexManifest, error) {
	img, err := remote.Image(info.Reference, remote.WithAuthFromKeychain(authn.DefaultKeychain))
	if err != nil {
		return nil, fmt.Errorf("failed to fetch SOCI index image: %w", err)
	}

	manifest, err := img.Manifest()
	if err != nil {
		return nil, fmt.Errorf("failed to get manifest: %w", err)
	}

	// The SOCI index is actually stored as an OCI index
	// We need to parse it differently
	return nil, fmt.Errorf("SOCI index parsing not yet implemented")
}
