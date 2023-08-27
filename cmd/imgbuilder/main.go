package main

import (
	"log/slog"
	"os"
	"path/filepath"

	"github.com/AlexSSD7/linsk/cmd/imgbuilder/builder"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use: "imgbuilder",
	// TODO: Fill this
	// Short:        "",
	// Long:         ``,
	Args: cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		baseISOPath := filepath.Clean(args[0])
		outImagePath := filepath.Clean(args[1])

		bc, err := builder.NewBuildContext(slog.With("caller", "build-context"), baseISOPath, outImagePath, vmDebugFlag)
		if err != nil {
			slog.Error("Failed to create a new build context", "error", err)
			os.Exit(1)
		}

		err = bc.BuildWithInterruptHandler()
		if err != nil {
			slog.Error("Failed to build an image", "error", err)
			os.Exit(1)
		}

		slog.Info("Success")
	},
}

var vmDebugFlag bool

func init() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, nil)))

	rootCmd.PersistentFlags().BoolVar(&vmDebugFlag, "vmdebug", false, "Enable VM debug mode. This will open an accessible VM monitor. You can log in with root user and no password.")
}

func main() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}
