package extractor

import (
	"context"
	"fmt"

	"github.com/amartani/oci-extract/internal/detector"
	"github.com/amartani/oci-extract/internal/estargz"
	"github.com/amartani/oci-extract/internal/registry"
	"github.com/amartani/oci-extract/internal/remote"
	"github.com/amartani/oci-extract/internal/soci"
	"github.com/amartani/oci-extract/internal/standard"
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
	// Get enhanced image layers with blob URLs
	enhancedLayers, err := o.client.GetEnhancedLayers(ctx, opts.ImageRef)
	if err != nil {
		return fmt.Errorf("failed to get image layers: %w", err)
	}

	if o.verbose {
		fmt.Printf("Found %d layers in image\n", len(enhancedLayers))
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
	for i := len(enhancedLayers) - 1; i >= 0; i-- {
		layerInfo := enhancedLayers[i]

		if o.verbose {
			fmt.Printf("Checking layer %s...\n", layerInfo.Digest)
		}

		// Try extraction
		extracted, err := o.extractFromLayer(ctx, layerInfo, sociIndex, opts)
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
func (o *Orchestrator) extractFromLayer(ctx context.Context, layerInfo *registry.EnhancedLayerInfo, sociIndex *soci.IndexInfo, opts ExtractOptions) (bool, error) {
	// Detect format if not forced
	format := opts.ForceFormat
	if format == detector.FormatUnknown {
		var err error
		format, err = detector.DetectFormat(ctx, layerInfo.Layer)
		if err != nil {
			if o.verbose {
				fmt.Printf("  Format detection failed: %v, trying eStargz anyway\n", err)
			}
			format = detector.FormatEStargz
		}
	}

	if o.verbose {
		fmt.Printf("  Detected format: %s\n", format)
	}

	// Try eStargz extraction
	if format == detector.FormatUnknown || format == detector.FormatEStargz {
		if o.verbose {
			fmt.Println("  Trying eStargz format...")
		}

		extracted, err := o.extractEStargz(ctx, layerInfo, opts)
		if err == nil && extracted {
			return true, nil
		}

		if o.verbose && err != nil {
			fmt.Printf("  eStargz extraction failed: %v\n", err)
		}
	}

	// Try SOCI extraction if index is available
	if (format == detector.FormatUnknown || format == detector.FormatSOCI) && sociIndex != nil {
		if o.verbose {
			fmt.Println("  Trying SOCI format...")
		}

		extracted, err := o.extractSOCI(ctx, layerInfo, sociIndex, opts)
		if err == nil && extracted {
			return true, nil
		}

		if o.verbose && err != nil {
			fmt.Printf("  SOCI extraction failed: %v\n", err)
		}
	}

	// Try standard extraction as fallback
	if format == detector.FormatUnknown || format == detector.FormatStandard {
		if o.verbose {
			fmt.Println("  Trying standard format...")
		}

		extracted, err := o.extractStandard(ctx, layerInfo, opts)
		if err == nil && extracted {
			return true, nil
		}

		if o.verbose && err != nil {
			fmt.Printf("  Standard extraction failed: %v\n", err)
		}
	}

	return false, nil
}

// extractEStargz extracts from an eStargz layer
func (o *Orchestrator) extractEStargz(ctx context.Context, layerInfo *registry.EnhancedLayerInfo, opts ExtractOptions) (bool, error) {
	// Create RemoteReader for the layer using its blob URL
	reader, err := remote.NewRemoteReader(layerInfo.BlobURL)
	if err != nil {
		return false, fmt.Errorf("failed to create remote reader: %w", err)
	}
	defer func() { _ = reader.Close() }()

	// Create eStargz extractor
	extractor := estargz.NewExtractor(reader, layerInfo.Size)

	// Try to extract the file
	err = extractor.ExtractFile(ctx, opts.FilePath, opts.OutputPath)
	if err != nil {
		return false, err
	}
	return true, nil
}

// extractSOCI extracts from a SOCI-indexed layer
func (o *Orchestrator) extractSOCI(ctx context.Context, layerInfo *registry.EnhancedLayerInfo, sociIndex *soci.IndexInfo, opts ExtractOptions) (bool, error) {
	if sociIndex == nil {
		return false, fmt.Errorf("no SOCI index available")
	}

	// Create RemoteReader for the layer using its blob URL
	reader, err := remote.NewRemoteReader(layerInfo.BlobURL)
	if err != nil {
		return false, fmt.Errorf("failed to create remote reader: %w", err)
	}
	defer func() { _ = reader.Close() }()

	// Get the zTOC for this specific layer
	// This would involve looking up the layer's digest in the SOCI index
	// and fetching the corresponding zTOC blob
	// For now, this is a placeholder
	var ztocBlob []byte // Would be fetched from registry

	// Create SOCI extractor
	extractor, err := soci.NewExtractor(reader, layerInfo.Size, ztocBlob)
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
func (o *Orchestrator) extractStandard(ctx context.Context, layerInfo *registry.EnhancedLayerInfo, opts ExtractOptions) (bool, error) {
	// Create standard extractor
	// This downloads and decompresses the entire layer
	extractor := standard.NewExtractor(layerInfo.Layer)

	// Try to extract the file
	err := extractor.ExtractFile(ctx, opts.FilePath, opts.OutputPath)
	if err != nil {
		return false, err
	}

	return true, nil
}
