package cmd

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/amartani/oci-extract/internal/detector"
	"github.com/amartani/oci-extract/internal/estargz"
	"github.com/amartani/oci-extract/internal/registry"
	"github.com/amartani/oci-extract/internal/remote"
	"github.com/amartani/oci-extract/internal/standard"
	"github.com/spf13/cobra"
)

var (
	outputPath string
	format     string
)

// extractCmd represents the extract command
var extractCmd = &cobra.Command{
	Use:   "extract <image> <file-path>",
	Short: "Extract a file from an OCI image",
	Long: `Extract a specific file or directory from an OCI image without mounting it.

The command automatically detects the image format (standard, eStargz, or SOCI)
and uses the most efficient method to extract the requested file.

Examples:
  # Extract a binary from an image
  oci-extract extract alpine:latest /bin/sh -o ./sh

  # Extract a config file
  oci-extract extract nginx:latest /etc/nginx/nginx.conf -o ./nginx.conf

  # Force using a specific format
  oci-extract extract myimage:latest /app/data --format estargz -o ./data`,
	Args: cobra.ExactArgs(2),
	RunE: runExtract,
}

func init() {
	rootCmd.AddCommand(extractCmd)

	extractCmd.Flags().StringVarP(&outputPath, "output", "o", "", "Output path (default: current directory + filename)")
	extractCmd.Flags().StringVar(&format, "format", "auto", "Force format: auto, estargz, soci, standard")
}

func runExtract(cmd *cobra.Command, args []string) error {
	imageRef := args[0]
	filePath := args[1]

	ctx := context.Background()

	// Determine output path
	if outputPath == "" {
		outputPath = filepath.Base(filePath)
	}

	verbose, _ := cmd.Flags().GetBool("verbose")
	if verbose {
		fmt.Printf("Extracting %s from %s\n", filePath, imageRef)
		fmt.Printf("Output: %s\n", outputPath)
	}

	// Create registry client
	client := registry.NewClient()

	// Get enhanced layers with blob URLs
	enhancedLayers, err := client.GetEnhancedLayers(ctx, imageRef)
	if err != nil {
		return fmt.Errorf("failed to get image layers: %w", err)
	}

	if verbose {
		fmt.Printf("Found %d layers\n", len(enhancedLayers))
	}

	// Try to extract from each layer (bottom-up)
	for i := len(enhancedLayers) - 1; i >= 0; i-- {
		layerInfo := enhancedLayers[i]

		if verbose {
			fmt.Printf("Checking layer %s...\n", layerInfo.Digest)
		}

		// Detect format and extract
		extracted, err := extractFromLayer(ctx, layerInfo, filePath, outputPath, format, verbose)
		if err != nil {
			if verbose {
				fmt.Printf("  Error: %v\n", err)
			}
			continue
		}

		if extracted {
			fmt.Printf("Successfully extracted %s to %s\n", filePath, outputPath)
			return nil
		}
	}

	return fmt.Errorf("file %s not found in any layer of image %s", filePath, imageRef)
}

func extractFromLayer(ctx context.Context, layerInfo *registry.EnhancedLayerInfo, filePath, outputPath, formatHint string, verbose bool) (bool, error) {
	// Detect format if auto
	formatToUse := formatHint
	if formatHint == "auto" {
		detectedFormat, err := detector.DetectFormat(ctx, layerInfo.Layer)
		if err != nil {
			if verbose {
				fmt.Printf("  Format detection failed: %v, trying eStargz anyway\n", err)
			}
			detectedFormat = detector.FormatEStargz
		}
		formatToUse = detectedFormat.String()
		if verbose {
			fmt.Printf("  Detected format: %s\n", formatToUse)
		}
	}

	// Try eStargz extraction
	if formatToUse == "auto" || formatToUse == "estargz" {
		if verbose {
			fmt.Println("  Trying eStargz format...")
		}

		// Create RemoteReader for the layer
		reader, err := remote.NewRemoteReader(layerInfo.BlobURL)
		if err != nil {
			return false, fmt.Errorf("failed to create remote reader: %w", err)
		}
		defer func() { _ = reader.Close() }()

		// Create eStargz extractor
		extractor := estargz.NewExtractor(reader, layerInfo.Size)

		// Try to extract the file
		err = extractor.ExtractFile(ctx, filePath, outputPath)
		if err == nil {
			// Success!
			return true, nil
		}

		if verbose {
			fmt.Printf("  eStargz extraction failed: %v\n", err)
		}

		// If it's not a "file not found" error, return the error
		// Otherwise, continue trying other formats
	}

	// Try standard layer extraction as fallback
	if formatToUse == "auto" || formatToUse == "standard" {
		if verbose {
			fmt.Println("  Trying standard format...")
		}

		// Create standard extractor
		// This downloads and decompresses the entire layer
		extractor := standard.NewExtractor(layerInfo.Layer)

		// Try to extract the file
		err := extractor.ExtractFile(ctx, filePath, outputPath)
		if err == nil {
			// Success!
			return true, nil
		}

		if verbose {
			fmt.Printf("  Standard extraction failed: %v\n", err)
		}

		// Return the error if it's not a "file not found" error
		// Otherwise, continue to next layer
	}

	return false, nil
}
