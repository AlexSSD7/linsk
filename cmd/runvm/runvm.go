package runvm

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"log/slog"

	"github.com/AlexSSD7/linsk/share"
	"github.com/AlexSSD7/linsk/vm"
)

type Func func(context.Context, *vm.VM, *vm.FileManager, *share.NetTapRuntimeContext) int

func RunVM(vi *vm.VM, initFileManager bool, tapRuntimeCtx *share.NetTapRuntimeContext, fn Func) int {
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

				switch {
				case i == 0:
					lg.Warn("Caught interrupt, safely shutting down")
				case i < 10:
					lg.Warn("Caught subsequent interrupt, please interrupt n more times to panic", "n", 10-i)
				default:
					panic("force interrupt")
				}

				err := vi.Cancel()
				if err != nil {
					lg.Warn("Failed to cancel VM context", "error", err.Error())
				}
			}
		}
	}()

	var fm *vm.FileManager
	if initFileManager {
		fm = vm.NewFileManager(slog.Default().With("caller", "file-manager"), vi)
	}

	for {
		select {
		case err := <-runErrCh:
			if err == nil {
				err = fmt.Errorf("operation canceled by user")
			}

			slog.Error("Failed to start the VM", "error", err.Error())
			return 1
		case <-vi.SSHUpNotifyChan():
			if fm != nil {
				err := fm.Init()
				if err != nil {
					slog.Error("Failed to initialize File Manager", "error", err.Error())
					return 1
				}
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

			err := vi.Cancel()
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
