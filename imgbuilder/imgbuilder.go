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

package imgbuilder

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"log/slog"

	"github.com/AlexSSD7/linsk/cmd/runvm"
	"github.com/AlexSSD7/linsk/osspecifics"
	"github.com/AlexSSD7/linsk/share"
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

func NewBuildContext(logger *slog.Logger, baseISOPath string, outPath string, debug bool, biosPath string) (*BuildContext, error) {
	baseISOPath = filepath.Clean(baseISOPath)
	outPath = filepath.Clean(outPath)

	_, err := os.Stat(outPath)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, errors.Wrap(err, "stat output file")
		}

		// File doesn't exist. Going forward with creating a new .qcow2 image.
	} else {
		return nil, fmt.Errorf("output file already exists")
	}

	err = createQEMUImg(outPath)
	if err != nil {
		return nil, errors.Wrap(err, "create temporary qemu image")
	}

	vi, err := vm.NewVM(logger.With("subcaller", "vm"), vm.Config{
		CdromImagePath: baseISOPath,
		BIOSPath:       biosPath,
		Drives: []vm.DriveConfig{{
			Path: outPath,
		}},

		MemoryAlloc: 512,

		UnrestrictedNetworking: true,
		Debug:                  debug,
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

	if osspecifics.IsWindows() {
		baseCmd += ".exe"
	}

	err := exec.Command(baseCmd, "create", "-f", "qcow2", outPath, "1G").Run()
	if err != nil {
		return errors.Wrap(err, "run qemu-img create cmd")
	}

	return nil
}

func (bc *BuildContext) RunCLIBuild() int {
	return runvm.RunVM(bc.vi, false, nil, func(ctx context.Context, v *vm.VM, fm *vm.FileManager, ntrc *share.NetTapRuntimeContext) int {
		sc, err := bc.vi.DialSSH()
		if err != nil {
			bc.logger.Error("Failed to dial VM SSH", "error", err.Error())
			return 1
		}

		defer func() { _ = sc.Close() }()

		bc.logger.Info("VM OS installation in progress")

		err = runAlpineSetup(sc, []string{"openssh", "lvm2", "util-linux", "cryptsetup", "vsftpd", "samba", "netatalk"})
		if err != nil {
			bc.logger.Error("Failed to set up Alpine Linux", "error", err.Error())
			return 1
		}

		return 0
	})
}

func runAlpineSetup(sc *ssh.Client, pkgs []string) error {
	sess, err := sc.NewSession()
	if err != nil {
		return errors.Wrap(err, "new session")
	}

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

	//nolint:dupword
	cmd += `&& chroot /mnt ash -c 'echo "PasswordAuthentication no" >> /etc/ssh/sshd_config && addgroup -g 1000 linsk && adduser -D -h /mnt -G linsk linsk -u 1000 && touch /etc/network/interfaces'`

	err = sess.Run(cmd)
	if err != nil {
		return utils.WrapErrWithLog(err, "run setup cmd", stderr.String())
	}

	return nil
}
