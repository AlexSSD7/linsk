package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"

	"github.com/AlexSSD7/vldisk/vm"
	"github.com/spf13/cobra"
)

var lsCmd = &cobra.Command{
	Use: "ls",
	// TODO: Fill this
	// Short: "",
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		runVM(args[0], func(ctx context.Context, i *vm.Instance, fm *vm.FileManager) {
			lsblkOut, err := fm.Lsblk()
			if err != nil {
				slog.Error("Failed to list block devices in the VM", "error", err.Error())
				os.Exit(1)
			}

			fmt.Print(string(lsblkOut))
		})

		return nil
	},
}

func getDevicePassthroughConfig(val string) vm.USBDevicePassthroughConfig {
	valSplit := strings.Split(val, ":")
	if want, have := 2, len(valSplit); want != have {
		slog.Error("Bad device passthrough syntax", "error", fmt.Errorf("wrong items split by ':' count: want %v, have %v", want, have).Error())
		os.Exit(1)
	}

	switch valSplit[0] {
	case "usb":
		usbValsSplit := strings.Split(valSplit[1], ",")
		if want, have := 2, len(usbValsSplit); want != have {
			slog.Error("Bad USB device passthrough syntax", "error", fmt.Errorf("wrong args split by ',' count: want %v, have %v", want, have).Error())
			os.Exit(1)
		}

		usbBus, err := strconv.ParseUint(usbValsSplit[0], 10, 8)
		if err != nil {
			slog.Error("Bad USB device bus number", "value", usbValsSplit[0])
			os.Exit(1)
		}

		usbPort, err := strconv.ParseUint(usbValsSplit[1], 10, 8)
		if err != nil {
			slog.Error("Bad USB device port number", "value", usbValsSplit[1])
			os.Exit(1)
		}

		return vm.USBDevicePassthroughConfig{
			HostBus:  uint8(usbBus),
			HostPort: uint8(usbPort),
		}
	default:
		slog.Error("Unknown device passthrough type", "value", valSplit[0])
		os.Exit(1)
		// This unreachable code is required to compile.
		return vm.USBDevicePassthroughConfig{}
	}
}
