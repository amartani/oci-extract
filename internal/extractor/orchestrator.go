package extractor

import (
	"context"
	"fmt"
	"io"

	"github.com/amartani/oci-extract/internal/detector"
	"github.com/amartani/oci-extract/internal/estargz"
	"github.com/amartani/oci-extract/internal/registry"
	"github.com/amartani/oci-extract/internal/remote"
	"github.com/amartani/oci-extract/internal/soci"
	v1 "github.com/google/go-containerregistry/pkg/v1"
)

// Orchestrator manages the file extraction process
type Orchestrator struct {
	client  *registry.Client
	verbose bool
}

// NewOrchestrator creates a new extraction orchestrator
func NewOrchestrator(verbose bool) *Orchestrator {
	return &Orchestrator{
		client:  registry.NewClient(),
		verbose: verbose,
	}
}

// ExtractOptions contains options for file extraction
type ExtractOptions struct {
	ImageRef   string
	FilePath   string
	OutputPath string
	ForceFormat detector.Format
}

// Extract extracts a file from an OCI image
func (o *Orchestrator) Extract(ctx context.Context, opts ExtractOptions) error {
	// Get image layers
	layers, err := o.client.GetLayers(ctx, opts.ImageRef)
	if err != nil {
		return fmt.Errorf("failed to get image layers: %w", err)
	}

	if o.verbose {
		fmt.Printf("Found %d layers in image\n", len(layers))
	}

	// Check if SOCI index exists for this image
	var sociIndex *soci.IndexInfo
	if opts.ForceFormat == detector.FormatSOCI || opts.ForceFormat == detector.FormatUnknown {
		sociIndex, err = soci.DiscoverSOCIIndex(ctx, opts.ImageRef)
		if err != nil && o.verbose {
			fmt.Printf("No SOCI index found: %v\n", err)
		} else if sociIndex != nil && o.verbose {
			fmt.Println("Found SOCI index for image")
		}
	}

	// Try to extract from each layer (bottom-up, as layers are applied in order)
	for i := len(layers) - 1; i >= 0; i-- {
		layer := layers[i]

		digest, _ := layer.Digest()
		if o.verbose {
			fmt.Printf("Checking layer %s...\n", digest)
		}

		// Try extraction
		extracted, err := o.extractFromLayer(ctx, layer, sociIndex, opts)
		if err != nil {
			if o.verbose {
				fmt.Printf("  Failed: %v\n", err)
			}
			continue
		}

		if extracted {
			return nil
		}
	}

	return fmt.Errorf("file %s not found in any layer", opts.FilePath)
}

// extractFromLayer attempts to extract a file from a single layer
func (o *Orchestrator) extractFromLayer(ctx context.Context, layer v1.Layer, sociIndex *soci.IndexInfo, opts ExtractOptions) (bool, error) {
	// Detect format if not forced
	format := opts.ForceFormat
	if format == detector.FormatUnknown {
		var err error
		format, err = detector.DetectFormat(ctx, layer)
		if err != nil {
			return false, fmt.Errorf("failed to detect format: %w", err)
		}
	}

	if o.verbose {
		fmt.Printf("  Detected format: %s\n", format)
	}

	// Get layer URL and create remote reader
	reader, err := o.createLayerReader(layer)
	if err != nil {
		return false, fmt.Errorf("failed to create layer reader: %w", err)
	}
	defer func() { _ = reader.Close() }()

	// Extract based on format
	switch format {
	case detector.FormatEStargz:
		return o.extractEStargz(ctx, reader, opts)
	case detector.FormatSOCI:
		return o.extractSOCI(ctx, reader, layer, sociIndex, opts)
	case detector.FormatStandard:
		return o.extractStandard(ctx, reader, opts)
	default:
		return false, fmt.Errorf("unsupported format: %s", format)
	}
}

// extractEStargz extracts from an eStargz layer
func (o *Orchestrator) extractEStargz(ctx context.Context, reader io.ReaderAt, opts ExtractOptions) (bool, error) {
	// RemoteReader has a Size() method
	rr := reader.(*remote.RemoteReader)
	extractor := estargz.NewExtractor(reader, rr.Size())
	err := extractor.ExtractFile(ctx, opts.FilePath, opts.OutputPath)
	if err != nil {
		return false, err
	}
	return true, nil
}

// extractSOCI extracts from a SOCI-indexed layer
func (o *Orchestrator) extractSOCI(ctx context.Context, reader io.ReaderAt, layer v1.Layer, sociIndex *soci.IndexInfo, opts ExtractOptions) (bool, error) {
	if sociIndex == nil {
		return false, fmt.Errorf("no SOCI index available")
	}

	// Get the zTOC for this specific layer
	// This would involve looking up the layer's digest in the SOCI index
	// and fetching the corresponding zTOC blob
	// For now, this is a placeholder

	var ztocBlob []byte // Would be fetched from registry

	// RemoteReader has a Size() method
	rr := reader.(*remote.RemoteReader)
	extractor, err := soci.NewExtractor(reader, rr.Size(), ztocBlob)
	if err != nil {
		return false, fmt.Errorf("failed to create SOCI extractor: %w", err)
	}

	err = extractor.ExtractFile(ctx, opts.FilePath, opts.OutputPath)
	if err != nil {
		return false, err
	}
	return true, nil
}

// extractStandard extracts from a standard OCI layer
func (o *Orchestrator) extractStandard(ctx context.Context, reader io.ReaderAt, opts ExtractOptions) (bool, error) {
	// Standard extraction would involve:
	// 1. Decompressing the entire layer (or using streaming decompression)
	// 2. Reading the tar archive
	// 3. Finding the target file
	// 4. Extracting it

	// This is less efficient than eStargz/SOCI but works for any layer
	return false, fmt.Errorf("standard format extraction not yet implemented")
}

// createLayerReader creates a remote reader for a layer
func (o *Orchestrator) createLayerReader(layer v1.Layer) (*remote.RemoteReader, error) {
	// Get the layer's digest
	digest, err := layer.Digest()
	if err != nil {
		return nil, fmt.Errorf("failed to get layer digest: %w", err)
	}

	// In a real implementation, we'd construct the blob URL based on:
	// - The registry domain (from the image reference)
	// - The repository name
	// - The blob digest
	// Example: https://registry.example.com/v2/library/alpine/blobs/sha256:abc123...

	// For now, we'll try to use the layer's compressed reader
	// and convert it to a ReaderAt (which is not ideal but works as a fallback)

	// This is a placeholder - in production you'd construct the proper URL
	url := fmt.Sprintf("placeholder://%s", digest)

	return remote.NewRemoteReader(url)
}
