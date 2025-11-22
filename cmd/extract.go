package cmd

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/amartani/oci-extract/internal/detector"
	"github.com/amartani/oci-extract/internal/extractor"
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

	// Parse format hint
	var formatHint detector.Format
	switch format {
	case "estargz":
		formatHint = detector.FormatEStargz
	case "soci":
		formatHint = detector.FormatSOCI
	case "standard":
		formatHint = detector.FormatStandard
	default:
		formatHint = detector.FormatUnknown // Auto-detect
	}

	// Create orchestrator
	orch := extractor.NewOrchestrator(verbose)

	// Extract the file
	err := orch.Extract(ctx, extractor.ExtractOptions{
		ImageRef:    imageRef,
		FilePath:    filePath,
		OutputPath:  outputPath,
		ForceFormat: formatHint,
	})
	if err != nil {
		return err
	}

	fmt.Printf("Successfully extracted %s to %s\n", filePath, outputPath)
	return nil
}
