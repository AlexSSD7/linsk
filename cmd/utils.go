package cmd

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"os/user"
	"runtime"
	"sync"
	"syscall"

	"log/slog"

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
		os.Exit(1)
	}

	if !ok {
		slog.Error("USB passthrough on your OS requires this program to be ran as root")
		os.Exit(1)
	}
}

func runVM(passthroughArg string, fn func(context.Context, *vm.VM, *vm.FileManager) int, forwardPortsRules []vm.PortForwardingRule, unrestrictedNetworking bool) int {
	var passthroughConfig []vm.USBDevicePassthroughConfig

	if passthroughArg != "" {
		passthroughConfig = []vm.USBDevicePassthroughConfig{getDevicePassthroughConfig(passthroughArg)}
		doUSBRootCheck()
	}

	vmCfg := vm.VMConfig{
		Drives: []vm.DriveConfig{{
			Path:         "alpine.qcow2",
			SnapshotMode: true,
		}},

		MemoryAlloc: vmMemAllocFlag,

		USBDevices:               passthroughConfig,
		ExtraPortForwardingRules: forwardPortsRules,

		UnrestrictedNetworking: unrestrictedNetworking,
		ShowDisplay:            vmDebugFlag,
	}

	// TODO: Alpine image should be downloaded from somewhere.
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
