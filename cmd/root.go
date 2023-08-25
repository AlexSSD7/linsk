package cmd

import (
	"os"

	"log/slog"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use: "vldisk",
	// TODO: Fill this
	// Short:        "",
	// Long:         ``,
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, nil)))

	rootCmd.AddCommand(lsCmd)
	rootCmd.AddCommand(runCmd)
}
