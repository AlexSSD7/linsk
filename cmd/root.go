package cmd

import (
	"os"

	"log/slog"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use: "linsk",
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

var vmDebugFlag bool

func init() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, nil)))

	rootCmd.AddCommand(lsCmd)
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(shellCmd)

	rootCmd.PersistentFlags().BoolVar(&vmDebugFlag, "vmdebug", false, "Enable VM debug mode. This will open an accessible VM monitor. You can log in with root user and no password.")
}
