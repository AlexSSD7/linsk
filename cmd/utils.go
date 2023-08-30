package cmd

import (
	"context"
	"fmt"
	"net"
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
	case "darwin":
		// Root privileges is not required in macOS.
		return
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

func runVM(passthroughArg string, fn func(context.Context, *vm.VM, *vm.FileManager) int, forwardPortsRules []vm.PortForwardingRule, unrestrictedNetworking bool) int {
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
		slog.Error("Failed to check/download VM BIOS", "error", err)
		os.Exit(1)
	}

	var passthroughConfig vm.PassthroughConfig

	if passthroughArg != "" {
		passthroughConfig = getDevicePassthroughConfig(passthroughArg)
		doUSBRootCheck()
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

		OSUpTimeout:  time.Duration(vmOSUpTimeoutFlag) * time.Second,
		SSHUpTimeout: time.Duration(vmSSHSetupTimeoutFlag) * time.Second,

		UnrestrictedNetworking: unrestrictedNetworking,
		ShowDisplay:            vmDebugFlag,
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

			exitCode := fn(ctx, vi, fm)

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

func checkPortAvailable(port uint16, subsequent uint16) (bool, error) {
	if port+subsequent < port {
		return false, fmt.Errorf("subsequent ports exceed allowed port range")
	}

	if subsequent == 0 {
		ln, err := net.Listen("tcp", ":"+fmt.Sprint(port))
		if err != nil {
			if opErr, ok := err.(*net.OpError); ok {
				if sysErr, ok := opErr.Err.(*os.SyscallError); ok {
					if sysErr.Err == syscall.EADDRINUSE {
						// The port is in use.
						return false, nil
					}
				}
			}

			return false, errors.Wrapf(err, "net listen (port %v)", port)
		}

		err = ln.Close()
		if err != nil {
			return false, errors.Wrap(err, "close ephemeral listener")
		}

		return true, nil
	}

	for i := uint16(0); i < subsequent; i++ {
		ok, err := checkPortAvailable(port+i, 0)
		if err != nil {
			return false, errors.Wrapf(err, "check subsequent port available (base: %v, seq: %v)", port, i)
		}

		if !ok {
			return false, nil
		}
	}

	return true, nil
}

func getClosestAvailPortWithSubsequent(port uint16, subsequent uint16) (uint16, error) {
	// We use 10 as port range
	for i := port; i < 65535; i += subsequent {
		ok, err := checkPortAvailable(i, subsequent)
		if err != nil {
			return 0, errors.Wrapf(err, "check port available (%v)", i)
		}

		if ok {
			return i, nil
		}
	}

	return 0, fmt.Errorf("no available port (with %v subsequent ones) found", subsequent)
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
		stat, err := os.Stat(devPath)
		if err != nil {
			slog.Error("Failed to stat the device path", "error", err.Error(), "path", devPath)
			os.Exit(1)
		}

		isDev := stat.Mode()&os.ModeDevice != 0
		if !isDev {
			slog.Error("Provided path is not a path to a valid block device", "path", devPath, "file-mode", stat.Mode())
			os.Exit(1)
		}

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
