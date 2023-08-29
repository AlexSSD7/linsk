package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"

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

func getDevicePassthroughConfig(val string) vm.USBDevicePassthroughConfig {
	valSplit := strings.Split(val, ":")
	if want, have := 2, len(valSplit); want != have {
		slog.Error("Bad device passthrough syntax", "error", fmt.Errorf("wrong items split by ':' count: want %v, have %v", want, have))
		os.Exit(1)
	}

	switch valSplit[0] {
	case "usb":
		usbValsSplit := strings.Split(valSplit[1], ",")
		if want, have := 2, len(usbValsSplit); want != have {
			slog.Error("Bad USB device passthrough syntax", "error", fmt.Errorf("wrong args split by ',' count: want %v, have %v", want, have))
			os.Exit(1)
		}

		vendorID, err := strconv.ParseUint(usbValsSplit[0], 16, 32)
		if err != nil {
			slog.Error("Bad USB vendor ID", "value", usbValsSplit[0])
			os.Exit(1)
		}

		productID, err := strconv.ParseUint(usbValsSplit[1], 16, 32)
		if err != nil {
			slog.Error("Bad USB product ID", "value", usbValsSplit[1])
			os.Exit(1)
		}

		return vm.USBDevicePassthroughConfig{
			VendorID:  uint16(vendorID),
			ProductID: uint16(productID),
		}
	default:
		slog.Error("Unknown device passthrough type", "value", valSplit[0])
		os.Exit(1)
		// This unreachable code is required to compile.
		return vm.USBDevicePassthroughConfig{}
	}
}
