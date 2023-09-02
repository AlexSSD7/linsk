package cmd

import (
	"log/slog"
	"os"

	"github.com/spf13/cobra"
)

var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "Build (set up) a VM image for local use. This needs to be run after the initial installation.",
	Run: func(cmd *cobra.Command, args []string) {
		store := createStoreOrExit()

		exitCode := store.RunCLIImageBuild(vmDebugFlag, buildOverwriteFlag)
		if exitCode != 0 {
			os.Exit(exitCode)
		}

		slog.Info("VM image built successfully", "path", store.GetVMImagePath())
	},
}

var buildOverwriteFlag bool

func init() {
	buildCmd.Flags().BoolVar(&buildOverwriteFlag, "overwrite", false, "Specifies whether the VM image should be overwritten with the build.")
}
