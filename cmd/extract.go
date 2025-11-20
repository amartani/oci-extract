package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/amartani/oci-extract/internal/estargz"
	"github.com/amartani/oci-extract/internal/registry"
	"github.com/amartani/oci-extract/internal/remote"
	"github.com/amartani/oci-extract/internal/soci"
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

	// Get image layers
	layers, err := client.GetLayers(ctx, imageRef)
	if err != nil {
		return fmt.Errorf("failed to get image layers: %w", err)
	}

	if verbose {
		fmt.Printf("Found %d layers\n", len(layers))
	}

	// Try to extract from each layer (bottom-up)
	for i := len(layers) - 1; i >= 0; i-- {
		layer := layers[i]

		if verbose {
			digest, _ := layer.Digest()
			fmt.Printf("Checking layer %s...\n", digest)
		}

		// Detect format and extract
		extracted, err := extractFromLayer(ctx, layer, filePath, outputPath, format, verbose)
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

func extractFromLayer(ctx context.Context, layer interface{}, filePath, outputPath, formatHint string, verbose bool) (bool, error) {
	// This is where we'd implement the format detection and extraction logic
	// For now, let's try eStargz first

	if formatHint == "auto" || formatHint == "estargz" {
		// Try eStargz extraction
		if verbose {
			fmt.Println("  Trying eStargz format...")
		}

		// Get layer as compressed blob
		// In a real implementation, we'd create a RemoteReader from the layer URL
		// For now, this is a placeholder

		// Example of how it would work:
		// reader, err := remote.NewRemoteReader(layerURL)
		// if err != nil {
		//     return false, err
		// }
		// defer reader.Close()
		//
		// extractor := estargz.NewExtractor(reader)
		// err = extractor.ExtractFile(ctx, filePath, outputPath)
		// if err == nil {
		//     return true, nil
		// }
	}

	if formatHint == "auto" || formatHint == "soci" {
		// Try SOCI extraction
		if verbose {
			fmt.Println("  Trying SOCI format...")
		}

		// SOCI extraction would follow a similar pattern
	}

	return false, fmt.Errorf("extraction not implemented for this format")
}

// Helper function to create a remote reader from a layer
func createRemoteReader(layer interface{}) (*remote.RemoteReader, error) {
	// This would construct the proper blob URL based on the registry and digest
	// For now, this is a placeholder
	return nil, fmt.Errorf("not implemented")
}
