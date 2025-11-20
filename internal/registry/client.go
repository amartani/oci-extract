package registry

import (
	"context"
	"fmt"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

// Client handles OCI registry operations
type Client struct {
	authOpts []remote.Option
	imageRef string // Store the image reference for URL construction
	ref      name.Reference
}

// NewClient creates a new registry client with authentication
func NewClient() *Client {
	return &Client{
		authOpts: []remote.Option{
			remote.WithAuthFromKeychain(authn.DefaultKeychain),
		},
	}
}

// GetImage fetches an image from a registry
func (c *Client) GetImage(ctx context.Context, imageRef string) (v1.Image, error) {
	ref, err := name.ParseReference(imageRef)
	if err != nil {
		return nil, fmt.Errorf("failed to parse image reference %s: %w", imageRef, err)
	}

	// Store the reference for later use
	c.imageRef = imageRef
	c.ref = ref

	img, err := remote.Image(ref, c.authOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch image %s: %w", imageRef, err)
	}

	return img, nil
}

// GetManifest fetches the manifest for an image
func (c *Client) GetManifest(ctx context.Context, imageRef string) (*v1.Manifest, error) {
	img, err := c.GetImage(ctx, imageRef)
	if err != nil {
		return nil, err
	}

	manifest, err := img.Manifest()
	if err != nil {
		return nil, fmt.Errorf("failed to get manifest: %w", err)
	}

	return manifest, nil
}

// GetLayers returns all layers from an image
func (c *Client) GetLayers(ctx context.Context, imageRef string) ([]v1.Layer, error) {
	img, err := c.GetImage(ctx, imageRef)
	if err != nil {
		return nil, err
	}

	layers, err := img.Layers()
	if err != nil {
		return nil, fmt.Errorf("failed to get layers: %w", err)
	}

	return layers, nil
}

// GetLayerURL returns the direct URL for a layer blob
func (c *Client) GetLayerURL(layer v1.Layer) (string, error) {
	digest, err := layer.Digest()
	if err != nil {
		return "", fmt.Errorf("failed to get layer digest: %w", err)
	}

	if c.ref == nil {
		return "", fmt.Errorf("no image reference available - call GetImage first")
	}

	// Construct the blob URL: registry/v2/repository/blobs/digest
	repo := c.ref.Context()
	registry := repo.Registry.Name()
	repoName := repo.RepositoryStr()

	// For Docker Hub, use registry-1.docker.io
	if registry == "index.docker.io" {
		registry = "registry-1.docker.io"
	}

	blobURL := fmt.Sprintf("https://%s/v2/%s/blobs/%s", registry, repoName, digest.String())
	return blobURL, nil
}

// LayerInfo contains metadata about a layer
type LayerInfo struct {
	Digest    v1.Hash
	Size      int64
	MediaType string
	BlobURL   string // The direct URL to download the layer
}

// EnhancedLayerInfo contains a layer with its metadata and download URL
type EnhancedLayerInfo struct {
	Layer     v1.Layer
	Digest    v1.Hash
	Size      int64
	MediaType string
	BlobURL   string
}

// GetLayerInfo returns metadata about a layer
func (c *Client) GetLayerInfo(layer v1.Layer) (*LayerInfo, error) {
	digest, err := layer.Digest()
	if err != nil {
		return nil, fmt.Errorf("failed to get digest: %w", err)
	}

	size, err := layer.Size()
	if err != nil {
		return nil, fmt.Errorf("failed to get size: %w", err)
	}

	mediaType, err := layer.MediaType()
	if err != nil {
		return nil, fmt.Errorf("failed to get media type: %w", err)
	}

	blobURL, err := c.GetLayerURL(layer)
	if err != nil {
		return nil, fmt.Errorf("failed to get blob URL: %w", err)
	}

	return &LayerInfo{
		Digest:    digest,
		Size:      size,
		MediaType: string(mediaType),
		BlobURL:   blobURL,
	}, nil
}

// GetEnhancedLayers returns all layers with their metadata and download URLs
func (c *Client) GetEnhancedLayers(ctx context.Context, imageRef string) ([]*EnhancedLayerInfo, error) {
	layers, err := c.GetLayers(ctx, imageRef)
	if err != nil {
		return nil, err
	}

	enhancedLayers := make([]*EnhancedLayerInfo, 0, len(layers))
	for _, layer := range layers {
		info, err := c.GetLayerInfo(layer)
		if err != nil {
			return nil, fmt.Errorf("failed to get layer info: %w", err)
		}

		enhancedLayers = append(enhancedLayers, &EnhancedLayerInfo{
			Layer:     layer,
			Digest:    info.Digest,
			Size:      info.Size,
			MediaType: info.MediaType,
			BlobURL:   info.BlobURL,
		})
	}

	return enhancedLayers, nil
}
