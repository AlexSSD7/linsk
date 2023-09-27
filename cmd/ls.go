// Linsk - A utility to access Linux-native file systems on non-Linux operating systems.
// Copyright (c) 2023 The Linsk Authors.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/AlexSSD7/linsk/share"
	"github.com/AlexSSD7/linsk/vm"
	"github.com/spf13/cobra"
)

var lsCmd = &cobra.Command{
	Use:   "ls",
	Short: "Start a VM and list all user drives within the VM. Uses lsblk command under the hood.",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		configureVMRuntimeFlags()

		os.Exit(runVM(args[0], func(ctx context.Context, i *vm.VM, fm *vm.FileManager, trc *share.NetTapRuntimeContext) int {
			if vmRuntimeLUKSContainerDevice != "" {
				err := fm.PreopenLUKSContainer(vmRuntimeLUKSContainerDevice)
				if err != nil {
					slog.Error("Failed to preopen LUKS container", "error", err.Error())
					return 1
				}
			}

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
		}, nil, false, false))
	},
}

func init() {
	initVMRuntimeFlags(lsCmd.Flags())
}
