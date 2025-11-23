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
	"github.com/amartani/oci-extract/internal/zstd"
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
	ImageRef    string
	FilePath    string
	OutputPath  string
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

// ListOptions contains options for listing files
type ListOptions struct {
	ImageRef    string
	ForceFormat detector.Format
}

// List lists all files in an OCI image
func (o *Orchestrator) List(ctx context.Context, opts ListOptions) ([]string, error) {
	// Get enhanced image layers with blob URLs
	enhancedLayers, err := o.client.GetEnhancedLayers(ctx, opts.ImageRef)
	if err != nil {
		return nil, fmt.Errorf("failed to get image layers: %w", err)
	}

	if o.verbose {
		fmt.Printf("Found %d layers in image\n", len(enhancedLayers))
	}

	var allFiles []string

	// List files from each layer (bottom-up, as layers are applied in order)
	for i := len(enhancedLayers) - 1; i >= 0; i-- {
		layerInfo := enhancedLayers[i]

		if o.verbose {
			fmt.Printf("Listing files in layer %s...\n", layerInfo.Digest)
		}

		// List files from this layer
		files, err := o.listFromLayer(ctx, layerInfo, opts)
		if err != nil {
			if o.verbose {
				fmt.Printf("  Failed to list files: %v\n", err)
			}
			continue
		}

		// Add files to the list (avoiding duplicates from upper layers)
		fileSet := make(map[string]bool)
		for _, f := range allFiles {
			fileSet[f] = true
		}
		for _, f := range files {
			if !fileSet[f] {
				allFiles = append(allFiles, f)
			}
		}
	}

	return allFiles, nil
}

// listFromLayer lists files from a single layer
func (o *Orchestrator) listFromLayer(ctx context.Context, layerInfo *registry.EnhancedLayerInfo, opts ListOptions) ([]string, error) {
	// Detect format if not forced
	format := opts.ForceFormat
	if format == detector.FormatUnknown {
		var err error
		format, err = detector.DetectFormat(ctx, layerInfo.Layer)
		if err != nil {
			if o.verbose {
				fmt.Printf("  Format detection failed: %v, defaulting to standard\n", err)
			}
			format = detector.FormatStandard
		}
	}

	if o.verbose {
		fmt.Printf("  Detected format: %s\n", format)
	}

	// Try eStargz listing
	if format == detector.FormatUnknown || format == detector.FormatEStargz {
		if o.verbose {
			fmt.Println("  Trying eStargz format...")
		}

		files, err := o.listEStargz(ctx, layerInfo)
		if err == nil {
			return files, nil
		}

		if o.verbose && err != nil {
			fmt.Printf("  eStargz listing failed: %v\n", err)
		}
	}

	// Try SOCI listing (requires index discovery first)
	if format == detector.FormatUnknown || format == detector.FormatSOCI {
		if o.verbose {
			fmt.Println("  Trying SOCI format...")
		}

		sociIndex, err := soci.DiscoverSOCIIndex(ctx, opts.ImageRef)
		if err == nil && sociIndex != nil {
			files, err := o.listSOCI(ctx, layerInfo, sociIndex)
			if err == nil {
				return files, nil
			}

			if o.verbose && err != nil {
				fmt.Printf("  SOCI listing failed: %v\n", err)
			}
		}
	}

	// Try zstd:chunked listing
	if format == detector.FormatUnknown || format == detector.FormatZstd || format == detector.FormatZstdChunked {
		if o.verbose {
			fmt.Println("  Trying zstd:chunked format...")
		}

		files, err := o.listZstdChunked(ctx, layerInfo)
		if err == nil {
			return files, nil
		}

		if o.verbose && err != nil {
			fmt.Printf("  zstd:chunked listing failed: %v\n", err)
		}
	}

	// Try zstd listing
	if format == detector.FormatUnknown || format == detector.FormatZstd {
		if o.verbose {
			fmt.Println("  Trying zstd format...")
		}

		files, err := o.listZstd(ctx, layerInfo)
		if err == nil {
			return files, nil
		}

		if o.verbose && err != nil {
			fmt.Printf("  zstd listing failed: %v\n", err)
		}
	}

	// Try standard listing as fallback
	if o.verbose {
		fmt.Println("  Using standard format...")
	}

	files, err := o.listStandard(ctx, layerInfo)
	if err != nil {
		return nil, err
	}

	return files, nil
}

// listEStargz lists files from an eStargz layer
func (o *Orchestrator) listEStargz(ctx context.Context, layerInfo *registry.EnhancedLayerInfo) ([]string, error) {
	// Create RemoteReader for the layer using its blob URL
	reader, err := remote.NewRemoteReader(layerInfo.BlobURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create remote reader: %w", err)
	}
	defer func() { _ = reader.Close() }()

	// Create eStargz extractor
	extractor := estargz.NewExtractor(reader, layerInfo.Size)

	// List files
	files, err := extractor.ListFiles(ctx)
	if err != nil {
		return nil, err
	}

	return files, nil
}

// listSOCI lists files from a SOCI-indexed layer
func (o *Orchestrator) listSOCI(ctx context.Context, layerInfo *registry.EnhancedLayerInfo, sociIndex *soci.IndexInfo) ([]string, error) {
	// Get the zTOC for this specific layer
	ztocBlob, err := soci.GetZtocForLayer(ctx, sociIndex, layerInfo.Digest)
	if err != nil {
		return nil, fmt.Errorf("failed to get zTOC for layer: %w", err)
	}

	// Create RemoteReader for the layer using its blob URL
	reader, err := remote.NewRemoteReader(layerInfo.BlobURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create remote reader: %w", err)
	}
	defer func() { _ = reader.Close() }()

	// Create SOCI extractor
	extractor, err := soci.NewExtractor(reader, layerInfo.Size, ztocBlob)
	if err != nil {
		return nil, fmt.Errorf("failed to create SOCI extractor: %w", err)
	}

	// List files
	files := extractor.ListFiles()
	return files, nil
}

// listStandard lists files from a standard OCI layer
func (o *Orchestrator) listStandard(ctx context.Context, layerInfo *registry.EnhancedLayerInfo) ([]string, error) {
	// Create standard extractor
	extractor := standard.NewExtractor(layerInfo.Layer)

	// List files
	files, err := extractor.ListFiles(ctx)
	if err != nil {
		return nil, err
	}

	return files, nil
}

// listZstd lists files from a zstd-compressed OCI layer
func (o *Orchestrator) listZstd(ctx context.Context, layerInfo *registry.EnhancedLayerInfo) ([]string, error) {
	// Create zstd extractor
	extractor := zstd.NewExtractor(layerInfo.Layer)

	// List files
	files, err := extractor.ListFiles(ctx)
	if err != nil {
		return nil, err
	}

	return files, nil
}

// listZstdChunked lists files from a zstd:chunked layer
func (o *Orchestrator) listZstdChunked(ctx context.Context, layerInfo *registry.EnhancedLayerInfo) ([]string, error) {
	// Create RemoteReader for the layer using its blob URL
	reader, err := remote.NewRemoteReader(layerInfo.BlobURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create remote reader: %w", err)
	}
	defer func() { _ = reader.Close() }()

	// Create zstd:chunked extractor
	extractor := zstd.NewChunkedExtractor(reader, layerInfo.Size)

	// List files
	files, err := extractor.ListFiles(ctx)
	if err != nil {
		return nil, err
	}

	return files, nil
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

	// Try zstd:chunked extraction
	if format == detector.FormatUnknown || format == detector.FormatZstd || format == detector.FormatZstdChunked {
		if o.verbose {
			fmt.Println("  Trying zstd:chunked format...")
		}

		extracted, err := o.extractZstdChunked(ctx, layerInfo, opts)
		if err == nil && extracted {
			return true, nil
		}

		if o.verbose && err != nil {
			fmt.Printf("  zstd:chunked extraction failed: %v\n", err)
		}
	}

	// Try zstd extraction
	if format == detector.FormatUnknown || format == detector.FormatZstd {
		if o.verbose {
			fmt.Println("  Trying zstd format...")
		}

		extracted, err := o.extractZstd(ctx, layerInfo, opts)
		if err == nil && extracted {
			return true, nil
		}

		if o.verbose && err != nil {
			fmt.Printf("  zstd extraction failed: %v\n", err)
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
	ztocBlob, err := soci.GetZtocForLayer(ctx, sociIndex, layerInfo.Digest)
	if err != nil {
		return false, fmt.Errorf("failed to get zTOC for layer: %w", err)
	}

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

// extractZstd extracts from a zstd-compressed OCI layer
func (o *Orchestrator) extractZstd(ctx context.Context, layerInfo *registry.EnhancedLayerInfo, opts ExtractOptions) (bool, error) {
	// Create zstd extractor
	extractor := zstd.NewExtractor(layerInfo.Layer)

	// Try to extract the file
	err := extractor.ExtractFile(ctx, opts.FilePath, opts.OutputPath)
	if err != nil {
		return false, err
	}

	return true, nil
}

// extractZstdChunked extracts from a zstd:chunked layer
func (o *Orchestrator) extractZstdChunked(ctx context.Context, layerInfo *registry.EnhancedLayerInfo, opts ExtractOptions) (bool, error) {
	// Create RemoteReader for the layer using its blob URL
	reader, err := remote.NewRemoteReader(layerInfo.BlobURL)
	if err != nil {
		return false, fmt.Errorf("failed to create remote reader: %w", err)
	}
	defer func() { _ = reader.Close() }()

	// Create zstd:chunked extractor
	extractor := zstd.NewChunkedExtractor(reader, layerInfo.Size)

	// Try to extract the file
	err = extractor.ExtractFile(ctx, opts.FilePath, opts.OutputPath)
	if err != nil {
		return false, err
	}

	return true, nil
}
