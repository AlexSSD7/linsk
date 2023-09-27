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
				err := fm.InitLVM()
				if err != nil {
					slog.Error("Failed to initialize File Manager LVM", "error", err.Error())
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
