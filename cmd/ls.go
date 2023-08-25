package cmd

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/AlexSSD7/vldisk/vm"
	"github.com/inconshreveable/log15"
	"github.com/spf13/cobra"
)

var lsCmd = &cobra.Command{
	Use: "ls",
	// TODO: Fill this
	// Short: "",
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		doRootCheck()

		passthroughConfig := getDevicePassthroughConfig(args[0])

		// TODO: We should download alpine image ourselves.
		// TODO: ALSO, we need make it usable offline. We can't always download packages from the web.

		// TODO: CLI-friendly logging.
		vi, err := vm.NewInstance(log15.New(), "alpine-img/alpine.qcow2", []vm.USBDevicePassthroughConfig{passthroughConfig}, true)
		if err != nil {
			fmt.Printf("Failed to create VM instance: %v.\n", err)
			os.Exit(1)
		}

		runErrCh := make(chan error, 1)

		var wg sync.WaitGroup

		wg.Add(1)
		go func() {
			defer wg.Done()

			err := vi.Run()
			runErrCh <- err
		}()

		fm := vm.NewFileManager(vi)

		for {
			select {
			case err := <-runErrCh:
				fmt.Printf("Failed to run the VM: %v.\n", err)
				os.Exit(1)
			case <-vi.SSHUpNotifyChan():
				err := fm.Init()
				if err != nil {
					fmt.Printf("Failed to initialize file manager: %v.\n", err)
					os.Exit(1)
				}

				lsblkOut, err := fm.Lsblk()
				if err != nil {
					fmt.Printf("Failed to run list block devices in the VM: %v.\n", err)
					os.Exit(1)
				}

				fmt.Print(string(lsblkOut))

				err = vi.Cancel()
				if err != nil {
					fmt.Printf("Failed to cancel VM context: %v.\n", err)
					os.Exit(1)
				}

				wg.Wait()

				return nil
			}
		}
	},
}

func getDevicePassthroughConfig(val string) vm.USBDevicePassthroughConfig {
	valSplit := strings.Split(val, ":")
	if want, have := 2, len(valSplit); want != have {
		fmt.Printf("Bad device passthrough syntax (wrong items split by ':' count: want %v, have %v).\n", want, have)
		os.Exit(1)
	}

	switch valSplit[0] {
	case "usb":
		usbValsSplit := strings.Split(valSplit[1], ",")
		if want, have := 2, len(usbValsSplit); want != have {
			fmt.Printf("Bad USB device passthrough syntax (wrong args split by ',' count: want %v, have %v).\n", want, have)
			os.Exit(1)
		}

		usbBus, err := strconv.ParseUint(usbValsSplit[0], 10, 8)
		if err != nil {
			fmt.Printf("Bad USB device bus number '%v' (%v).\n", usbValsSplit[0], err)
			os.Exit(1)
		}

		usbPort, err := strconv.ParseUint(usbValsSplit[1], 10, 8)
		if err != nil {
			fmt.Printf("Bad USB device port number '%v' (%v).\n", usbValsSplit[1], err)
			os.Exit(1)
		}

		return vm.USBDevicePassthroughConfig{
			HostBus:  uint8(usbBus),
			HostPort: uint8(usbPort),
		}
	default:
		fmt.Printf("Unknown device passthrough type '%v'.\n", valSplit[0])
		os.Exit(1)
		// This unreachable code is required to compile.
		return vm.USBDevicePassthroughConfig{}
	}
}
