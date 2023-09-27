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

	"github.com/spf13/pflag"
)

const defaultVMMountDevName = "vdb"

func getLUKSContainerDevice() string {
	var luksContainerDevice string

	if vmRuntimeLUKSContainerFlag != "" {
		if vmRuntimeLUKSContainerEntireDriveFlag {
			slog.Error("--luks-container and --luks-container-entire-drive (-c) cannot be both specified at once")
			os.Exit(1)
		}

		luksContainerDevice = vmRuntimeLUKSContainerFlag
	} else if vmRuntimeLUKSContainerEntireDriveFlag {
		luksContainerDevice = defaultVMMountDevName
	}

	return luksContainerDevice
}

var (
	vmRuntimeLUKSContainerFlag            string
	vmRuntimeLUKSContainerEntireDriveFlag bool

	// These are for internal use by the initVMRuntimeFlags and configureVMRuntimeFlags functions.
	vmRuntimeInternalAllowLUKSLowMemoryFlag bool

	// These are to be initialized (set) by the initVMRuntimeFlags function.
	vmRuntimeLUKSContainerDevice string
)

func initVMRuntimeFlags(flags *pflag.FlagSet) {
	flags.StringVar(&vmRuntimeLUKSContainerFlag, "luks-container", "", `Specifies a device path (without "dev/" prefix) to preopen as a LUKS container (password will be prompted). Useful for accessing LVM partitions behind LUKS.`)
	flags.BoolVarP(&vmRuntimeLUKSContainerEntireDriveFlag, "luks-container-entire-drive", "c", false, `Similar to --luks-container, but this assumes that the entire passed-through volume is a LUKS container (password will be prompted).`)
	flags.BoolVar(&vmRuntimeInternalAllowLUKSLowMemoryFlag, "allow-luks-low-memory", false, "Allow VM memory allocation lower than 2048 MiB when LUKS is enabled.")
}

func configureVMRuntimeFlags() {
	vmRuntimeLUKSContainerDevice = getLUKSContainerDevice()

	if (luksFlag || vmRuntimeLUKSContainerDevice != "") && !vmRuntimeInternalAllowLUKSLowMemoryFlag {
		if vmMemAllocFlag < defaultMemAllocLUKS {
			if vmMemAllocFlag != defaultMemAlloc {
				slog.Warn("Enforcing minimum LUKS memory allocation. Please add --allow-luks-low-memory to disable this.", "min", vmMemAllocFlag, "specified", vmMemAllocFlag)
			}

			vmMemAllocFlag = defaultMemAllocLUKS
		}
	}
}
