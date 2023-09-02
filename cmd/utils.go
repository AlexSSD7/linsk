package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"log/slog"

	"github.com/AlexSSD7/linsk/nettap"
	"github.com/AlexSSD7/linsk/osspecifics"
	"github.com/AlexSSD7/linsk/share"
	"github.com/AlexSSD7/linsk/storage"
	"github.com/AlexSSD7/linsk/vm"
	"github.com/pkg/errors"
)

func createStoreOrExit() *storage.Storage {
	store, err := storage.NewStorage(slog.With("caller", "storage"), dataDirFlag)
	if err != nil {
		slog.Error("Failed to create Linsk data storage", "error", err.Error(), "data-dir", dataDirFlag)
		os.Exit(1)
	}

	return store
}

type runVMFunc func(context.Context, *vm.VM, *vm.FileManager, *share.NetTapRuntimeContext) int

func runVM(passthroughArg string, fn runVMFunc, forwardPortsRules []vm.PortForwardingRule, unrestrictedNetworking bool, withNetTap bool) int {
	store := createStoreOrExit()

	vmImagePath, err := store.CheckVMImageExists()
	if err != nil {
		slog.Error("Failed to check whether VM image exists", "error", err.Error())
		return 1
	}

	if vmImagePath == "" {
		slog.Error("VM image does not exist. You need to build it first before attempting to start Linsk. Please run `linsk build` first.")
		return 1
	}

	biosPath, err := store.CheckDownloadVMBIOS()
	if err != nil {
		slog.Error("Failed to check/download VM BIOS", "error", err.Error())
		return 1
	}

	var passthroughConfig vm.PassthroughConfig

	if passthroughArg != "" {
		passthroughConfigPtr, err := getDevicePassthroughConfig(passthroughArg)
		if err != nil {
			slog.Error("Failed to get device passthrough config", "error", err.Error())
			return 1
		}

		passthroughConfig = *passthroughConfigPtr
	}

	if len(passthroughConfig.USB) != 0 {
		// Log USB-related warnings.

		// Unfortunately USB passthrough is unstable in macOS and Windows. On Windows, you also need to install external
		// libusbK driver, which nullifies the UX. This is a problem with how QEMU works, and unfortunately there isn't
		// much we can do about it from our side.

		switch runtime.GOOS {
		case "windows":
			// TODO: To document: installation of libusbK driver with Zadig utility.
			slog.Warn("USB passthrough is unstable on Windows and requires installation of libusbK driver. Please consider using raw block device passthrough instead.")
		case "darwin":
			slog.Warn("USB passthrough is unstable on macOS. Please consider using raw block device passthrough instead.")
		}
	}

	var tapRuntimeCtx *share.NetTapRuntimeContext
	var tapsConfig []vm.TapConfig

	if withNetTap {
		tapManager, err := nettap.NewTapManager(slog.With("caller", "nettap-manager"))
		if err != nil {
			slog.Error("Failed to create new network tap manager", "error", err.Error())
			return 1
		}

		tapNameToUse, err := nettap.NewUniqueTapName()
		if err != nil {
			slog.Error("Failed to generate new network tap name", "error", err.Error())
			return 1
		}

		knownAllocs, err := store.ListNetTapAllocations()
		if err != nil {
			slog.Error("Failed to list net tap allocations", "error", err.Error())
			return 1
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
			return 1
		}

		tapManager, err = nettap.NewTapManager(slog.Default())
		if err != nil {
			slog.Error("Failed to create net tap manager", "error", err.Error())
			return 1
		}

		err = tapManager.CreateNewTap(tapNameToUse)
		if err != nil {
			releaseErr := store.ReleaseNetTapAllocation(tapNameToUse)
			if releaseErr != nil {
				slog.Error("Failed to release net tap allocation", "error", releaseErr.Error(), "tap-name", tapNameToUse)
				// Non-critical error.
			}

			slog.Error("Failed to create new tap", "error", err.Error())
			return 1
		}

		defer func() {
			err := tapManager.DeleteTap(tapNameToUse)
			if err != nil {
				slog.Error("Failed to clean up net tap", "error", err.Error(), "tap-name", tapNameToUse)
			} else {
				err = store.ReleaseNetTapAllocation(tapNameToUse)
				if err != nil {
					slog.Error("Failed to release net tap allocation", "error", err.Error(), "tap-name", tapNameToUse)
				}
			}
		}()

		tapNet, err := nettap.GenerateNet()
		if err != nil {
			slog.Error("Failed to generate tap net plan", "error", err.Error())
			return 1
		}

		err = tapManager.ConfigureNet(tapNameToUse, tapNet.HostCIDR)
		if err != nil {
			slog.Error("Failed to configure tap net", "error", err.Error())
			return 1
		}

		tapRuntimeCtx = &share.NetTapRuntimeContext{
			Manager: tapManager,
			Name:    tapNameToUse,
			Net:     tapNet,
		}

		tapsConfig = []vm.TapConfig{{
			Name: tapNameToUse,
		}}
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

	return innerRunVM(vmCfg, tapRuntimeCtx, fn)
}

func innerRunVM(vmCfg vm.VMConfig, tapRuntimeCtx *share.NetTapRuntimeContext, fn runVMFunc) int {
	vi, err := vm.NewVM(slog.Default().With("caller", "vm"), vmCfg)
	if err != nil {
		slog.Error("Failed to create vm instance", "error", err.Error())
		return 1
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
			return 1
		case <-vi.SSHUpNotifyChan():
			err := fm.Init()
			if err != nil {
				slog.Error("Failed to initialize File Manager", "error", err.Error())
				return 1
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
				return 1
			}

			wg.Wait()

			select {
			case err := <-runErrCh:
				if err != nil {
					slog.Error("Failed to run the VM", "error", err.Error())
					return 1
				}
			default:
			}

			return exitCode
		}
	}
}

func getDevicePassthroughConfig(val string) (*vm.PassthroughConfig, error) {
	isRoot, err := osspecifics.CheckRunAsRoot()
	if err != nil {
		return nil, errors.Wrap(err, "check whether the program is run as root")
	}

	if !isRoot {
		return nil, fmt.Errorf("device passthrough of any type requires root (admin) privileges")
	}

	valSplit := strings.Split(val, ":")
	if want, have := 2, len(valSplit); want != have {
		return nil, fmt.Errorf("bad device passthrough syntax: wrong items split by ':' count: want %v, have %v", want, have)
	}

	switch valSplit[0] {
	case "usb":
		usbValsSplit := strings.Split(valSplit[1], ",")
		if want, have := 2, len(usbValsSplit); want != have {
			return nil, fmt.Errorf("bad usb device passthrough syntax: wrong args split by ',' count: want %v, have %v", want, have)
		}

		vendorID, err := strconv.ParseUint(usbValsSplit[0], 16, 32)
		if err != nil {
			return nil, fmt.Errorf("bad usb vendor id '%v'", usbValsSplit[0])
		}

		productID, err := strconv.ParseUint(usbValsSplit[1], 16, 32)
		if err != nil {
			return nil, fmt.Errorf("bad usb product id '%v'", usbValsSplit[1])
		}

		return &vm.PassthroughConfig{
			USB: []vm.USBDevicePassthroughConfig{{
				VendorID:  uint16(vendorID),
				ProductID: uint16(productID),
			}},
		}, nil
	case "dev":
		devPath := filepath.Clean(valSplit[1])

		err := osspecifics.CheckValidDevicePath(devPath)
		if err != nil {
			return nil, errors.Wrapf(err, "check whether device path is valid '%v'", devPath)
		}

		return &vm.PassthroughConfig{Block: []vm.BlockDevicePassthroughConfig{{
			Path: devPath,
		}}}, nil
	default:
		return nil, fmt.Errorf("unknown device passthrough type '%v'", val)
	}
}
