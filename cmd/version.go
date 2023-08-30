package cmd

import (
	"fmt"
	"runtime"

	"github.com/AlexSSD7/linsk/constants"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show Linsk version.",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Linsk %v %v/%v %v", constants.Version, runtime.GOOS, runtime.GOARCH, runtime.Version())
	},
}
