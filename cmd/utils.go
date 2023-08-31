package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"log/slog"

	"github.com/AlexSSD7/linsk/nettap"
	"github.com/AlexSSD7/linsk/share"
	"github.com/AlexSSD7/linsk/storage"
	"github.com/AlexSSD7/linsk/vm"
	"github.com/pkg/errors"
)

func checkIfRoot() (bool, error) {
	currentUser, err := user.Current()
	if err != nil {
		return false, errors.Wrap(err, "get current user")
	}
	return currentUser.Username == "root", nil
}

func doUSBRootCheck() {
	switch runtime.GOOS {
	case "windows":
		// Administrator privileges are not required in Windows.
		return
	default:
		// As for everything else, we will likely need root privileges
		// for the USB passthrough.
	}

	ok, err := checkIfRoot()
	if err != nil {
		slog.Error("Failed to check whether the command is ran by root", "error", err.Error())
		return
	}

	if !ok {
		slog.Warn("USB passthrough on your OS usually requires this program to be ran as root")
	}
}

func createStore() *storage.Storage {
	store, err := storage.NewStorage(slog.With("caller", "storage"), dataDirFlag)
	if err != nil {
		slog.Error("Failed to create Linsk data storage", "error", err.Error(), "data-dir", dataDirFlag)
		os.Exit(1)
	}

	return store
}

func runVM(passthroughArg string, fn func(context.Context, *vm.VM, *vm.FileManager, *share.NetTapRuntimeContext) int, forwardPortsRules []vm.PortForwardingRule, unrestrictedNetworking bool, withNetTap bool) int {
	store := createStore()

	vmImagePath, err := store.CheckVMImageExists()
	if err != nil {
		slog.Error("Failed to check whether VM image exists", "error", err.Error())
		os.Exit(1)
	}

	if vmImagePath == "" {
		slog.Error("VM image does not exist. You need to build it first before attempting to start Linsk. Please run `linsk build` first.")
		os.Exit(1)
	}

	biosPath, err := store.CheckDownloadVMBIOS()
	if err != nil {
		slog.Error("Failed to check/download VM BIOS", "error", err.Error())
		os.Exit(1)
	}

	var passthroughConfig vm.PassthroughConfig

	if passthroughArg != "" {
		passthroughConfig = getDevicePassthroughConfig(passthroughArg)
		doUSBRootCheck()
	}

	var tapRuntimeCtx *share.NetTapRuntimeContext
	var tapsConfig []vm.TapConfig

	if withNetTap {
		tapManager, err := nettap.NewTapManager(slog.With("caller", "nettap-manager"))
		if err != nil {
			slog.Error("Failed to create new network tap manager", "error", err.Error())
			os.Exit(1)
		}

		tapNameToUse := nettap.NewRandomTapName()
		// TODO: Run two instances at the same time and check whether nothing is wrongfully pruned.
		knownAllocs, err := store.ListNetTapAllocations()
		if err != nil {
			slog.Error("Failed to list net tap allocations", "error", err.Error())
			os.Exit(1)
		}

		removedTaps, err := tapManager.PruneTaps(knownAllocs)
		if err != nil {
			slog.Error("Failed to prune dangling network taps", "error", err.Error())
		} else {
			// This is optional, meaning that we won't exit in panic if this fails.
			for _, removedTap := range removedTaps {
				err = store.ReleaseNetTapAllocation(removedTap)
				if err != nil {
					slog.Error("Failed to release a danging net tap allocation", "error", err.Error())
				}
			}
		}

		err = store.SaveNetTapAllocation(tapNameToUse, os.Getpid())
		if err != nil {
			slog.Error("Failed to save net tap allocation", "error", err.Error())
			os.Exit(1)
		}

		tapManager, err = nettap.NewTapManager(slog.Default())
		if err != nil {
			slog.Error("Failed to create net tap manager", "error", err.Error())
			os.Exit(1)
		}

		err = tapManager.CreateNewTap(tapNameToUse)
		if err != nil {
			releaseErr := store.ReleaseNetTapAllocation(tapNameToUse)
			if releaseErr != nil {
				slog.Error("Failed to release net tap allocation", "error", releaseErr.Error(), "tapname", tapNameToUse)
			}

			slog.Error("Failed to create new tap", "error", err.Error())
			os.Exit(1)
		}

		tapNet, err := nettap.GenerateNet()
		if err != nil {
			slog.Error("Failed to generate tap net plan", "error", err.Error())
			os.Exit(1)
		}

		err = tapManager.ConfigureNet(tapNameToUse, tapNet.HostCIDR)
		if err != nil {
			slog.Error("Failed to configure tap net", "error", err.Error())
			os.Exit(1)
		}

		tapRuntimeCtx = &share.NetTapRuntimeContext{
			Manager: tapManager,
			Name:    tapNameToUse,
			Net:     tapNet,
		}

		tapsConfig = []vm.TapConfig{{
			Name: tapNameToUse,
		}}

		// TODO: Clean the tap up before exiting.
	}

	vmCfg := vm.VMConfig{
		Drives: []vm.DriveConfig{{
			Path:         vmImagePath,
			SnapshotMode: true,
		}},

		MemoryAlloc: vmMemAllocFlag,
		BIOSPath:    biosPath,

		PassthroughConfig:        passthroughConfig,
		ExtraPortForwardingRules: forwardPortsRules,

		UnrestrictedNetworking: unrestrictedNetworking,
		Taps:                   tapsConfig,

		OSUpTimeout:  time.Duration(vmOSUpTimeoutFlag) * time.Second,
		SSHUpTimeout: time.Duration(vmSSHSetupTimeoutFlag) * time.Second,

		ShowDisplay: vmDebugFlag,
	}

	vi, err := vm.NewVM(slog.Default().With("caller", "vm"), vmCfg)
	if err != nil {
		slog.Error("Failed to create vm instance", "error", err.Error())
		os.Exit(1)
	}

	runErrCh := make(chan error, 1)
	var wg sync.WaitGroup

	ctx, ctxCancel := context.WithCancel(context.Background())
	defer ctxCancel()

	interrupt := make(chan os.Signal, 2)
	signal.Notify(interrupt, syscall.SIGTERM, syscall.SIGINT)

	wg.Add(1)
	go func() {
		defer wg.Done()

		err := vi.Run()
		ctxCancel()
		runErrCh <- err
	}()

	go func() {
		for i := 0; ; i++ {
			select {
			case <-ctx.Done():
				signal.Reset()
				return
			case sig := <-interrupt:
				lg := slog.With("signal", sig)

				if i == 0 {
					lg.Warn("Caught interrupt, safely shutting down")
				} else if i < 10 {
					lg.Warn("Caught subsequent interrupt, please interrupt n more times to panic", "n", 10-i)
				} else {
					panic("force interrupt")
				}

				err := vi.Cancel()
				if err != nil {
					lg.Warn("Failed to cancel VM context", "error", err.Error())
				}
			}
		}
	}()

	fm := vm.NewFileManager(slog.Default().With("caller", "file-manager"), vi)

	for {
		select {
		case err := <-runErrCh:
			if err == nil {
				err = fmt.Errorf("operation canceled by user")
			}

			slog.Error("Failed to start the VM", "error", err.Error())
			os.Exit(1)
		case <-vi.SSHUpNotifyChan():
			err := fm.Init()
			if err != nil {
				slog.Error("Failed to initialize File Manager", "error", err.Error())
				os.Exit(1)
			}

			startupFailed := false

			if tapRuntimeCtx != nil {
				err := vi.ConfigureInterfaceStaticNet(context.Background(), "eth1", tapRuntimeCtx.Net.GuestCIDR)
				if err != nil {
					slog.Error("Failed to configure tag interface network", "error", err.Error())
					startupFailed = true
				}
			}

			var exitCode int

			if !startupFailed {
				exitCode = fn(ctx, vi, fm, tapRuntimeCtx)
			} else {
				exitCode = 1
			}

			err = vi.Cancel()
			if err != nil {
				slog.Error("Failed to cancel VM context", "error", err.Error())
				os.Exit(1)
			}

			wg.Wait()

			select {
			case err := <-runErrCh:
				if err != nil {
					slog.Error("Failed to run the VM", "error", err.Error())
					os.Exit(1)
				}
			default:
			}

			return exitCode
		}
	}
}

func getDevicePassthroughConfig(val string) vm.PassthroughConfig {
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

		return vm.PassthroughConfig{
			USB: []vm.USBDevicePassthroughConfig{{
				VendorID:  uint16(vendorID),
				ProductID: uint16(productID),
			}},
		}
	case "dev":
		devPath := filepath.Clean(valSplit[1])
		// TODO: This is for Linux only. Should support Windows as well.
		// stat, err := os.Stat(devPath)
		// if err != nil {
		// 	slog.Error("Failed to stat the device path", "error", err.Error(), "path", devPath)
		// 	os.Exit(1)
		// }

		// isDev := stat.Mode()&os.ModeDevice != 0
		// if !isDev {
		// 	slog.Error("Provided path is not a path to a valid block device", "path", devPath, "file-mode", stat.Mode())
		// 	os.Exit(1)
		// }

		return vm.PassthroughConfig{Block: []vm.BlockDevicePassthroughConfig{{
			Path: devPath,
		}}}
	default:
		slog.Error("Unknown device passthrough type", "value", valSplit[0])
		os.Exit(1)
		// This unreachable code is required to compile.
		return vm.PassthroughConfig{}
	}
}
