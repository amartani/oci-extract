package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// completionCmd generates shell completion scripts.
var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish|powershell]",
	Short: "Generate shell completion scripts",
	Long: `Generate shell completion scripts for oci-extract.

To load completions:

Bash:
  $ source <(oci-extract completion bash)

  # To load completions for each session, execute once:
  # Linux:
  $ oci-extract completion bash > /etc/bash_completion.d/oci-extract
  # macOS:
  $ oci-extract completion bash > $(brew --prefix)/etc/bash_completion.d/oci-extract

Zsh:
  # If shell completion is not already enabled in your environment,
  # you will need to enable it. You can execute the following once:
  $ echo "autoload -U compinit; compinit" >> ~/.zshrc

  # To load completions for each session, execute once:
  $ oci-extract completion zsh > "${fpath[1]}/_oci-extract"

Fish:
  $ oci-extract completion fish | source

  # To load completions for each session, execute once:
  $ oci-extract completion fish > ~/.config/fish/completions/oci-extract.fish

PowerShell:
  PS> oci-extract completion powershell | Out-String | Invoke-Expression

  # To load completions for every new session, run:
  PS> oci-extract completion powershell > oci-extract.ps1
  # and source this file from your PowerShell profile.
`,
	Args:      cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
	ValidArgs: []string{"bash", "zsh", "fish", "powershell"},
	RunE: func(cmd *cobra.Command, args []string) error {
		switch args[0] {
		case "bash":
			return rootCmd.GenBashCompletion(cmd.OutOrStdout())
		case "zsh":
			return rootCmd.GenZshCompletion(cmd.OutOrStdout())
		case "fish":
			return rootCmd.GenFishCompletion(cmd.OutOrStdout(), true)
		case "powershell":
			return rootCmd.GenPowerShellCompletionWithDesc(cmd.OutOrStdout())
		default:
			return fmt.Errorf("unsupported shell %q: must be one of bash, zsh, fish, powershell", args[0])
		}
	},
}

func init() {
	rootCmd.AddCommand(completionCmd)
}
