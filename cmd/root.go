package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	// Version information
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

// rootCmd represents the base command
var rootCmd = &cobra.Command{
	Use:   "oci-extract",
	Short: "Extract files from OCI images without mounting",
	Long: `oci-extract is a CLI tool that extracts specific files or folders from
remote OCI images without requiring root privileges or mounting the image.

It supports multiple OCI image formats:
  - Standard OCI/Docker images
  - eStargz (seekable tar.gz with TOC)
  - SOCI (Seekable OCI with zTOC indices)

The tool uses HTTP Range requests to fetch only the necessary bytes,
making it efficient for extracting small files from large images.`,
	Version: fmt.Sprintf("%s (commit: %s, built: %s)", version, commit, date),
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	// Global flags
	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "Enable verbose output")
	rootCmd.PersistentFlags().BoolP("debug", "d", false, "Enable debug output")
}
