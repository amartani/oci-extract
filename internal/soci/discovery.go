package soci

import (
	"context"
	"fmt"
	"io"

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
	// Construct a proper Digest reference from the repository and hash
	repo := ref.Context()
	digestRef, err := name.NewDigest(fmt.Sprintf("%s@%s", repo.String(), digest.String()))
	if err != nil {
		return nil, fmt.Errorf("failed to construct digest reference: %w", err)
	}

	// Query the referrers API
	index, err := remote.Referrers(digestRef, remote.WithAuthFromKeychain(authn.DefaultKeychain))
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
	// Fetch the SOCI index using the descriptor's digest
	repo := info.Reference.Context()
	digestRef, err := name.NewDigest(fmt.Sprintf("%s@%s", repo.String(), info.Descriptor.Digest.String()))
	if err != nil {
		return nil, fmt.Errorf("failed to construct digest reference: %w", err)
	}

	// Fetch the SOCI index as an OCI Image Index
	idx, err := remote.Index(digestRef, remote.WithAuthFromKeychain(authn.DefaultKeychain))
	if err != nil {
		return nil, fmt.Errorf("failed to fetch SOCI index: %w", err)
	}

	// Get the index manifest
	manifest, err := idx.IndexManifest()
	if err != nil {
		return nil, fmt.Errorf("failed to get index manifest: %w", err)
	}

	return manifest, nil
}

// GetZtocForLayer fetches the zTOC blob for a specific layer
func GetZtocForLayer(ctx context.Context, info *IndexInfo, layerDigest v1.Hash) ([]byte, error) {
	// Get the SOCI index manifest
	indexManifest, err := GetSOCIIndex(ctx, info)
	if err != nil {
		return nil, err
	}

	// Find the zTOC descriptor for the layer
	// SOCI index manifests contain descriptors for zTOC blobs
	// Each zTOC is annotated with the layer digest it corresponds to
	var ztocDescriptor *v1.Descriptor
	for i, desc := range indexManifest.Manifests {
		// Check annotations for layer digest reference
		if desc.Annotations != nil {
			if digest, ok := desc.Annotations["com.amazon.aws.soci.layer.digest"]; ok {
				if digest == layerDigest.String() {
					ztocDescriptor = &indexManifest.Manifests[i]
					break
				}
			}
		}
	}

	if ztocDescriptor == nil {
		return nil, fmt.Errorf("no zTOC found for layer %s", layerDigest)
	}

	// Fetch the zTOC blob
	repo := info.Reference.Context()
	ztocRef, err := name.NewDigest(fmt.Sprintf("%s@%s", repo.String(), ztocDescriptor.Digest.String()))
	if err != nil {
		return nil, fmt.Errorf("failed to construct zTOC reference: %w", err)
	}

	// Fetch the zTOC blob
	layer, err := remote.Layer(ztocRef, remote.WithAuthFromKeychain(authn.DefaultKeychain))
	if err != nil {
		return nil, fmt.Errorf("failed to fetch zTOC blob: %w", err)
	}

	// Read the zTOC blob
	rc, err := layer.Uncompressed()
	if err != nil {
		return nil, fmt.Errorf("failed to get uncompressed zTOC: %w", err)
	}
	defer func() { _ = rc.Close() }()

	// Read all the zTOC data
	ztocData, err := io.ReadAll(rc)
	if err != nil {
		return nil, fmt.Errorf("failed to read zTOC data: %w", err)
	}

	return ztocData, nil
}
