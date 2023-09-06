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
	"fmt"
	"os"
	"path/filepath"

	"log/slog"

	"github.com/AlexSSD7/linsk/osspecifics"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "linsk",
	Short: "Access Linux-native file system infrastructure (including LVM and LUKS) on macOS and Linux. Powered by a lightweight Alpine Linux VM and FTP.",
	Long: `Linsk is a utility that allows you to access Linux-native file system infrastructure, including device mapping technologies like LVM and LUKS without compromise on other operating systems that have little ` +
		`to no support for Linux's wide range of file systems, mainly aiming macOS and Windows. Linsk does not reimplement any file system. Instead, Linsk ` +
		`utilizes a lightweight Alpine Linux VM to tap into the native Linux software ecosystem. The files are then exposed to the host via fast and widely-supported FTP, ` +
		`operating at near-hardware speeds.`,
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

var (
	vmDebugFlag                bool
	unrestrictedNetworkingFlag bool
	vmMemAllocFlag             uint32
	vmSSHSetupTimeoutFlag      uint32
	vmOSUpTimeoutFlag          uint32
	dataDirFlag                string
)

const (
	defaultMemAlloc     = 512
	defaultMemAllocLUKS = 2048
)

func init() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, nil)))

	rootCmd.AddCommand(lsCmd)
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(shellCmd)
	rootCmd.AddCommand(cleanCmd)
	rootCmd.AddCommand(buildCmd)
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(copyrightCmd)

	rootCmd.PersistentFlags().BoolVar(&vmDebugFlag, "vm-debug", false, "Enables the VM debug mode. This will open an accessible VM monitor and enable direct QEMU command log passthrough. You can log in with root user and no password.")
	rootCmd.PersistentFlags().BoolVar(&unrestrictedNetworkingFlag, "vm-unrestricted-networking", false, "Enables unrestricted networking. This will allow the VM to connect to the internet.")
	rootCmd.PersistentFlags().Uint32Var(&vmMemAllocFlag, "vm-mem-alloc", defaultMemAlloc, fmt.Sprintf("Specifies the VM memory allocation in KiB. (the default is %v in LUKS mode)", defaultMemAllocLUKS))
	rootCmd.PersistentFlags().Uint32Var(&vmOSUpTimeoutFlag, "vm-os-up-timeout", 30, "Specifies the VM OS-up timeout in seconds.")
	rootCmd.PersistentFlags().Uint32Var(&vmSSHSetupTimeoutFlag, "vm-ssh-setup-timeout", 60, "Specifies the VM SSH server setup timeout in seconds. This cannot be lower than the OS-up timeout.")

	defaultDataDir := "linsk-data-dir"

	homeDir, err := os.UserHomeDir()
	if err != nil {
		slog.Error("Failed to get user home directory, will use a local directory as a fallback", "error", err.Error(), "dir", defaultDataDir)
	} else {
		homeDirName := ".linsk"
		if osspecifics.IsWindows() {
			homeDirName = "Linsk"
		}

		defaultDataDir = filepath.Join(homeDir, homeDirName)
	}

	rootCmd.PersistentFlags().StringVarP(&dataDirFlag, "data-dir", "d", defaultDataDir, "Specifies the data directory (folder) to use. VM images and related work files will be stored here.")
}
