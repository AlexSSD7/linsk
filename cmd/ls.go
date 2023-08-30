package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/AlexSSD7/linsk/vm"
	"github.com/spf13/cobra"
)

var lsCmd = &cobra.Command{
	Use:   "ls",
	Short: "Start a VM and list all user drives within the VM. Uses lsblk command under the hood.",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		os.Exit(runVM(args[0], func(ctx context.Context, i *vm.VM, fm *vm.FileManager) int {
			lsblkOut, err := fm.Lsblk()
			if err != nil {
				slog.Error("Failed to list block devices in the VM", "error", err.Error())
				return 1
			}

			if len(lsblkOut) == 0 {
				fmt.Printf("<empty lsblk output>\n")
			} else {
				fmt.Print(string(lsblkOut))
			}

			return 0
		}, nil, false))
	},
}
