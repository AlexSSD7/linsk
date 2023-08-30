package cmd

import (
	"bufio"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var cleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Remove all downloaded VM images.",
	Run: func(cmd *cobra.Command, args []string) {
		store := createStore()

		fmt.Fprintf(os.Stderr, "Will delete all VM images in the data directory. Proceed? (y/n) > ")

		reader := bufio.NewReader(os.Stdin)
		answer, err := reader.ReadBytes('\n')
		if err != nil {
			slog.Error("Failed to read answer", "error", err.Error())
			os.Exit(1)
		}

		if strings.ToLower(string(answer)) != "y\n" {
			fmt.Fprintf(os.Stderr, "Aborted.\n")
			os.Exit(2)
		}

		deleted, err := store.CleanImages(false)
		if err != nil {
			slog.Error("Failed to clean images", "error", err.Error())
			os.Exit(1)
		}

		slog.Info("Successful VM image cleanup", "deleted", deleted)
	},
}
