package cmd

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/AlexSSD7/linsk/vm"
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

		// TODO: `slog` library prints entire stack traces for errors which makes reading errors challenging.

		runVM(args[0], func(ctx context.Context, i *vm.Instance, fm *vm.FileManager) {
			err := fm.Mount(vmMountDevName, vm.MountOptions{
				FSType: fsType,
				LUKS:   luksFlag,
			})
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

var luksFlag bool

func init() {
	runCmd.Flags().BoolVarP(&luksFlag, "luks", "l", false, "Use cryptsetup to open a LUKS volume (password will be prompted)")
}
