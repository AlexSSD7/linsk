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
	"strings"

	"github.com/AlexSSD7/linsk/osspecifics"
	"github.com/AlexSSD7/linsk/share"
	"github.com/AlexSSD7/linsk/vm"
	"github.com/sethvargo/go-password/password"
	"github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Start a VM and expose an FTP file share.",
	Args:  cobra.ExactArgs(3),
	Run: func(cmd *cobra.Command, args []string) {
		vmMountDevName := args[1]
		fsType := args[2]

		newBackendFunc := share.GetBackend(shareBackendFlag)
		if newBackendFunc == nil {
			slog.Error("Unknown file share backend", "type", shareBackendFlag)
			os.Exit(1)
		}

		cfg, err := share.RawUserConfiguration{
			ListenIP: shareListenIPFlag,

			FTPExtIP:   ftpExtIPFlag,
			SMBExtMode: smbUseExternAddrFlag,
		}.Process(shareBackendFlag, slog.With("caller", "share-config"))
		if err != nil {
			slog.Error("Failed to process raw configuration", "error", err.Error())
			os.Exit(1)
		}

		backend, vmOpts, err := newBackendFunc(cfg)
		if err != nil {
			slog.Error("Failed to initialize share backend", "backend", shareBackendFlag, "error", err.Error())
			os.Exit(1)
		}

		if (luksFlag || luksContainerFlag != "") && !allowLUKSLowMemoryFlag {
			if vmMemAllocFlag < defaultMemAllocLUKS {
				if vmMemAllocFlag != defaultMemAlloc {
					slog.Warn("Enforcing minimum LUKS memory allocation. Please add --allow-luks-low-memory to disable this.", "min", vmMemAllocFlag, "specified", vmMemAllocFlag)
				}

				vmMemAllocFlag = defaultMemAllocLUKS
			}
		}

		os.Exit(runVM(args[0], func(ctx context.Context, i *vm.VM, fm *vm.FileManager, tapCtx *share.NetTapRuntimeContext) int {
			slog.Info("Mounting the device", "dev", vmMountDevName, "fs", fsType, "luks", luksFlag)

			err := fm.Mount(vmMountDevName, vm.MountOptions{
				LUKSContainerPreopen: luksContainerFlag,

				FSType: fsType,
				LUKS:   luksFlag,
			})
			if err != nil {
				slog.Error("Failed to mount the disk inside the VM", "error", err.Error())
				return 1
			}

			sharePWD, err := password.Generate(16, 10, 0, false, false)
			if err != nil {
				slog.Error("Failed to generate ephemeral password for the network file share", "error", err.Error())
				return 1
			}

			lg := slog.With("backend", shareBackendFlag)

			shareURI, err := backend.Apply(ctx, sharePWD, &share.VMShareContext{
				Instance:    i,
				FileManager: fm,
				NetTapCtx:   tapCtx,
			})
			if err != nil {
				lg.Error("Failed to apply (start) file share backend", "error", err.Error())
				return 1
			}

			lg.Info("Started the network share successfully")

			fmt.Fprintf(os.Stderr, "===========================\n[Network File Share Config]\nThe network file share was started. Please use the credentials below to connect to the file server.\n\nType: "+strings.ToUpper(shareBackendFlag)+"\nURL: %v\nUsername: linsk\nPassword: %v\n===========================\n", shareURI, sharePWD)

			ctxWait := true

			if debugShellFlag {
				slog.Warn("Starting a debug VM shell")
				err := runVMShell(ctx, i)
				if err != nil {
					slog.Error("Failed to run VM shell", "error", err.Error())
				} else {
					ctxWait = false
				}
			}

			if ctxWait {
				<-ctx.Done()
			}

			return 0
		}, vmOpts.Ports, unrestrictedNetworkingFlag, vmOpts.EnableTap))
	},
}

var (
	luksFlag               bool
	luksContainerFlag      string
	allowLUKSLowMemoryFlag bool
	shareListenIPFlag      string
	ftpExtIPFlag           string
	shareBackendFlag       string
	smbUseExternAddrFlag   bool
	debugShellFlag         bool
)

func init() {
	runCmd.Flags().BoolVarP(&luksFlag, "luks", "l", false, "Use cryptsetup to open a LUKS volume (password will be prompted).")
	runCmd.Flags().StringVar(&luksContainerFlag, "luks-container", "", `Specifies a device path (without "dev/" prefix) to preopen as a LUKS container (password will be prompted). Useful for accessing LVM partitions behind LUKS.`)
	runCmd.Flags().BoolVar(&allowLUKSLowMemoryFlag, "allow-luks-low-memory", false, "Allow VM memory allocation lower than 2048 MiB when LUKS is enabled.")
	runCmd.Flags().BoolVar(&debugShellFlag, "debug-shell", false, "Start a VM shell when the network file share is active.")

	var defaultShareType string
	switch {
	case osspecifics.IsWindows():
		defaultShareType = "smb"
	case osspecifics.IsMacOS():
		defaultShareType = "afp"
	default:
		defaultShareType = "ftp"
	}

	runCmd.Flags().StringVar(&shareBackendFlag, "share-backend", defaultShareType, `Specifies the file share backend to use. The default value is OS-specific. (available "smb", "afp", "ftp")`)
	runCmd.Flags().StringVar(&shareListenIPFlag, "share-listen", share.GetDefaultListenIPStr(), "Specifies the IP to bind the network share port to. NOTE: For FTP, changing the bind address is not enough to connect remotely. You should also specify --ftp-extip.")

	runCmd.Flags().StringVar(&ftpExtIPFlag, "ftp-extip", share.GetDefaultListenIPStr(), "Specifies the external IP the FTP server should advertise.")
	runCmd.Flags().BoolVar(&smbUseExternAddrFlag, "smb-extern", share.IsSMBExtModeDefault(), "Specifies whether Linsk emulate external networking for the VM's SMB server. This is the default for Windows as there is no way to specify ports in Windows SMB client.")
}
