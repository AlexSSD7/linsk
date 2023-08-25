package cmd

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/AlexSSD7/vldisk/vm"
	"github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
	Use: "run",
	// TODO: Fill this
	// Short: "",
	Args: cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		vmMountDevName := args[1]
		fsType := args[2]

		runVM(args[0], func(ctx context.Context, i *vm.Instance, fm *vm.FileManager) {
			err := fm.Mount(vmMountDevName, vm.MountOptions{FSType: fsType})
			if err != nil {
				slog.Error("Failed to mount the disk inside the VM", "error", err)
				return
			}

			fmt.Println("Mounted! Now sleeping")
			<-ctx.Done()
		})

		return nil
	},
}
