package cmd

import (
	"context"
	"os"
	"os/user"
	"sync"

	"log/slog"

	"github.com/AlexSSD7/vldisk/vm"
	"github.com/pkg/errors"
)

func checkIfRoot() (bool, error) {
	currentUser, err := user.Current()
	if err != nil {
		return false, errors.Wrap(err, "get current user")
	}
	return currentUser.Username == "root", nil
}

func doRootCheck() {
	ok, err := checkIfRoot()
	if err != nil {
		slog.Error("Failed to check whether the command is ran by root", "error", err.Error())
		os.Exit(1)
	}

	if !ok {
		slog.Error("You must run this program as root")
		os.Exit(1)
	}
}

func runVM(passthroughArg string, fn func(context.Context, *vm.Instance, *vm.FileManager)) *vm.Instance {
	doRootCheck()

	passthroughConfig := getDevicePassthroughConfig(passthroughArg)

	// TODO: Alpine image should be downloaded from somewhere.
	vi, err := vm.NewInstance(slog.Default(), "alpine-img/alpine.qcow2", []vm.USBDevicePassthroughConfig{passthroughConfig}, true)
	if err != nil {
		slog.Error("Failed to create vm instance", "error", err.Error())
		os.Exit(1)
	}

	runErrCh := make(chan error, 1)
	var wg sync.WaitGroup

	ctx, ctxCancel := context.WithCancel(context.Background())

	wg.Add(1)
	go func() {
		defer wg.Done()

		err := vi.Run()
		ctxCancel()
		runErrCh <- err
	}()

	fm := vm.NewFileManager(vi)

	for {
		select {
		case err := <-runErrCh:
			slog.Error("Failed to start the VM", "error", err.Error())
			os.Exit(1)
		case <-vi.SSHUpNotifyChan():
			err := fm.Init()
			if err != nil {
				slog.Error("Failed to initialize File Manager", "error", err.Error())
				os.Exit(1)
			}

			fn(ctx, vi, fm)

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

			return nil
		}
	}
}
