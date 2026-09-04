package cli

import (
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jparkerweb/azw3-to-pdf/internal/engine"
	"github.com/jparkerweb/azw3-to-pdf/internal/pdfout"
	"github.com/jparkerweb/azw3-to-pdf/internal/presets"
)

// completeBookFiles offers Kindle books and directories for path arguments.
func completeBookFiles(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	exts := make([]string, 0, len(engine.BookExtensions))
	for _, e := range engine.BookExtensions {
		exts = append(exts, strings.TrimPrefix(e, "."))
	}
	return exts, cobra.ShellCompDirectiveFilterFileExt
}

func registerCompletions(cmd *cobra.Command) {
	_ = cmd.RegisterFlagCompletionFunc("preset", func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		var out []string
		for _, p := range presets.All() {
			out = append(out, p.Key+"\t"+p.Description)
		}
		return out, cobra.ShellCompDirectiveNoFileComp
	})
	_ = cmd.RegisterFlagCompletionFunc("page-size", func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		var out []string
		for _, s := range pdfout.PageSizes() {
			out = append(out, s.Name+"\t"+s.Label)
		}
		return out, cobra.ShellCompDirectiveNoFileComp
	})
	_ = cmd.RegisterFlagCompletionFunc("font", func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return []string{
			"serif\tsystem serif face, falling back to the embedded Go font",
			"sans\tsystem sans face",
			"mono\tsystem monospaced face",
		}, cobra.ShellCompDirectiveDefault
	})
	_ = cmd.RegisterFlagCompletionFunc("input", completeBookFiles)
	_ = cmd.RegisterFlagCompletionFunc("output-dir", func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return nil, cobra.ShellCompDirectiveFilterDirs
	})
}

func completionCmd() *cobra.Command {
	return &cobra.Command{
		Use:       "completion [bash|zsh|fish|powershell]",
		Short:     "Print a shell completion script",
		Args:      cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
		ValidArgs: []string{"bash", "zsh", "fish", "powershell"},
		RunE: func(cmd *cobra.Command, args []string) error {
			root := cmd.Root()
			switch args[0] {
			case "bash":
				return root.GenBashCompletionV2(os.Stdout, true)
			case "zsh":
				return root.GenZshCompletion(os.Stdout)
			case "fish":
				return root.GenFishCompletion(os.Stdout, true)
			default:
				return root.GenPowerShellCompletionWithDesc(os.Stdout)
			}
		},
	}
}
