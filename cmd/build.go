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
	"log/slog"
	"os"

	"github.com/spf13/cobra"
)

var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "Build (set up) a VM image for local use. This needs to be run after the initial installation.",
	Run: func(cmd *cobra.Command, args []string) {
		store := createStoreOrExit()

		exitCode := store.RunCLIImageBuild(vmDebugFlag, buildOverwriteFlag)
		if exitCode != 0 {
			os.Exit(exitCode)
		}

		slog.Info("VM image built successfully", "path", store.GetVMImagePath())
	},
}

var buildOverwriteFlag bool

func init() {
	buildCmd.Flags().BoolVar(&buildOverwriteFlag, "overwrite", false, "Specifies whether the VM image should be overwritten with the build.")
}
