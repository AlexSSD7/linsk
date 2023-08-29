package builder

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"

	"log/slog"

	"github.com/AlexSSD7/linsk/utils"
	"github.com/AlexSSD7/linsk/vm"
	"github.com/alessio/shellescape"
	"github.com/pkg/errors"
	"golang.org/x/crypto/ssh"
)

type BuildContext struct {
	logger *slog.Logger

	vi *vm.VM
}

func NewBuildContext(logger *slog.Logger, baseISOPath string, outPath string, showVMDisplay bool) (*BuildContext, error) {
	baseISOPath = filepath.Clean(baseISOPath)
	outPath = filepath.Clean(outPath)

	_, err := os.Stat(outPath)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, errors.Wrap(err, "stat output file")
		}

		// File doesn't exist. Continuing.
	} else {
		return nil, fmt.Errorf("output file already exists")
	}

	err = createQEMUImg(outPath)
	if err != nil {
		return nil, errors.Wrap(err, "create temporary qemu image")
	}

	vi, err := vm.NewVM(logger.With("subcaller", "vm"), vm.VMConfig{
		CdromImagePath: baseISOPath,
		Drives: []vm.DriveConfig{{
			Path: outPath,
		}},
		MemoryAlloc:            512,
		UnrestrictedNetworking: true,
		ShowDisplay:            showVMDisplay,
		InstallBaseUtilities:   true,
	})
	if err != nil {
		return nil, errors.Wrap(err, "create vm instance")
	}

	return &BuildContext{
		logger: logger,

		vi: vi,
	}, nil
}

func createQEMUImg(outPath string) error {
	outPath = filepath.Clean(outPath)
	baseCmd := "qemu-img"

	if runtime.GOOS == "windows" {
		baseCmd += ".exe"
	}

	err := exec.Command(baseCmd, "create", "-f", "qcow2", outPath, "1G").Run()
	if err != nil {
		return errors.Wrap(err, "run qemu-img create cmd")
	}

	return nil
}

func (bc *BuildContext) BuildWithInterruptHandler() error {
	defer func() {
		err := bc.vi.Cancel()
		if err != nil {
			bc.logger.Error("Failed to cancel VM context", "error", err.Error())
		}
	}()

	runErrCh := make(chan error, 1)
	var wg sync.WaitGroup

	ctx, ctxCancel := context.WithCancel(context.Background())
	defer ctxCancel()

	interrupt := make(chan os.Signal, 2)
	signal.Notify(interrupt, syscall.SIGTERM, syscall.SIGINT)
	defer signal.Reset()

	wg.Add(1)
	go func() {
		defer wg.Done()

		err := bc.vi.Run()
		ctxCancel()
		runErrCh <- err
	}()

	go func() {
		for i := 0; ; i++ {
			select {
			case <-ctx.Done():
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

				err := bc.vi.Cancel()
				if err != nil {
					lg.Warn("Failed to cancel VM context", "error", err.Error())
				}
			}
		}
	}()

	for {
		select {
		case err := <-runErrCh:
			if err == nil {
				return fmt.Errorf("operation canceled by user")
			}

			return errors.Wrap(err, "start vm")
		case <-bc.vi.SSHUpNotifyChan():
			sc, err := bc.vi.DialSSH()
			if err != nil {
				return errors.Wrap(err, "dial vm ssh")
			}

			defer func() { _ = sc.Close() }()

			bc.logger.Info("Installation in progress")

			err = runAlpineSetupCmd(sc, []string{"openssh", "lvm2", "util-linux", "cryptsetup", "vsftpd"})
			if err != nil {
				return errors.Wrap(err, "run alpine setup cmd")
			}

			err = bc.vi.Cancel()
			if err != nil {
				return errors.Wrap(err, "cancel vm context")
			}

			select {
			case err := <-runErrCh:
				if err != nil {
					return errors.Wrap(err, "run vm")
				}
			default:
			}

			return nil
		}
	}
}

func runAlpineSetupCmd(sc *ssh.Client, pkgs []string) error {
	sess, err := sc.NewSession()
	if err != nil {
		return errors.Wrap(err, "new session")
	}

	// TODO: Timeout for this command.

	stderr := bytes.NewBuffer(nil)
	sess.Stderr = stderr

	defer func() {
		_ = sess.Close()
	}()

	cmd := "ifconfig eth0 up && ifconfig lo up && udhcpc && true > /etc/apk/repositories && setup-apkrepos -c -1 && printf 'y' | setup-disk -m sys /dev/vda"

	if len(pkgs) != 0 {
		pkgsQuoted := make([]string, len(pkgs))
		for i, rawPkg := range pkgs {
			pkgsQuoted[i] = shellescape.Quote(rawPkg)
		}

		cmd += " && mount /dev/vda3 /mnt && chroot /mnt apk add " + strings.Join(pkgsQuoted, " ")
	}

	cmd += `&& chroot /mnt ash -c 'echo "PasswordAuthentication no" >> /etc/ssh/sshd_config && addgroup -g 1000 linsk && adduser -D -h /mnt -G linsk linsk -u 1000'`

	err = sess.Run(cmd)
	if err != nil {
		return utils.WrapErrWithLog(err, "run setup cmd", stderr.String())
	}

	return nil
}
