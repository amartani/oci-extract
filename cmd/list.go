package cmd

import (
	"context"
	"fmt"

	"github.com/amartani/oci-extract/internal/detector"
	"github.com/amartani/oci-extract/internal/extractor"
	"github.com/spf13/cobra"
)

// listCmd represents the list command
var listCmd = &cobra.Command{
	Use:   "list <image>",
	Short: "List all files in an OCI image",
	Long: `List all files in an OCI image without downloading the entire image.

The command automatically detects the image format (standard, eStargz, or SOCI)
and uses the most efficient method to list files.

Examples:
  # List all files in an image
  oci-extract list alpine:latest

  # List files with verbose output
  oci-extract list alpine:latest --verbose

  # Force using a specific format
  oci-extract list myimage:latest --format estargz`,
	Args: cobra.ExactArgs(1),
	RunE: runList,
}

func init() {
	rootCmd.AddCommand(listCmd)

	listCmd.Flags().StringVar(&format, "format", "auto", "Force format: auto, estargz, soci, standard")
}

func runList(cmd *cobra.Command, args []string) error {
	imageRef := args[0]
	ctx := context.Background()

	verbose, _ := cmd.Flags().GetBool("verbose")
	if verbose {
		fmt.Printf("Listing files in %s\n", imageRef)
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

	// List files
	files, err := orch.List(ctx, extractor.ListOptions{
		ImageRef:    imageRef,
		ForceFormat: formatHint,
	})
	if err != nil {
		return err
	}

	// Print the list of files
	for _, file := range files {
		fmt.Println(file)
	}

	if verbose {
		fmt.Printf("\nTotal files: %d\n", len(files))
	}

	return nil
}
